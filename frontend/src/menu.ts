import { icon, type IconName } from './icons'
import { taxonomy } from './taxonomy'
import { el, result, run } from './business'
import { openPlaza } from './discovery'

interface Point { x: number; y: number }
export function pointInSafeTriangle(p: Point, origin: Point, top: Point, bottom: Point) {
  const progress = (p.x - origin.x) / (top.x - origin.x)
  if (!(progress > 0 && progress <= 1)) return false
  return p.y >= origin.y + progress * (top.y - origin.y) &&
    p.y <= origin.y + progress * (bottom.y - origin.y)
}

interface Action { label: string; act: () => void; disabled?: boolean }
export function installMenu(refresh: () => void) {
  const root = el('menuRoot'), toggle = el('menuToggle'), panel = el('menuPanel')
  const parents = el('menuParents'), children = el('menuChildren')
  const options = (id: string): Action[] => {
    const select = el<HTMLSelectElement>(id)
    return Array.from(select.options, opt => ({
      label: opt.text, act: () => {
        select.value = opt.value
        select.dispatchEvent(new Event('change', { bubbles: true }))
      },
    }))
  }
  let active = -1, opening = 0, closing = 0
  let pinned = false
  let previous: Point | undefined
  let scheduled = -1, protectedPath = false
  function cancelSwitch() {
    clearTimeout(opening)
    scheduled = -1
  }
  const groups: { label: string; shape: IconName; actions: () => Action[] }[] = [
    { label: '频道分区', shape: 'Grid', actions: () => [{ label: '全部房间', act: () => openPlaza('') }] },
    { label: '弹幕外观', shape: 'Palette', actions: () => [
      ...options('inkMode'), ...options('inkColor'),
      { label: el<HTMLInputElement>('overlayEnabled').checked ? '隐藏弹幕' : '显示弹幕', act: () => el('overlayEnabled').click() },
    ] },
    { label: '直播间', shape: 'Monitor', actions: () => [
      { label: '房间管理', disabled: el('staffOpen').hidden, act: () => el('staffOpen').click() },
      { label: '历史回放', act: () => el('replaysOpen').click() },
      { label: '刷新直播源', disabled: !el('replayBar').hidden, act: refresh },
      { label: '复制当前房间链接', act: () => void run(async () => {
        await navigator.clipboard.writeText(location.href)
        result('已复制房间链接')
      }) },
    ] },
    { label: '个人中心', shape: 'User', actions: () => [
      { label: '账号与钱包', act: () => el('accountOpen').click() },
      { label: '我的关注', act: () => openPlaza(undefined, true) },
      { label: '直播广场', act: () => openPlaza() },
    ] },
  ]
  function close() {
    cancelSwitch()
    clearTimeout(closing)
    if (active >= 0) parents.children[active].setAttribute('aria-expanded', 'false')
    active = -1
    previous = undefined
    pinned = false
    children.hidden = panel.hidden = true
    toggle.setAttribute('aria-expanded', 'false')
  }
  function open() {
    panel.hidden = false
    toggle.setAttribute('aria-expanded', 'true')
  }
  function appendAction(action: Action) {
    const button = document.createElement('button')
    button.textContent = action.label
    button.disabled = !!action.disabled
    button.onclick = () => { close(); toggle.focus(); action.act() }
    children.append(button)
  }
  function select(index: number) {
    cancelSwitch()
    if (index === active) return
    if (active >= 0) parents.children[active].setAttribute('aria-expanded', 'false')
    active = index
    parents.children[index].setAttribute('aria-expanded', 'true')
    children.replaceChildren()
    groups[index].actions().forEach(appendAction)
    children.hidden = false
    panel.classList.toggle('flip', panel.getBoundingClientRect().right + children.offsetWidth > innerWidth)
    if (index === 0) void run(async () => {
      const data = await taxonomy()
      if (active === 0 && children.childElementCount === 1) {
        data.categories.forEach(c => appendAction({ label: c.name, act: () => openPlaza(c.code) }))
      }
    })
  }
  groups.forEach((group, index) => {
    const button = document.createElement('button')
    button.append(icon(group.shape), group.label, icon('AngleRight'))
    button.setAttribute('aria-expanded', 'false')
    button.onclick = () => select(index)
    parents.append(button)
  })
  parents.onpointermove = event => {
    const button = (event.target as HTMLElement).closest('button')
    const index = Array.from(parents.children).indexOf(button!)
    const point = { x: event.clientX, y: event.clientY }
    const rect = children.getBoundingClientRect()
    const x = panel.classList.contains('flip') ? rect.right : rect.left
    const safe = active >= 0 && previous && pointInSafeTriangle(point, previous,
      { x, y: rect.top - 4 }, { x, y: rect.bottom + 4 })
    previous = point
    if (index < 0 || index === active) return cancelSwitch()
    if (index === scheduled && protectedPath === Boolean(safe)) return
    cancelSwitch()
    scheduled = index
    protectedPath = Boolean(safe)
    opening = window.setTimeout(() => select(index), safe ? 320 : 120)
  }
  children.onpointerenter = cancelSwitch
  root.onpointerenter = () => clearTimeout(closing)
  root.onpointerleave = () => {
    cancelSwitch()
    closing = window.setTimeout(close, 240)
  }
  toggle.onpointerenter = event => {
    if (event.pointerType === 'mouse') opening = window.setTimeout(open, 120)
  }
  toggle.onclick = () => {
    cancelSwitch()
    if (pinned) return close()
    pinned = true
    open()
  }
  document.addEventListener('pointerdown', e => { if (!root.contains(e.target as Node)) close() })
  root.addEventListener('focusout', e => { if (!root.contains(e.relatedTarget as Node)) close() })
  root.onkeydown = e => {
    if (e.key === 'Escape') { close(); toggle.focus() }
    if (e.key === 'ArrowRight' && parents.contains(document.activeElement)) {
      select(Array.from(parents.children).indexOf(document.activeElement!))
      children.querySelector<HTMLButtonElement>('button:enabled')?.focus()
    }
    if (e.key === 'ArrowLeft' && active >= 0) (parents.children[active] as HTMLElement).focus()
  }
}
