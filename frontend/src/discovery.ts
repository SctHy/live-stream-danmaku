import { taxonomy, categoryOptions, roomFields } from './taxonomy'
import { api, base, button, el, enter, list, onForm, result, room, run, session, type Room } from './business'
import { loggedIn, openAccount } from './account'
import { roomCover, roomInitial } from './covers'
import { icon, type IconName } from './icons'

type PlazaRoom = Room & { online: number; heat: number; recommend_reason: string }

const plazaView = el('plaza')
const watchView = el('watchView')
const sort = el<HTMLSelectElement>('sort')
const filter = el<HTMLSelectElement>('filter')
const category = el<HTMLSelectElement>('categoryFilter')
const strategy = el<HTMLSelectElement>('strategy')
const searchInput = el<HTMLInputElement>('searchInput')
const plazaOpenBtn = el('plazaOpen')
const watchOpenBtn = el('watchOpen')
const sortRail = el('plazaSortRail')
const catRail = el('plazaCatRail')
const liveRail = el('railLive')

const categoryIcons: Record<string, IconName> = {
  '': 'Compass',
  chat: 'ChatDots',
  gaming: 'Gamepad',
  coding: 'Code',
  music: 'Music',
  outdoor: 'Globe',
}

let cachedRooms: PlazaRoom[] = []
let cachedCategories: { code: string; name: string }[] = []
let liveChannelsLoaded = false
let searchQuery = ''
let version = 0

export const isPlazaOpen = () => !plazaView.hidden

function syncNavState() {
  const onPlaza = isPlazaOpen()
  plazaOpenBtn.setAttribute('aria-current', onPlaza ? 'page' : 'false')
  watchOpenBtn.setAttribute('aria-current', onPlaza ? 'false' : 'page')
  document.body.dataset.view = onPlaza ? 'plaza' : 'watch'
  document.title = onPlaza ? 'Maku · 直播广场' : 'Maku · 弹幕放映室'
}

function syncRailState() {
  sortRail.querySelectorAll<HTMLButtonElement>('button[data-sort]').forEach((b) => {
    b.setAttribute('aria-current', String(b.dataset.sort === sort.value))
  })
  catRail.querySelectorAll<HTMLButtonElement>('button[data-cat]').forEach((b) => {
    b.setAttribute('aria-current', String(sort.value !== 'following' && (b.dataset.cat || '') === category.value))
  })
  const sortLabel = sort.selectedOptions[0]?.textContent || '直播广场'
  const catLabel = category.value ? category.selectedOptions[0]?.textContent : ''
  el('plazaHeading').textContent =
    sort.value === 'following' ? '我的关注' : catLabel ? catLabel : sort.value === 'hot' ? '直播广场' : sortLabel
}

function renderRooms(rooms: PlazaRoom[], degraded: boolean) {
  const q = searchQuery.trim().toLowerCase()
  const filtered = q
    ? rooms.filter((r) =>
        [r.title, r.slug, r.description, ...(r.tags || [])].some((v) => (v || '').toLowerCase().includes(q)),
      )
    : rooms

  el('rooms').replaceChildren(
    ...filtered.map((r) => {
      const catName = cachedCategories.find((c) => c.code === r.category)?.name || r.category
      const isLive = r.status === 'live'
      const card = button('', async () => enter(r))
      card.className = 'stream-card'
      card.title = [r.description, catName, ...(r.tags || []).map((t) => '#' + t), r.recommend_reason]
        .filter(Boolean)
        .join(' · ')

      const cover = document.createElement('span')
      cover.className = 'cover'
      cover.dataset.theme = r.cover_theme
      cover.dataset.live = String(isLive)

      const img = document.createElement('img')
      img.src = roomCover(r.cover_theme, r.category, r.slug)
      img.alt = ''
      img.loading = 'lazy'
      img.decoding = 'async'

      const statusBadge = document.createElement('span')
      statusBadge.className = isLive ? 'cover-badge live' : 'cover-badge'
      statusBadge.textContent = isLive ? '直播中' : '已关播'

      const cornerBadge = document.createElement('span')
      cornerBadge.className = 'cover-corner'
      cornerBadge.textContent = isLive ? r.online + ' 观看' : catName

      cover.append(img, statusBadge, cornerBadge)

      const body = document.createElement('div')
      body.className = 'card-body'

      const avatar = document.createElement('span')
      avatar.className = 'card-avatar'
      avatar.setAttribute('aria-hidden', 'true')
      avatar.textContent = roomInitial(r.title, r.slug)

      const info = document.createElement('div')
      info.className = 'card-info'

      const title = document.createElement('strong')
      title.className = 'card-title'
      title.textContent = r.title

      const meta = document.createElement('small')
      meta.textContent = [r.slug, catName, '热度 ' + r.heat].filter(Boolean).join(' · ')

      info.append(title, meta)
      body.append(avatar, info)
      card.append(cover, body)
      return card
    }),
  )

  result(
    degraded
      ? '房间资料暂不可用，仍可进入活跃房间'
      : filtered.length
        ? ''
        : q
          ? '没有匹配的直播间'
          : '暂无房间',
  )
}

