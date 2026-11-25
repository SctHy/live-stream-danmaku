import {api, button, el, roomPath, run, showDialog} from './business'
import {loggedIn, openAccount} from './account'
import {settings} from './danmaku'

interface Recording {
  id: number
  title: string
  status: string
  duration_ms: number
  started_at: string
  danmaku_count: number
  video_url: string
  chunk_ms: number
}
interface Item {
  id: number
  username: string
  content: string
  video_offset_ms: number
}
function stamp(ms: number) {
  const seconds = Math.floor(ms / 1000)
  const parts = [Math.floor(seconds / 60) % 60, seconds % 60]
  if (seconds >= 3600) parts.unshift(Math.floor(seconds / 3600))
  return parts.map(n => String(n).padStart(2, '0')).join(':')
}
const root = roomPath + '/replays'
export function replayLibrary(open: (r: Recording) => void) {
  let before = 0, revision = 0
  async function page(reset: boolean) {
    const version = ++revision
    const more = el('moreReplays'), list = el('recordings')
    more.hidden = true
    if (reset) {
      before = 0
      list.replaceChildren()
    }
    const data = await api<{items: Recording[]; has_more: boolean; next_before_id: number}>(root + '?before_id=' + before)
    if (version !== revision) return
    for (const r of data.items) {
      const row = document.createElement('li')
      const play = button(r.title, async () => {
        const detail = await api<Recording>(root + '/' + r.id)
        if (version !== revision || !el<HTMLDialogElement>('replays').open) return
        el<HTMLDialogElement>('replays').close()
        open(detail)
      })
      play.disabled = r.status !== 'ready'
      const meta = document.createElement('small')
      meta.textContent = [new Date(r.started_at).toLocaleString(), stamp(r.duration_ms), r.danmaku_count + ' 条弹幕', r.status].join(' · ')
      row.append(play, meta)
      if (!el('owner').hidden && !['live','processing'].includes(r.status)) row.append(button('删除', async () => {
        if (!confirm('永久删除此回放？')) return
        await api(root + '/' + r.id, 'DELETE')
        await page(true)
      }))
      list.append(row)
    }
    if (!list.children.length) list.textContent = '暂无回放'
    before = data.next_before_id
    more.hidden = !data.has_more
  }
  el('replaysOpen').onclick = () => { showDialog('replays'); void run(() => page(true)) }
  el('moreReplays').onclick = () => void run(() => page(false))
}
export function startReplay(video: HTMLVideoElement, r: Recording) {
  const scope = new AbortController()
  const endpoint = root + '/' + r.id + '/danmaku'
  const chunks = new Set<number>()
  const retryTimers = new Set<number>()
  const items = new Map<number, Item>()
  const buttons = new Map<number, HTMLButtonElement>()
  const ink = el('replayInk'), list = el('timeline')
  const sprites = new Map<number, HTMLElement>()
  let sorted: Item[] = [], active: number | undefined, frame = 0
  const reduced = matchMedia('(prefers-reduced-motion: reduce)')
  function merge(batch: Item[]) {
    for (const item of batch) {
      if (items.has(item.id)) continue
      items.set(item.id, item)
      const b = document.createElement('button')
      b.textContent = '[' + stamp(item.video_offset_ms) + '] ' + item.username + '　' + item.content
      b.onclick = () => { video.currentTime = item.video_offset_ms / 1000 }
      buttons.set(item.id, b)
      const li = document.createElement('li')
      li.append(b)
    }
    sorted = Array.from(items.values()).sort((a,b) => a.video_offset_ms - b.video_offset_ms || a.id - b.id)
    const fragment = document.createDocumentFragment()
    for (const item of sorted) fragment.append(buttons.get(item.id)!.parentElement!)
    list.replaceChildren(fragment)
    draw()
  }
  async function load(index: number) {
    const from = index * r.chunk_ms
    if (chunks.has(index) || index < 0 || from > r.duration_ms) return
    chunks.add(index)
    const query = new URLSearchParams({from_ms: String(from), to_ms: String(from + r.chunk_ms)})
    try {
      const batch: Item[] = []
      for (;;) {
        const res = await fetch(endpoint + '?' + query, {signal: AbortSignal.any([scope.signal, AbortSignal.timeout(10000)])})
        if (!res.ok) throw Error()
        const data: {items: Item[]; has_more: boolean; next_offset_ms: number; next_after_id: number} = await res.json()
        batch.push(...data.items)
        if (!data.has_more) break
        query.set('after_offset_ms', String(data.next_offset_ms))
        query.set('after_id', String(data.next_after_id))
      }
      if (!scope.signal.aborted) merge(batch)
    } catch {
      if (!scope.signal.aborted) {
        el('feedback').textContent = '回放弹幕加载失败，稍后重试'
        const timer = window.setTimeout(() => {
          retryTimers.delete(timer)
          chunks.delete(index)
          preload()
        }, 3000)
        retryTimers.add(timer)
      }
    }
  }
  function preload() {
    const index = Math.floor(video.currentTime * 1000 / r.chunk_ms)
    for (const n of [index - 1, index, index + 1]) void load(n)
  }
  function draw() {
    const now = video.currentTime * 1000
    let lo = 0, hi = sorted.length
    while (lo < hi) {
      const mid = Math.floor((lo + hi) / 2)
      if (sorted[mid].video_offset_ms <= now) lo = mid + 1
      else hi = mid
    }
    const id = sorted[lo - 1]?.id
    if (active !== id) {
      if (active !== undefined) buttons.get(active)?.removeAttribute('aria-current')
      buttons.get(id)?.setAttribute('aria-current','time')
      active = id
    }
    const width = ink.clientWidth, pitch = settings.size + 12
    const lanes = Math.floor(ink.clientHeight * settings.area / 100 / pitch)
    const visible = new Set<number>()
    const duration = Math.max(2000, width / settings.speed * 1000)
    for (let i = lo - 1; lanes && i >= 0 && visible.size < settings.density; i--) {
      const item = sorted[i], age = now - item.video_offset_ms
      if (age >= duration) break
      visible.add(item.id)
      let sprite = sprites.get(item.id)
      if (!sprite) {
        sprite = document.createElement('span')
        sprite.textContent = item.content
        ink.append(sprite)
        sprites.set(item.id, sprite)
      }
      sprite.style.color = settings.color
      sprite.style.fontSize = settings.size + 'px'
      const progress = age / duration
      const x = reduced.matches ? '3px' : `calc(${width * (1 - progress)}px - ${progress * 100}%)`
      sprite.style.transform = `translate(${x}, ${i % lanes * pitch}px)`
    }
    ink.style.opacity = String(settings.opacity / 100)
    for (const [key, sprite] of sprites) if (!visible.has(key)) { sprite.remove(); sprites.delete(key) }
  }
  function tick() {
    draw()
    if (!video.paused && !video.ended) frame = requestAnimationFrame(tick)
  }
  function sync() {
    cancelAnimationFrame(frame)
    preload()
    tick()
  }
  el('replayTitle').textContent = '回放 · ' + r.title
  el('source-status').textContent = '录播视频 · MP4'
  el('heading').textContent = '时间轴弹幕'
  el('messages').hidden = true
  list.hidden = el('replayBar').hidden = video.hidden = false
  const rate = el<HTMLSelectElement>('rate')
  rate.value = '1'
  rate.onchange = () => { video.playbackRate = Number(rate.value) }
  for (const event of ['playing','pause','seeking','seeked','timeupdate','loadedmetadata','ended'])
    video.addEventListener(event, sync, {signal: scope.signal})
  el('settings').addEventListener('input', draw, {signal: scope.signal})
  video.src = r.video_url
  video.playbackRate = 1
  preload()
  void video.play().catch(() => {})
  return {
    async send(content: string) {
      if (!loggedIn()) { openAccount(); el('account').querySelector('.result')!.textContent = '登录后可发送回放弹幕'; return false }
      const item = await api<Item>(endpoint, 'POST', {content, video_offset_ms: Math.floor(video.currentTime * 1000)})
      if (scope.signal.aborted) return false
      merge([item])
      return true
    },
    stop() {
      scope.abort()
      for (const timer of retryTimers) clearTimeout(timer)
      cancelAnimationFrame(frame)
      rate.onchange = null
      video.pause()
      video.removeAttribute('src')
      video.load()
      video.playbackRate = 1
      ink.replaceChildren()
      list.replaceChildren()
      list.hidden = el('replayBar').hidden = true
      el('messages').hidden = false
      el('heading').textContent = '实时弹幕'
    },
  }
}
