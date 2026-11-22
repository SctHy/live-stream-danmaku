import './Settings'
import {enqueue, removeMessage, resetOverlay} from './danmaku'
import {playRoom} from './player'
const protocolURL = '/assets/protocol.js'
const packetType = await import(/* @vite-ignore */ protocolURL)

const $ = (id) => document.getElementById(id)
const params = new URLSearchParams(location.search)
const room = params.get('room') || '6'
const session = params.get('session') || 'screening-01'
$('join').elements.room.value = room
$('join').elements.session.value = session
const base = '/api/rooms/' + [room, session].map(encodeURIComponent).join('/')
const video = $('video')
const listLimit = 100
if (/^\d+$/.test(room)) {
  video.hidden = false
  playRoom(video, room, text => { $('source-status').textContent = text })
}
$('fullscreen').onclick = () => {
  if (document.fullscreenElement) void document.exitFullscreen()
  else document.querySelector('.screen').requestFullscreen().catch(() => feedback('浏览器未允许全屏'))
}
let socket,
  cursor = '',
  ready = false,
  retries = 0
let pendingSend = null
let userID = null
const visible = new Map()
const feedback = (text) => {
  $('feedback').textContent = text
}
function controls() {
  for (const id of ['send', 'like', 'message']) $(id).disabled = !ready

  $('send').disabled ||= !!pendingSend
  $('retry').hidden = !pendingSend
  $('retry').disabled = !ready
}
function clear() {
  visible.clear()
  $('target').length = 1
  $('messages').replaceChildren()
  resetOverlay()
  queuedRows.clear()
  snapshotRows = false
}
const queuedRows = new Map()
let listFrame = 0
let snapshotRows = false
function show(d, animate) {
  if (visible.has(d.event_id) || queuedRows.has(d.event_id)) return
  if (animate) enqueue(d)
  else snapshotRows = true
  if (document.hidden) return
  queuedRows.set(d.event_id, d)
  if (queuedRows.size > listLimit) queuedRows.delete(queuedRows.keys().next().value)
  if (!listFrame) listFrame = requestAnimationFrame(flushRows)
}
function flushRows() {
  listFrame = 0
  // Keep sidebar layout work bounded independently of socket traffic.
  const recent = Array.from(queuedRows.values())
  for (const d of (snapshotRows ? recent : recent.slice(-12))) appendRow(d)
  queuedRows.clear()
  snapshotRows = false
}
function appendRow(d) {
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
  if (visible.size > listLimit) {
    const key = visible.keys().next().value
    visible.get(key).remove()
    visible.delete(key)
  }
}
function withdraw(id) {
  visible.get(id)?.remove()
  visible.delete(id)
  for (const option of Array.from($('target').options))
    if (option.value === id) option.remove()
  queuedRows.delete(id)
  removeMessage(id)
}
function apply(p, animate) {
  if (p.type === packetType.withdraw) withdraw(p.data.event_id)
  if (p.type === packetType.danmaku) {
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
          if (reset) {
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
        if (p.type === packetType.ready) {
          void sync()
        } else if (p.type === packetType.migrate) {
          if (promoted) void connect()
          else ws.close()
        } else if (p.type === packetType.stats && promoted) {
          $('stats').textContent =
            '在线 ' + p.data.online + ' · 赞 ' + p.data.likes
          if (p.data.pos && cursor && !older(p.data.pos, cursor)) void sync()
        } else if (
          p.type === packetType.receipt &&
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
              filtered: '内容包含敏感词',
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
  if (pendingSend || !ready || !wsOpen()) return
  const text = $('message').value.trim()
  if (!text) return
  pendingSend = {
    started: Date.now(),
    data: {
      client_msg_id: crypto.randomUUID(),
      content: text,
    },
  }
  sendPending()
  $('message').value = ''
  feedback('发送中')
}
function sendPending() {
  if (!pendingSend || !wsOpen() || !ready) return
  if (Date.now() - pendingSend.started >= 9 * 60 * 1000) {
    pendingSend = null
    feedback('重试期限已过，请在近期弹幕中确认')
  } else {
    socket.send(JSON.stringify({type: packetType.danmaku, data: pendingSend.data}))
  }
  controls()
}
$('retry').onclick = sendPending
function wsOpen() {
  return socket?.readyState === WebSocket.OPEN
}
$('like').onclick = () => {
  if (wsOpen()) socket.send(JSON.stringify({ type: packetType.like, data: { count: 1 } }))
}
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
