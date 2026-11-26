import { taxonomy, categoryOptions, roomFields } from './taxonomy'
import { roomInitial } from './covers'
import {
  api,
  base,
  button,
  el,
  enter,
  list,
  onForm,
  result,
  roomPath,
  run,
  session,
  showDialog,
  type Contributor,
  type Controls,
  type Room,
} from './business'
import {accountID, loggedIn, openAccount} from './account'
let current: Room | undefined
let liveControls: Controls | undefined
function input(form: string, field: string) {
  return el<HTMLFormElement>(form).elements.namedItem(field) as HTMLInputElement
}
export function state(c: Controls) {
  liveControls = c
  const role = current?.role
  el('state').textContent = [
    c.status === 'live' ? '直播中' : '已关播',
    role === 'owner' ? '房主' : role === 'moderator' ? '房管' : '',
    c.mute_all ? '全员禁言' : '',
    c.slow_mode_seconds ? '慢速模式 · ' + c.slow_mode_seconds + ' 秒' : '',
  ]
    .filter(Boolean)
    .join(' · ')
  el('notice').textContent = c.notice
}
let revision = 0
export async function roomInfo() {
  const version = ++revision
  const room = await api<Room>(roomPath)
  if (version !== revision) return
  current = room
  el('title').textContent = current.title
  const avatar = document.getElementById('roomAvatar')
  if (avatar) avatar.textContent = roomInitial(current.title, current.slug)
  el('staffOpen').hidden = !['owner', 'moderator'].includes(current.role)
  document.body.classList.toggle('staff', !el('staffOpen').hidden)
  el('owner').hidden = current.role !== 'owner'
  el('moderation-panel').hidden = !el('staffOpen').hidden
  el('follow').hidden = false
  el('follow').textContent = (current.following ? '已关注' : '关注') + ' · ' + current.follower_count + ' 粉丝'
  state(liveControls || room)
  const data = await taxonomy().catch(() => undefined)
  if (version !== revision) return
  const category = data?.categories.find(c => c.code === room.category)?.name || room.category
  el('roomTags').textContent = [category, ...(room.tags || []).map(t => '#' + t)].filter(Boolean).join(' · ')
  return current
}
el('follow').onclick = () => {
  if (!loggedIn()) return openAccount()
  const b = el<HTMLButtonElement>('follow')
  b.disabled = true
  void run(async () => {
    await api(roomPath + '/follow', current?.following ? 'DELETE' : 'POST')
    await roomInfo()
  }).finally(() => { b.disabled = false })
}
async function moderate(data: object) {
  await api(base + '/moderate', 'POST', data)
  result('房管操作已执行')
}
export function actions(event_id: string, user_id: number) {
  const group = document.createElement('span')
  group.className = 'row-actions'
  group.append(
    button('撤回', () => moderate({ action: 'withdraw', event_id })),
  )
  if (user_id !== accountID()) group.append(
    button('禁言 10m', () => moderate({ action: 'mute', user_id, seconds: 600 })),
  )
  return group
}
onForm('noticeForm', (data) => moderate({ action: 'notice', notice: data.notice }))
onForm('slowForm', (data) => moderate({ action: 'slow_mode', seconds: Number(data.slow_mode_seconds) }))
onForm('muteForm', (data) => moderate({ action: 'mute_all', enabled: !!data.mute_all }))
onForm('editRoom', async (data) => {
  const updated = await api<Room>(roomPath, 'PATCH', roomFields(data))
  if (updated.active_session !== session) enter(updated)
  else location.reload()
})
async function moderators() {
  const { moderators: members } = await api<{ moderators: { user_id: number; username: string }[] }>(
    roomPath + '/moderators',
  )
  list(
    'mods',
    (members || []).map((x) => x.username),
  )
  members?.forEach((x, i) =>
    el('mods').children[i].append(
      button('撤销', async () => {
        await api(roomPath + '/moderators/' + x.user_id, 'DELETE')
        await moderators()
      }),
    ),
  )
}
onForm('grant', async (data) => {
  await api(roomPath + '/moderators', 'POST', data)
  await moderators()
})
async function controls() {
  if (!current) return
  const c = liveControls || current
  input('noticeForm', 'notice').value = c.notice
  input('slowForm', 'slow_mode_seconds').value = String(c.slow_mode_seconds)
  input('muteForm', 'mute_all').checked = c.mute_all
  if (current.role !== 'owner') return
  for (const key of ['title', 'description', 'active_session', 'bilibili_room_id', 'cover_theme'] as const)
    input('editRoom', key).value = current[key]
  const categories = el<HTMLFormElement>('editRoom').querySelector<HTMLSelectElement>('[name=category]')!
  categories.replaceChildren(...categoryOptions(await taxonomy()))
  categories.value = current.category
  input('editRoom', 'tags').value = (current.tags || []).join(', ')
  input('editRoom', 'status').value = c.status
  await Promise.all([moderators(), credentials(false)])
}
async function mutes() {
  const r = await api<{ mutes: { user_id: number; expires_at: number }[] }>(base + '/mutes')
  list(
    'mutes',
    (r.mutes || []).map(
      (x) => '观众 ' + x.user_id + ' · 至 ' + new Date(x.expires_at * 1000).toLocaleTimeString(),
    ),
  )
  r.mutes?.forEach((x, i) =>
    el('mutes').children[i].append(
      button('解除', async () => {
        await moderate({ action: 'unmute', user_id: x.user_id })
        await mutes()
      }),
    ),
  )
}
let before = 0,
  logVersion = 0
