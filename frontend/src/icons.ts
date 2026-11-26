import {
  Add,
  AngleRight,
  ArrowLeft,
  ChatDots,
  Clock,
  Code,
  Compass,
  Expand,
  Flame,
  Gamepad,
  Gift,
  Globe,
  Grid,
  Heart,
  History,
  Menu,
  Message,
  Monitor,
  Music,
  Palette,
  PlayStream,
  Search,
  Send,
  Settings,
  Sparkles,
  Trophy,
  Tv,
  User,
  Users,
} from 'reicon'

const shapes = {
  Add,
  AngleRight,
  ArrowLeft,
  ChatDots,
  Clock,
  Code,
  Compass,
  Expand,
  Flame,
  Gamepad,
  Gift,
  Globe,
  Grid,
  Heart,
  History,
  Menu,
  Message,
  Monitor,
  Music,
  Palette,
  PlayStream,
  Search,
  Send,
  Settings,
  Sparkles,
  Trophy,
  Tv,
  User,
  Users,
}

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
    '.brand': 'Tv',
    '.idle-screen': 'Monitor',
    '#menuToggle': 'Menu',
    '#plazaOpen': 'Compass',
    '#watchOpen': 'PlayStream',
    '#staffOpen': 'Settings',
    '#accountOpen': 'User',
    '#replaysOpen': 'History',
    '#fullscreen': 'Expand',
    '#heading': 'Message',
    '#like': 'Heart',
    '#createToggle': 'Add',
    '#plazaBack': 'ArrowLeft',
    '.search-icon': 'Search',
  }
  document.querySelectorAll('details > summary').forEach((summary) => summary.prepend(icon('AngleRight')))
  for (const [selector, name] of Object.entries(controls)) {
    document.querySelector(selector)?.prepend(icon(name))
  }
  document.querySelectorAll<HTMLElement>('[data-icon]').forEach((node) => {
    const name = node.dataset.icon as IconName | undefined
    if (name && name in shapes) node.prepend(icon(name))
  })
}
