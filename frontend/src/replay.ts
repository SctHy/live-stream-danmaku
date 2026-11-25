import {api, button, el, roomPath, run, showDialog} from './business'
import {loggedIn, openAccount} from './account'
import {settings, sendStyle} from './danmaku'

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
  mode?: string
  color?: string
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
  const chart = el('density')
  const svg = chart.querySelector('svg')!
  const marker = svg.querySelector('line')!
  void run(async () => {
    const data = await api<{duration_ms:number; bucket_ms:number; buckets:number[]; max_count:number}>(root + '/' + r.id + '/density')
    if (scope.signal.aborted || !data.duration_ms || !data.buckets.length) return
    const points = data.buckets.map((n,i) => (100 * (i + .5) / data.buckets.length) + ' ' + (23 - 21*n/Math.max(1,data.max_count)))
    const curve = 'M0 23 L' + points.join(' L') + ' L100 23'
    svg.querySelector('.area')!.setAttribute('d', curve+' L100 24 H0 Z')
    svg.querySelector('.line')!.setAttribute('d', curve)
    svg.setAttribute('aria-valuemax',String(Math.floor(data.duration_ms/1000)))
    const offset = (event: MouseEvent) => {
      const rect = svg.getBoundingClientRect()
      return Math.min(1,Math.max(0,(event.clientX-rect.left)/rect.width))*data.duration_ms
    }
    svg.onmousemove = event => {
      const i = Math.min(data.buckets.length-1,Math.floor(offset(event)/data.bucket_ms))
      chart.querySelector('output')!.value = stamp(i*data.bucket_ms)+'–'+stamp(Math.min(data.duration_ms,(i+1)*data.bucket_ms))+' · '+data.buckets[i]+' 条弹幕'
    }
    svg.onclick = event => { video.currentTime = offset(event)/1000 }
    svg.onkeydown = event => {
      if (!['ArrowLeft','ArrowRight'].includes(event.key)) return
      event.preventDefault()
      video.currentTime = Math.max(0,Math.min(data.duration_ms/1000,video.currentTime+(event.key==='ArrowRight'?1:-1)*data.bucket_ms/1000))
    }
    chart.hidden = false
  })
  const endpoint = root + '/' + r.id + '/danmaku'
  const chunks = new Set<number>()
  const retryTimers = new Set<number>()
  const items = new Map<number, Item>()
  const buttons = new Map<number, HTMLButtonElement>()
  const ink = el('replayInk'), list = el('timeline')
  const sprites = new Map<number, {node: HTMLElement; animation: Animation; layout: string; lane: number}>()
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
    if (!chart.hidden) {
      const pos = String(Math.min(100,100*now/r.duration_ms))
      marker.setAttribute('x1',pos)
      marker.setAttribute('x2',pos)
      svg.setAttribute('aria-valuenow',String(Math.floor(now/1000)))
    }
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
    const fixedLanes = {top: new Set<number>(), bottom: new Set<number>()}
    const duration = Math.max(2000, width / settings.speed * 1000)
    for (let i = lo - 1; lanes && i >= 0 && visible.size < settings.density; i--) {
      const item = sorted[i], age = now - item.video_offset_ms
      if (age >= duration) break
      let lane = i % lanes
      if (item.mode === 'top' || item.mode === 'bottom') {
        const slots = fixedLanes[item.mode]
        const count = Math.max(1,Math.min(lanes,Math.floor(ink.clientHeight/(2*pitch))))
        if (slots.size >= count) continue
        lane = sprites.get(item.id)?.lane ?? 0
        if (lane >= count || slots.has(lane)) lane = 0
        while (slots.has(lane)) lane++
        slots.add(lane)
      }
      visible.add(item.id)
      let sprite = sprites.get(item.id)
      const fixed = item.mode === 'top' || item.mode === 'bottom'
      const y = item.mode === 'bottom' ? ink.clientHeight - (lane + 1)*pitch : lane*pitch
      const layout = [width,y,duration,settings.size,settings.color,reduced.matches].join('/')
      if (sprite?.layout !== layout) {
        sprite?.animation.cancel()
        const node = sprite?.node || document.createElement('span')
        node.textContent = item.content
        node.style.color = settings.color === '#ffffff' && item.color ? item.color : settings.color
        node.style.fontSize = settings.size + 'px'
        if (!sprite) ink.append(node)
        const start = fixed ? `calc(${width/2}px - 50%)` : reduced.matches ? '3px' : width+'px'
        const end = fixed || reduced.matches ? start : '-100%'
        const animation = node.animate([
          {transform: `translate(${start}, ${y}px)`},
          {transform: `translate(${end}, ${y}px)`},
        ], {duration, fill:'both'})
        // Sample the media clock, including pauses, rate changes and seeks.
        animation.pause()
        sprite = {node, animation, layout, lane}
        sprites.set(item.id, sprite)
      }
      sprite.animation.currentTime = age
    }
    ink.style.opacity = String(settings.opacity / 100)
    for (const [key, sprite] of sprites) if (!visible.has(key)) { sprite.animation.cancel(); sprite.node.remove(); sprites.delete(key) }
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
      const item = await api<Item>(endpoint, 'POST', {content, ...sendStyle(), video_offset_ms: Math.floor(video.currentTime * 1000)})
      if (scope.signal.aborted) return false
      merge([item])
      return true
    },
    stop() {
      scope.abort()
      chart.hidden = true
      chart.querySelector('output')!.value = ''
      svg.onclick = svg.onmousemove = svg.onkeydown = null
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
