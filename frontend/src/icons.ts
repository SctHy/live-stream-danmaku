import { Menu, Grid, Palette, Monitor, User, History, Compass, Settings, Expand, Gift, Message, Tv, AngleRight } from 'reicon'

const shapes = { Menu, Grid, Palette, Monitor, User, History, Compass, Settings, Expand, Gift, Message, Tv, AngleRight }
export type IconName = keyof typeof shapes

export function icon(name: IconName) {
  const frame = document.createElement('span')
  frame.className = 'icon-frame'
  frame.setAttribute('aria-hidden', 'true')
  frame.append(shapes[name]({ size: 18, weight: 'Outline', strokeWidth: 1.5 }))
  return frame
}
export function decorateRoom() {
  const controls: Record<string, IconName> = {
    '.brand': 'Tv', '.idle-screen': 'Monitor', '#menuToggle': 'Menu', '#plazaOpen': 'Compass',
    '#staffOpen': 'Settings', '#accountOpen': 'User', '#replaysOpen': 'History', '#fullscreen': 'Expand',
    '#heading': 'Message',
  }
  document.querySelectorAll('details > summary').forEach(summary => summary.prepend(icon('AngleRight')))
  for (const [selector, name] of Object.entries(controls)) {
    document.querySelector(selector)?.prepend(icon(name))
  }
}
