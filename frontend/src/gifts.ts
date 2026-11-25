import { api, base, el, list, onForm, result, run, type Contributor, type Order } from './business'
import { loggedIn, openAccount, setBalance } from './account'
interface Gift {
  code: string
  name: string
  price: number
  is_superchat: boolean
}
const select = el<HTMLSelectElement>('gift'),
  qty = el<HTMLInputElement>('quantity'),
  message = el<HTMLInputElement>('sc')
const rank = el<HTMLDetailsElement>('rank')
const pins = new Map<number, HTMLElement>()
export function showGift(g: Order) {
  if (!g.pinned_until || pins.has(g.id)) return
  const remaining = Date.parse(g.pinned_until) - Date.now()
  if (remaining <= 0) return
  const row = document.createElement('p')
  row.textContent = g.sender_name + ' · ' + g.gift_name + '　' + g.message
  el('pins').prepend(row)
  pins.set(g.id, row)
  setTimeout(() => {
    row.remove()
    pins.delete(g.id)
  }, remaining)
}
async function leaderboard() {
  const r = await api<{ contributors: Contributor[] }>(base + '/leaderboard')
  list(
    'contributors',
    (r.contributors || []).map((c) => c.nickname + ' · ' + c.total_cost + ' 积分'),
  )
}
rank.ontoggle = () => {
  if (rank.open) void run(leaderboard)
}
export async function activePins() {
  const r = await api<{ active_superchats: Order[] }>(base + '/gifts')
  for (const g of r.active_superchats || []) showGift(g)
}
async function catalog() {
  const { gifts } = await api<{ gifts: Gift[] }>('/api/gifts/catalog')
  select.replaceChildren(...gifts.map((g) => new Option(g.name + ' · ' + g.price + ' 积分', g.code)))
  select.onchange = () => {
    const sc = gifts[select.selectedIndex].is_superchat
    qty.hidden = qty.disabled = message.required = sc
    message.hidden = !sc
    if (sc) qty.value = '1'
  }
}
void run(catalog)
let lastAttempt: { body: string; id: string } | undefined
onForm('giftForm', async () => {
  if (!loggedIn()) {
    openAccount()
    result('登录后可赠送礼物')
    return
  }
  const order = {
    gift_code: select.value,
    quantity: Number(qty.value),
    message: message.hidden ? '' : message.value,
  }
  const body = JSON.stringify(order)
  if (lastAttempt?.body !== body) lastAttempt = { body, id: crypto.randomUUID() }
  try {
    const r = await api<{ order: Order; balance: number }>(base + '/gifts', 'POST', {
      ...order,
      client_order_id: lastAttempt.id,
    })
    lastAttempt = undefined
    setBalance(r.balance)
    showGift(r.order)
    message.value = ''
    result('礼物已送达')
    await leaderboard()
  } catch (error) {
    if (error instanceof Error && error.message.includes('积分不足')) openAccount()
    throw error
  }
})
