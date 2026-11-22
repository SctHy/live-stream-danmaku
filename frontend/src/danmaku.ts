type Message = {event_id: string; content: string; color?: string}
type Sprite = {id: string; image: HTMLCanvasElement; x: number; lane: number}
export const settings = {size: 24, density: 80, area: 75, opacity: 90, speed: 150, color: '#ffffff'}
export const overlay = document.querySelector<HTMLCanvasElement>('#overlay')!
const ctx = overlay.getContext('2d')!
const pending: {message: Message; time: number}[] = []
let sprites: Sprite[] = []
let frame = 0
let previous = 0
let width = 0
let height = 0
const reduced = matchMedia('(prefers-reduced-motion: reduce)')

export function resetOverlay() {
  cancelAnimationFrame(frame)
  frame = 0
  pending.length = 0
  sprites = []
  ctx.clearRect(0, 0, width, height)
}
export function removeMessage(id: string) {
  for (let i = pending.length - 1; i >= 0; i--)
    if (pending[i].message.event_id === id) pending.splice(i, 1)
  sprites = sprites.filter(s => s.id !== id)
}
export function enqueue(message: Message) {
  if (document.hidden || !settings.density) return
  if (pending.length >= 200) pending.shift()
  pending.push({message, time: performance.now()})
  if (!frame) {
    previous = performance.now()
    frame = requestAnimationFrame(draw)
  }
}
function rasterize({content: text, color}: Message) {
  const font = '600 ' + settings.size + 'px system-ui, sans-serif'
  ctx.font = font
  const image = document.createElement('canvas')
  image.width = Math.max(1, Math.min(Math.ceil(ctx.measureText(text).width) + 6, 4096))
  image.height = settings.size + 12
  const ink = image.getContext('2d')!
  ink.font = font
  ink.textBaseline = 'middle'
  ink.lineWidth = 3
  ink.strokeText(text, 3, image.height / 2, image.width - 5)
  ink.fillStyle = settings.color.toLowerCase() === '#ffffff' && color && /^#[0-9a-f]{6}$/i.test(color)
    ? color : settings.color
  ink.fillText(text, 3, image.height / 2, image.width - 5)
  return image
}
function draw(now: number) {
  frame = 0
  const step = Math.max(0, now - previous) * settings.speed / 1000
  previous = now
  const pitch = settings.size + 12
  const lanes = Math.floor(height * settings.area / 100 / pitch)
  const occupied = new Set<number>()
  ctx.clearRect(0, 0, width, height)
  ctx.globalAlpha = settings.opacity / 100
  sprites = sprites.filter(s => s.lane < lanes && s.x + s.image.width > 0)
  for (const s of sprites) {
    s.x -= step
    if (reduced.matches || s.x + s.image.width + 16 > width) occupied.add(s.lane)
  }
  while (pending.length && now - pending[0].time > 1200) pending.shift()
  let admitted = 0
  for (let lane = 0; lane < lanes && pending.length && sprites.length < settings.density && admitted < 4; lane++) {
    if (occupied.has(lane)) continue
    const {message} = pending.shift()!
    sprites.push({id: message.event_id, image: rasterize(message), x: width, lane})
    admitted++
  }
  for (const s of sprites) ctx.drawImage(s.image, reduced.matches ? 3 : Math.round(s.x), s.lane * pitch)
  if (sprites.length || pending.length) frame = requestAnimationFrame(draw)
}
new ResizeObserver(() => {
  width = overlay.clientWidth
  height = overlay.clientHeight
  const scale = Math.min(devicePixelRatio, 1.5)
  overlay.width = Math.round(width * scale)
  overlay.height = Math.round(height * scale)
  ctx.setTransform(scale, 0, 0, scale, 0, 0)
}).observe(overlay)
document.addEventListener('visibilitychange', () => { if (document.hidden) resetOverlay() })