async function logs(reset = false) {
  const revision = ++logVersion
  if (reset) before = 0
  const page = await api<{
    audits: {
      action: string
      created_at: string
      actor_id: number
      target_user_id: number
      target_event_id: string
    }[]
    has_more: boolean
    next_before_id: number
  }>(base + '/audits?before_id=' + before)
  if (revision !== logVersion) return
  list(
    'logs',
    (page.audits || []).map((a) =>
      [
        new Date(a.created_at).toLocaleString(),
        a.action,
        '操作人 ' + a.actor_id,
        a.target_event_id || a.target_user_id || '',
      ].join(' · '),
    ),
    !reset,
  )
  before = page.next_before_id
  el('moreLogs').hidden = !page.has_more
}
el('moreLogs').onclick = () => void run(() => logs())
const labels = {
  online: '当前在线',
  peak_online: '峰值在线',
  danmaku_count: '已归档弹幕',
  active_senders: '活跃发言人数',
  withdrawn_count: '撤回数',
  moderation_count: '管控次数',
  gift_order_count: '礼物订单',
  gift_total_cost: '礼物总积分',
}
async function analytics() {
  const a = await api<Record<keyof typeof labels, number> & { top_contributors: Contributor[] }>(
    base + '/analytics',
  )
  el('metrics').replaceChildren(
    ...Object.entries(labels).map(([k, label]) => {
      const card = document.createElement('div'),
        value = document.createElement('strong')
      value.textContent = String(a[k as keyof typeof labels])
      card.append(label, value)
      return card
    }),
  )
  list(
    'top',
    (a.top_contributors || []).map((c) => c.nickname + ' · ' + c.total_cost + ' 积分'),
  )
}
async function tab(id: string) {
  document.querySelectorAll<HTMLButtonElement>('[data-tab]').forEach((b) => {
    const active = b.dataset.tab === id
    b.setAttribute('aria-pressed', String(active))
    el(b.dataset.tab!).hidden = !active
  })
  if (id === 'control') await controls()
  if (id === 'audit') await Promise.all([mutes(), logs(true)])
  if (id === 'analytics') await analytics()
}
el('staffOpen').onclick = () => {
  showDialog('staff')
  void run(() => tab('control'))
}
document
  .querySelectorAll<HTMLButtonElement>('[data-tab]')
  .forEach((b) => (b.onclick = () => void run(() => tab(b.dataset.tab!))))

const server = el<HTMLInputElement>('obsServer')
const key = el<HTMLInputElement>('obsKey')
function maskKey() {
  key.type = 'password'
  el('showKey').textContent = '显示'
}
async function credentials(rotate: boolean) {
  const version = revision
  const data = await api<{obs_server: string; obs_stream_key: string}>(
    roomPath + '/stream-key' + (rotate ? '/rotate' : ''), rotate ? 'POST' : 'GET',
  )
  if (version !== revision) return
  maskKey()
  server.value = data.obs_server
  key.value = data.obs_stream_key
  el('obsStatus').textContent = rotate ? '推流码已重置，请更新 OBS 配置' : '推流配置已就绪'
}
window.addEventListener('accountchange', () => {
  revision++
  current = undefined
  key.value = server.value = ''
  maskKey()
})
el('showKey').onclick = () => {
  if (key.type === 'text') maskKey()
  else {
    key.type = 'text'
    el('showKey').textContent = '隐藏'
  }
}
el('staff').addEventListener('close', maskKey)
el('rotateKey').onclick = () => {
  if (confirm('重置后，旧推流码将无法用于开播。继续吗？')) void run(() => credentials(true))
}
for (const [id, field] of [['copyServer', server], ['copyKey', key]] as const)
  el(id).onclick = () => void run(async () => {
    await navigator.clipboard.writeText(field.value)
    el('obsStatus').textContent = '已复制'
  })