function renderLiveRail(rooms: PlazaRoom[]) {
  const live = rooms.filter((r) => r.status === 'live').slice(0, 6)
  liveRail.hidden = !live.length
  if (!live.length) return
  liveRail.replaceChildren(
    ...live.map((r) => {
      const item = button('', async () => enter(r))
      item.className = 'rail-channel'
      const thumb = document.createElement('span')
      thumb.className = 'rail-avatar'
      thumb.textContent = roomInitial(r.title, r.slug)
      const name = document.createElement('span')
      name.className = 'rail-name'
      name.textContent = r.title
      const count = document.createElement('span')
      count.className = 'rail-online'
      count.textContent = String(r.online)
      item.append(thumb, name, count)
      return item
    }),
  )
}

function buildCategoryRail(categories: { code: string; name: string }[]) {
  if (catRail.childElementCount > 0) return
  const entries = [{ code: '', name: '全部分区' }, ...categories]
  catRail.replaceChildren(
    ...entries.map((c) => {
      const b = document.createElement('button')
      b.type = 'button'
      b.dataset.cat = c.code
      b.append(icon(categoryIcons[c.code] || 'Compass'), c.name)
      b.onclick = () => {
        if (sort.value === 'following') sort.value = 'hot'
        category.value = c.code
        void run(() => plaza())
      }
      return b
    }),
  )
}

async function plaza(selectedCategory?: string) {
  const current = ++version
  const data = await taxonomy()
  if (current !== version) return
  cachedCategories = data.categories
  if (category.options.length === 1) {
    category.append(...categoryOptions(data))
    strategy.append(...data.strategies.map((s) => new Option(s.label + ' · ' + s.name, s.name)))
    el<HTMLFormElement>('create').querySelector('select')!.replaceChildren(...categoryOptions(data))
  }
  buildCategoryRail(data.categories)
  if (selectedCategory !== undefined) category.value = selectedCategory
  strategy.hidden = sort.value !== 'recommend'
  const following = sort.value === 'following'
  category.hidden = filter.hidden = following
  syncRailState()

  if (following && !loggedIn()) {
    cachedRooms = []
    el('rooms').replaceChildren()
    el('effectiveStrategy').textContent = ''
    result('登录后可查看关注的主播')
    return
  }

  const params = new URLSearchParams({ sort: sort.value, status: filter.value })
  if (category.value) params.set('category', category.value)
  if (!strategy.hidden && strategy.value) params.set('strategy', strategy.value)
  const response = await api<{ rooms: PlazaRoom[]; degraded: boolean; strategy: string }>(
    following ? '/api/me/following' : '/api/rooms?' + params,
  )
  if (current !== version) return
  cachedRooms = response.rooms || []
  el('effectiveStrategy').textContent = strategy.hidden ? '' : '当前策略 · ' + response.strategy
  renderRooms(cachedRooms, response.degraded)

  if (!following && !category.value && filter.value === 'all') {
    liveChannelsLoaded = true
    renderLiveRail(cachedRooms)
  } else if (!liveChannelsLoaded) {
    liveChannelsLoaded = true
    void api<{ rooms: PlazaRoom[] }>('/api/rooms?sort=online&status=live')
      .then((res) => renderLiveRail(res.rooms || []))
      .catch(() => {})
  }
}

export function openPlaza(selectedCategory?: string, following = false, push = true) {
  if (following || selectedCategory !== undefined) {
    sort.value = following ? 'following' : 'hot'
    filter.value = 'all'
  }
  plazaView.hidden = false
  watchView.hidden = true
  el('plaza').querySelector('.result')!.textContent = ''
  if (push && location.pathname !== '/plaza') {
    history.pushState({ view: 'plaza' }, '', '/plaza')
  }
  syncNavState()
  window.scrollTo(0, 0)
  void run(() => plaza(selectedCategory))
}

