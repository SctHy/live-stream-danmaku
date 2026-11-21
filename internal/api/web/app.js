const $ = (id) => document.getElementById(id)
const params = new URLSearchParams(location.search)
const room = params.get('room') || 'studio'
const session = params.get('session') || 'screening-01'
$('join').elements.room.value = room
$('join').elements.session.value = session
const base = '/api/rooms/' + [room, session].map(encodeURIComponent).join('/')
const video = $('video')
const demoSource = video.getAttribute('src')
const listLimit = 100
const overlayLimit = 20
let sample = null
let seekPending = false
let importVersion = 0
let socket,
  cursor = '',
  live = true,
  ready = false,
  retries = 0,
  lane = 0
let clock = null
function alignLiveClip() {
  if (
    live &&
    clock &&
    Number.isFinite(clock.server) &&
    Number.isFinite(video.duration) &&
    video.duration > 0
  ) {
    video.currentTime =
      ((clock.server + performance.now() - clock.local) / 1000) % video.duration
  }
}
video.addEventListener('loadedmetadata', alignLiveClip)
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) alignLiveClip()
})
let pendingSend = null
let userID = null
let visible = new Map(),
  replayGeneration = 0,
  replayEvents = [],
  loadedUntil = 0
const feedback = (text) => {
  $('feedback').textContent = text
}
function controls() {
  for (const id of ['send', 'like', 'message']) $(id).disabled = !ready || !live

  $('send').disabled ||= !!pendingSend
  $('retry').hidden = !pendingSend
  $('retry').disabled = !ready || !live
}
function clear() {
  visible.clear()
  $('target').length = 1
  $('messages').replaceChildren()
  $('overlay').replaceChildren()
}
function show(d, animate) {
  if (visible.has(d.event_id)) return
  const item = document.createElement('li'),
    user = document.createElement('small')
  user.textContent = sample
    ? (d.video_offset_ms / 1000).toFixed(2) + ' 秒'
    : '观众 ' + d.user_id
  item.append(user, document.createTextNode(d.content))
  $('messages').prepend(item)
  if (!sample) {
    const option = new Option(d.content.slice(0, 28), d.event_id)
    option.dataset.uid = d.user_id
    $('target').add(option, 1)
    if ($('target').length > 101) $('target').remove(101)
  }
  visible.set(d.event_id, item)
  if (visible.size > listLimit) {
    const key = visible.keys().next().value
    visible.get(key).remove()
    visible.delete(key)
  }
  if (!animate || $('overlay').childElementCount >= overlayLimit) return
  const span = document.createElement('span')
  span.textContent = d.content
  span.className = 'fly'
  span.dataset.id = d.event_id
  span.style.top = (lane++ % 5) * 17 + '%'
  span.style.setProperty('--travel', video.clientWidth + 'px')
  $('overlay').append(span)
  setTimeout(() => span.remove(), 6200)
}
function withdraw(id) {
  visible.get(id)?.remove()
  visible.delete(id)
  for (const option of Array.from($('target').options))
    if (option.value === id) option.remove()
  for (const node of $('overlay').children)
    if (node.dataset.id === id) node.remove()
  replayEvents = replayEvents.filter((d) => d.event_id !== id)
  if (!live) void loadReplay()
}
function apply(p, animate) {
  if (!sample && p.type === 105) withdraw(p.data.event_id)
  if (live && p.type === 101) {
    show(p.data, animate)
    if (pendingSend && p.data.user_id === userID && p.data.client_msg_id === pendingSend.data.client_msg_id) {
      pendingSend = null
      controls()
      feedback('弹幕已送达')
    }
  }
  cursor = p.pos
}
function older(a, b) {
  const [ae, aid] = a.includes('@') ? a.split('@') : ['', a]
  const [be, bid] = b.includes('@') ? b.split('@') : ['', b]
  if (ae !== be) return false
  const [at, as] = aid.split('-').map(BigInt),
    [bt, bs] = bid.split('-').map(BigInt)
  return at < bt || (at === bt && as <= bs)
}
async function json(url, options) {
  const r = await fetch(url, options)
  if (!r.ok) throw Error(String(r.status))
  return r.json()
}
let connecting = false
let retryTimer
async function connect() {
  if (connecting) return
  clearTimeout(retryTimer)
  connecting = true
  try {
    const sessionInfo = await json('/api/session', { method: 'POST', signal: AbortSignal.timeout(8000) })
    if (userID !== sessionInfo.user_id && pendingSend) {
      pendingSend = null
      feedback('身份已变更，上一条的结果请查看近期弹幕')
    }
    userID = sessionInfo.user_id
    const ws = new WebSocket(
      location.origin.replace(/^http/, 'ws') + base + '/live?migrate=1',
      'danmaku.batch.v1',
    )
    let promoted = false
    const deadline = setTimeout(() => ws.close(), 10000)
    let pending = [],
      syncing = false
    async function sync() {
      if (syncing) return
      // A fresh Redis snapshot covers every event received before this request.
      pending = []
      syncing = true
      try {
        const after = cursor
        const res = await json(
          base + '/recent?after=' + encodeURIComponent(after),
          { signal: AbortSignal.timeout(8000) },
        )
        if (ws.readyState !== WebSocket.OPEN || (promoted && socket !== ws)) return
        // The old socket can advance the cursor while the snapshot is in flight.
        // Apply only the suffix beyond that cursor, including across a stream reset.
        let events = res.events
        let reset = res.reset
        let advance = true
        if (cursor !== after) {
          const index = events.findIndex((p) => p.pos === cursor)
          if (index >= 0) {
            events = events.slice(index + 1)
            reset = false
          } else if (cursor && older(res.pos, cursor)) {
            advance = false
          } else {
            syncing = false
            void sync()
            return
          }
        }
        if (advance) {
          if (reset && live) {
            clear()
            feedback('离线时间超出恢复窗口，已载入近期弹幕')
          }
          for (const p of events) apply(p, false)
          cursor = res.pos
        }
        if (!promoted) {
          const previous = socket
          socket = ws
          promoted = true
          connecting = false
          clearTimeout(deadline)
          previous?.close()
        }
        syncing = false
        const buffered = pending
        pending = []
        for (const p of buffered) ingest(p)
        if (syncing || ws.readyState !== WebSocket.OPEN) return
        ready = true
        retries = 0
        controls()
        $('connection').textContent = '已连接'
      } catch {
        ws.close()
      }
    }
    function ingest(p) {
      if (syncing) {
        if (pending.length >= 512) ws.close()
        else pending.push(p)
        return
      }
      if (cursor && older(p.pos, cursor)) return
      if (p.prev !== (cursor || '0-0')) {
        pending.push(p)
        void sync()
        return
      }
      apply(p, true)
    }
    ws.onmessage = ({ data }) => {
      if (promoted && ws !== socket) return
      const parsed = JSON.parse(data)
      for (const p of Array.isArray(parsed) ? parsed : [parsed]) {
        if (p.type === 106) {
          clock = { server: p.data.server_time_ms, local: performance.now() }
          alignLiveClip()
          void sync()
        } else if (p.type === 107) {
          if (promoted) void connect()
          else ws.close()
        } else if (p.type === 102 && promoted) {
          $('stats').textContent =
            '在线 ' + p.data.online + ' · 赞 ' + p.data.likes
          if (p.data.pos && cursor && !older(p.data.pos, cursor)) void sync()
        } else if (
          p.type === 104 &&
          (!p.data.client_msg_id || p.data.client_msg_id === pendingSend?.data.client_msg_id)
        ) {
          if (pendingSend && p.data.client_msg_id && p.data.status !== 'unavailable') {
            if (p.data.status !== 'accepted' && !$('message').value) {
              $('message').value = pendingSend.data.content
            }
            pendingSend = null
          }
          controls()
          feedback(
            {
              accepted: '弹幕已发布',
              invalid: '内容无效',
              muted: '当前已禁言',
              rate_limited: '发送过快',
              overloaded: '网关繁忙，请稍后重试',
            }[p.data.status] || '服务暂不可用',
          )
        } else if (p.pos) ingest(p)
      }
    }
    ws.onclose = () => {
      clearTimeout(deadline)
      if (promoted && socket !== ws) return
      if (!promoted) connecting = false
      reconnect()
    }
  } catch {
    connecting = false
    reconnect()
  }
}
function reconnect() {
  if (!wsOpen()) {
    ready = false
    if (pendingSend) feedback('连接中断，最后一条弹幕的发送结果待确认')
    controls()
    $('connection').textContent = '正在重连'
  }
  if (connecting) return
  clearTimeout(retryTimer)
  const backoff = Math.min(8000, 300 * 2 ** retries++)
  retryTimer = setTimeout(connect, backoff / 2 + Math.random() * backoff)
}
$('compose').onsubmit = (e) => {
  e.preventDefault()
  if (pendingSend || !ready || !live || !wsOpen()) return
  const text = $('message').value.trim()
  if (!text) return
  pendingSend = {
    started: Date.now(),
    data: {
      client_msg_id: crypto.randomUUID(),
      content: text,
      video_offset_ms: Math.floor(video.currentTime * 1000),
    },
  }
  sendPending()
  $('message').value = ''
  feedback('发送中')
}
function sendPending() {
  if (!pendingSend || !wsOpen() || !ready || !live) return
  if (Date.now() - pendingSend.started >= 9 * 60 * 1000) {
    pendingSend = null
    feedback('重试期限已过，请在近期弹幕或回放中确认')
  } else {
    socket.send(JSON.stringify({type: 101, data: pendingSend.data}))
  }
  controls()
}
$('retry').onclick = sendPending
function wsOpen() {
  return socket?.readyState === WebSocket.OPEN
}
$('like').onclick = () => {
  if (wsOpen()) socket.send(JSON.stringify({ type: 103, data: { count: 1 } }))
}
async function loadReplay(start = Math.floor(video.currentTime * 1000)) {
  const generation = ++replayGeneration
  const until = start + 10000
  let from = start,
    after = 0,
    events = []
  loadedUntil = until
  replayEvents = []
  try {
    for (;;) {
      const res = await json(
        base + '/playback?' + new URLSearchParams({ from, after, until }),
      )
      if (generation !== replayGeneration || live) return
      events.push(...res.events)
      if (res.events.length < 100) break
      const last = res.events.at(-1)
      from = last.video_offset_ms
      after = last.id
    }
    replayEvents = events
    drawReplay()
  } catch {
    if (generation === replayGeneration) feedback('回放加载失败，请重新定位')
  }
}
function firstEventAtOrAfter(events, ms, lo = 0) {
  let hi = events.length
  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2)
    if (events[mid].video_offset_ms < ms) lo = mid + 1
    else hi = mid
  }
  return lo
}
function drawReplay() {
  if (live || video.seeking || seekPending) return
  const now = Math.floor(video.currentTime * 1000)
  if (sample) {
    const start = sample.index
    const end = firstEventAtOrAfter(sample.events, now + 1, start)
    const tail = Math.max(start, end - listLimit)
    // Earlier rows in this tick would be evicted before the next paint.
    // Keep the first available overlay slots and the most recent sidebar rows.
    const slots = Math.max(0, overlayLimit - $('overlay').childElementCount)
    for (let i = start; i < Math.min(tail, start + slots); i++)
      show(sample.events[i], true)
    for (let i = tail; i < end; i++) show(sample.events[i], true)
    sample.index = end
    return
  }
  while (replayEvents.length && replayEvents[0].video_offset_ms <= now)
    show(replayEvents.shift(), true)
  if (now >= loadedUntil) void loadReplay(loadedUntil)
}
video.ontimeupdate = drawReplay
video.onseeking = () => {
  seekPending = true
}
video.onloadstart = () => {
  seekPending = false
}
video.onseeked = () => {
  seekPending = false
  if (!live) {
    clear()
    if (sample) {
      sample.index = firstEventAtOrAfter(
        sample.events,
        Math.round(video.currentTime * 1000),
      )
      drawReplay()
    } else void loadReplay()
  }
}

