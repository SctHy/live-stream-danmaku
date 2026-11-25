import { api, base, button, el, enter, list, onForm, result, run, showDialog, type Room } from './business'
const sort = el<HTMLSelectElement>('sort'),
  filter = el<HTMLSelectElement>('filter')
let version = 0
async function plaza() {
  const current = ++version
  const response = await api<{ rooms: (Room & { online: number; heat: number })[]; degraded: boolean }>(
    '/api/rooms?' + new URLSearchParams({ sort: sort.value, status: filter.value }),
  )
  if (current !== version) return
  el('rooms').replaceChildren(
    ...response.rooms.map((r) => {
      const card = button(r.title, async () => enter(r))
      const meta = document.createElement('small')
      meta.textContent = [
        r.slug,
        r.status === 'live' ? '直播中' : '已关播',
        r.online + ' 在线',
        '热度 ' + r.heat,
      ].join(' · ')
      card.append(meta, document.createTextNode(r.description))
      return card
    }),
  )
  result(response.degraded ? '房间资料暂不可用，仍可进入活跃房间' : response.rooms.length ? '' : '暂无房间')
}
el('plazaOpen').onclick = () => {
  showDialog('plaza')
  void run(plaza)
}
sort.onchange = filter.onchange = () => void run(plaza)
onForm('create', async (data) => enter(await api<Room>('/api/rooms', 'POST', data)))
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