export function closePlaza(push = true) {
  if (plazaView.hidden) return
  plazaView.hidden = true
  watchView.hidden = false
  if (push && location.pathname === '/plaza') {
    const url = '/?' + new URLSearchParams({ room, session }).toString()
    history.pushState({ view: 'watch' }, '', url)
  }
  syncNavState()
}

export function initPlazaView() {
  const qs = new URLSearchParams(location.search)
  const startOnPlaza = location.pathname === '/plaza' || qs.get('view') === 'plaza'
  if (startOnPlaza) {
    openPlaza(undefined, false, false)
  } else {
    plazaView.hidden = true
    watchView.hidden = false
    syncNavState()
  }
}

window.addEventListener('popstate', () => {
  if (location.pathname === '/plaza' || new URLSearchParams(location.search).get('view') === 'plaza') {
    openPlaza(undefined, false, false)
  } else {
    closePlaza(false)
  }
})

el('createToggle').onclick = () => {
  if (!loggedIn()) return openAccount()
  const box = el('createBox')
  box.hidden = !box.hidden
  el('createToggle').setAttribute('aria-expanded', String(!box.hidden))
}

window.addEventListener('accountchange', () => {
  el('createBox').hidden = true
  el('createToggle').setAttribute('aria-expanded', 'false')
  if (isPlazaOpen()) void run(() => plaza())
})

sortRail.querySelectorAll<HTMLButtonElement>('button[data-sort]').forEach((b) => {
  b.onclick = () => {
    sort.value = b.dataset.sort || 'hot'
    void run(() => plaza())
  }
})

plazaOpenBtn.onclick = () => openPlaza()
watchOpenBtn.onclick = () => closePlaza()
el('plazaBack').onclick = () => closePlaza()
document.querySelector<HTMLAnchorElement>('.brand')!.onclick = (e) => {
  e.preventDefault()
  openPlaza()
}

searchInput.oninput = () => {
  searchQuery = searchInput.value
  if (isPlazaOpen()) renderRooms(cachedRooms, false)
}

el<HTMLFormElement>('globalSearch').onsubmit = (e) => {
  e.preventDefault()
  const value = searchInput.value.trim()
  if (!value) {
    if (!isPlazaOpen()) openPlaza()
    return
  }
  const exactRoom = cachedRooms.find((r) => r.slug.toLowerCase() === value.toLowerCase())
  if (exactRoom) {
    enter(exactRoom)
    return
  }
  if (!isPlazaOpen() && /^[a-zA-Z0-9_-]{1,24}$/.test(value)) {
    location.href = '/?' + new URLSearchParams({ room: value, session: 'screening-01' }).toString()
    return
  }
  searchQuery = value
  if (!isPlazaOpen()) openPlaza()
  else renderRooms(cachedRooms, false)
}

category.onchange = strategy.onchange = sort.onchange = filter.onchange = () => void run(() => plaza())
onForm('create', async (data) => enter(await api<Room>('/api/rooms', 'POST', roomFields(data))))

let before = 0,
  query = '',
  searchVersion = 0
async function historySearch(reset = false) {
  const current = ++searchVersion
  if (reset) {
    before = 0
    query = el<HTMLFormElement>('search').querySelector('input')!.value.trim()
    el('past').replaceChildren()
    el('moreHistory').hidden = true
  }
  const page = await api<{
    messages: { nickname?: string; user_id: number; content: string }[]
    has_more: boolean
    next_before_id: number
  }>(base + '/history?' + new URLSearchParams({ q: query, before_id: String(before) }))
  if (current !== searchVersion) return
  list(
    'past',
    (page.messages || []).map((m) => (m.nickname || '观众 ' + m.user_id) + ' · ' + m.content),
    !reset,
  )
  before = page.next_before_id
  el('moreHistory').hidden = !page.has_more
}
onForm('search', () => historySearch(true))
el('moreHistory').onclick = () => void run(() => historySearch())
el('clearSearch').onclick = () => {
  el<HTMLFormElement>('search').reset()
  void run(() => historySearch(true))
}
el<HTMLDetailsElement>('history').ontoggle = (e) => {
  if ((e.target as HTMLDetailsElement).open) void run(() => historySearch(true))
}
