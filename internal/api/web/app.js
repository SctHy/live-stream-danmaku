const $ = (id) => document.getElementById(id)
const params = new URLSearchParams(location.search)
const room = params.get('room') || 'studio'
const session = params.get('session') || 'screening-01'
$('join').elements.room.value = room
$('join').elements.session.value = session
const base = '/api/rooms/' + [room, session].map(encodeURIComponent).join('/')
const video = $('video')
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
let lastSent = ''
let awaitingReply = false
let visible = new Map(),
  replayGeneration = 0,
  replayEvents = [],
  loadedUntil = 0
const feedback = (text) => {
  $('feedback').textContent = text
}
function controls() {
  for (const id of ['send', 'like', 'message']) $(id).disabled = !ready || !live
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
  user.textContent = '观众 ' + d.user_id
  item.append(user, document.createTextNode(d.content))
  $('messages').prepend(item)
  const option = new Option(d.content.slice(0, 28), d.event_id)
  option.dataset.uid = d.user_id
  $('target').add(option, 1)
  if ($('target').length > 101) $('target').remove(101)
  visible.set(d.event_id, item)
  if (visible.size > 100) {
    const key = visible.keys().next().value
    visible.get(key).remove()
    visible.delete(key)
  }
  if (!animate || $('overlay').childElementCount >= 20) return
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
  if (p.type === 105) withdraw(p.data.event_id)
  if (live && p.type === 101) {
    show(p.data, animate)
    if (lastSent && p.data.client_msg_id === lastSent) {
      awaitingReply = false
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
async function connect() {
  try {
    await json('/api/session', { method: 'POST' })
    const ws = new WebSocket(
      location.origin.replace(/^http/, 'ws') + base + '/live',
      'danmaku.batch.v1',
    )
    socket = ws
    let pending = [],
      syncing = false
    async function sync() {
      if (syncing) return
      // A fresh Redis snapshot covers every event received before this request.
      pending = []
      syncing = true
      try {
        const res = await json(
          base + '/recent?after=' + encodeURIComponent(cursor),
        )
        if (ws !== socket || ws.readyState !== WebSocket.OPEN) return
        if (res.reset && live) {
          clear()
          feedback('离线时间超出恢复窗口，已载入近期弹幕')
        }
        for (const p of res.events) apply(p, false)
        cursor = res.pos
        syncing = false
        const buffered = pending
        pending = []
        for (const p of buffered) ingest(p)
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
      const parsed = JSON.parse(data)
      for (const p of Array.isArray(parsed) ? parsed : [parsed]) {
        if (p.type === 106) {
          clock = { server: p.data.server_time_ms, local: performance.now() }
          alignLiveClip()
          void sync()
        } else if (p.type === 102) {
          $('stats').textContent =
            '在线 ' + p.data.online + ' · 赞 ' + p.data.likes
          if (p.data.pos && cursor && !older(p.data.pos, cursor)) void sync()
        } else if (
          p.type === 104 &&
          (!p.data.client_msg_id || p.data.client_msg_id === lastSent)
        ) {
          if (p.data.client_msg_id) awaitingReply = false
          feedback(
            {
              accepted: '网关已接受',
              invalid: '内容无效',
              muted: '当前已禁言',
              rate_limited: '发送过快',
              overloaded: '网关繁忙，请稍后重试',
            }[p.data.status] || '服务暂不可用',
          )
        } else if (p.pos) ingest(p)
      }
    }
    ws.onclose = reconnect
  } catch {
    reconnect()
  }
}
function reconnect() {
  ready = false
  if (awaitingReply) feedback('连接中断，最后一条弹幕的发送结果待确认')
  controls()
  $('connection').textContent = '正在重连'
  setTimeout(
    connect,
    Math.min(8000, 300 * 2 ** retries++) + Math.random() * 200,
  )
}
$('compose').onsubmit = (e) => {
  e.preventDefault()
  if (!ready || !live || !wsOpen()) return
  const text = $('message').value.trim()
  if (!text) return
  lastSent = crypto.randomUUID()
  awaitingReply = true
  socket.send(
    JSON.stringify({
      type: 101,
      data: {
        client_msg_id: lastSent,
        content: text,
        video_offset_ms: Math.floor(video.currentTime * 1000),
      },
    }),
  )
  $('message').value = ''
  feedback('发送中')
}
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
function drawReplay() {
  if (live) return
  const now = Math.floor(video.currentTime * 1000)
  while (replayEvents.length && replayEvents[0].video_offset_ms <= now)
    show(replayEvents.shift(), true)
  if (now >= loadedUntil) void loadReplay(loadedUntil)
}
video.ontimeupdate = drawReplay
video.onseeked = () => {
  if (!live) {
    clear()
    void loadReplay()
  }
}
function mode(isLive) {
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
  if (live) mode(false)
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
