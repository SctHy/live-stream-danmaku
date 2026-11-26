import { taxonomy, categoryOptions, roomFields } from './taxonomy'
import { api, base, button, el, enter, list, onForm, result, run, showDialog, type Room } from './business'
const sort = el<HTMLSelectElement>('sort'),
  filter = el<HTMLSelectElement>('filter')
const category = el<HTMLSelectElement>('categoryFilter')
const strategy = el<HTMLSelectElement>('strategy')
import {loggedIn, openAccount} from './account'
el('createToggle').onclick = () => {
  if (!loggedIn()) return openAccount()
  const box = el('createBox')
  box.hidden = !box.hidden
  el('createToggle').setAttribute('aria-expanded', String(!box.hidden))
}
window.addEventListener('accountchange', () => {
  el('createBox').hidden = true
  el('createToggle').setAttribute('aria-expanded','false')
  if (el<HTMLDialogElement>('plaza').open) void run(plaza)
})
let version = 0
async function plaza(selectedCategory?: string) {
  const current = ++version
  const data = await taxonomy()
  if (current !== version) return
  if (category.options.length === 1) {
    category.append(...categoryOptions(data))
    strategy.append(...data.strategies.map(s => new Option(s.label + ' · ' + s.name, s.name)))
    el<HTMLFormElement>('create').querySelector('select')!.replaceChildren(...categoryOptions(data))
  }
  if (selectedCategory !== undefined) category.value = selectedCategory
  strategy.hidden = sort.value !== 'recommend'
  const following = sort.value === 'following'
  category.hidden = filter.hidden = following
  if (following && !loggedIn()) {
    el('rooms').replaceChildren()
    el('effectiveStrategy').textContent = ''
    result('登录后可查看关注的主播')
    return
  }
  const params = new URLSearchParams({ sort: sort.value, status: filter.value })
  if (category.value) params.set('category', category.value)
  if (!strategy.hidden && strategy.value) params.set('strategy', strategy.value)
  const response = await api<{ rooms: (Room & { online: number; heat: number; recommend_reason: string })[]; degraded: boolean; strategy: string }>(
    following ? '/api/me/following' : '/api/rooms?' + params,
  )
  if (current !== version) return
  el('effectiveStrategy').textContent = strategy.hidden ? '' : '当前策略 · ' + response.strategy
  el('rooms').replaceChildren(
    ...response.rooms.map((r) => {
      const card = button(r.title, async () => enter(r))
      const meta = document.createElement('small')
      meta.textContent = [
        r.slug,

        r.online + ' 在线',
        '热度 ' + r.heat,
      ].join(' · ')
      card.append(meta)
      card.title = [r.description, data.categories.find(c => c.code === r.category)?.name,
        ...(r.tags || []).map(tag => '#' + tag), r.recommend_reason].filter(Boolean).join(' · ')
      const cover = document.createElement('span')
      cover.className = 'cover'
      cover.dataset.theme = r.cover_theme
      cover.dataset.live = String(r.status === 'live')
      cover.textContent = r.status === 'live' ? '直播中' : '已关播'
      card.prepend(cover)
      return card
    }),
  )
  result(response.degraded ? '房间资料暂不可用，仍可进入活跃房间' : response.rooms.length ? '' : '暂无房间')
}
export function openPlaza(selectedCategory?: string, following = false) {
  if (following || selectedCategory !== undefined) {
    sort.value = following ? 'following' : 'hot'
    filter.value = 'all'
  }
  showDialog('plaza')
  void run(() => plaza(selectedCategory))
}
el('plazaOpen').onclick = () => openPlaza()
category.onchange = strategy.onchange = sort.onchange = filter.onchange = () => void run(plaza)
onForm('create', async (data) => enter(await api<Room>('/api/rooms', 'POST', roomFields(data))))
let before = 0,
  query = '',
  searchVersion = 0
async function history(reset = false) {
  const version = ++searchVersion
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
  if (version !== searchVersion) return
  list(
    'past',
    (page.messages || []).map((m) => (m.nickname || '观众 ' + m.user_id) + ' · ' + m.content),
    !reset,
  )
  before = page.next_before_id
  el('moreHistory').hidden = !page.has_more
}
onForm('search', () => history(true))
el('moreHistory').onclick = () => void run(() => history())
el('clearSearch').onclick = () => {
  el<HTMLFormElement>('search').reset()
  void run(() => history(true))
}
el<HTMLDetailsElement>('history').ontoggle = (e) => {
  if ((e.target as HTMLDetailsElement).open) void run(() => history(true))
}