$('sample-files').onchange = async (event) => {
  const version = ++importVersion
  const files = Array.from(event.target.files)
  event.target.value = ''
  const list = files.find((f) => /\.json$/i.test(f.name))
  const media = files.find(
    (f) => f.type.startsWith('video/') || /\.(mp4|m4v|webm|mov)$/i.test(f.name),
  )
  if (files.length !== 2 || !list || !media) {
    $('sample-status').textContent = '请同时选择一个视频和一个 JSON 文件。'
    return
  }
  try {
    const rows = JSON.parse(await list.text())
    if (version !== importVersion) return
    if (!Array.isArray(rows) || !rows.length) throw Error()
    const events = rows
      .map((row, i) => {
        if (
          !Number.isSafeInteger(row.video_offset_ms) ||
          row.video_offset_ms < 0 ||
          typeof row.content !== 'string'
        )
          throw Error()
        return {
          video_offset_ms: row.video_offset_ms,
          content: row.content,
          event_id: 'local-' + i,
        }
      })
      .sort((a, b) => a.video_offset_ms - b.video_offset_ms)
    if (sample) URL.revokeObjectURL(sample.url)
    sample = { events, index: 0, url: URL.createObjectURL(media) }
    live = false
    replayGeneration++
    replayEvents = []
    clear()
    controls()
    feedback('')
    $('live').setAttribute('aria-pressed', 'false')
    $('replay').setAttribute('aria-pressed', 'false')
    $('heading').textContent = '本地回放'
    $('moderation-panel').hidden = true
    $('sample-status').textContent =
      media.name + ' · ' + events.length + ' 条弹幕'
    video.loop = false
    video.src = sample.url
    video.play().catch(() => {})
  } catch {
    if (version === importVersion)
      $('sample-status').textContent =
        '弹幕文件需要包含 video_offset_ms 和 content 的 JSON 数组。'
  }
}
function mode(isLive) {
  importVersion++
  if (sample) {
    const previous = sample.url
    sample = null
    video.src = demoSource
    URL.revokeObjectURL(previous)
    $('sample-status').textContent = ''
    $('moderation-panel').hidden = false
  }
  live = isLive
  replayGeneration++
  clear()
  controls()
  feedback('')
  $('live').setAttribute('aria-pressed', live)
  $('replay').setAttribute('aria-pressed', !live)
  $('heading').textContent = live ? '实时弹幕' : '回放弹幕'
  video.loop = live
  if (live) {
    cursor = ''
    socket?.close()
    video.play().catch(() => {})
  } else {
    video.currentTime = 0
    void loadReplay()
    video.play().catch(() => {})
  }
}
$('live').onclick = () => {
  if (!live) mode(true)
}
$('replay').onclick = () => {
  if (live || sample) mode(false)
}
video.loop = true
video.play().catch(() => {})
void connect()

$('moderation').onsubmit = async (e) => {
  e.preventDefault()
  const selected = $('target').selectedOptions[0]
  if (!selected?.value) return
  try {
    const response = await fetch(base + '/moderate', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: 'Bearer ' + $('modKey').value,
      },
      body: JSON.stringify({
        action: e.submitter.value,
        event_id: selected.value,
        user_id: Number(selected.dataset.uid),
        seconds: 60,
      }),
    })
    feedback(
      response.ok
        ? '房管操作已执行'
        : response.status === 403
          ? '房管密钥或房间不匹配'
          : '房管操作失败',
    )
  } catch {
    feedback('房管操作失败')
  }
}
