import { api, el, list, onForm, result, run, showDialog, type Order } from './business'
interface Account {
  authenticated: boolean
  user_id: number
  nickname: string
  balance: number | null
}
let me: Account | undefined
export const loggedIn = () => !!me?.authenticated
export function setBalance(value: number | null) {
  if (!me) return
  me.balance = value
  const amount = value === null ? '余额暂不可用' : value + ' 积分'
  el('identity').textContent = me.authenticated ? me.nickname + ' · ' + amount : '访客 ' + me.user_id
  el('balance').textContent = amount
}
export async function loadAccount() {
  me = await api<Account>('/api/auth/me')
  setBalance(me.balance)
  el('accountOpen').textContent = loggedIn() ? '账号与钱包' : '登录 / 注册'
  el('wallet').hidden = el('createBox').hidden = !loggedIn()
  el('auth').hidden = loggedIn()
  el<HTMLFormElement>('rename').querySelector('input')!.value = me.nickname
}
async function wallet() {
  const w = await api<{ balance: number; checked_in_today: boolean; orders: Order[] }>('/api/wallet')
  setBalance(w.balance)
  const claim = el<HTMLButtonElement>('claim')
  claim.disabled = w.checked_in_today
  claim.textContent = w.checked_in_today ? '今日已签到' : '每日签到领积分'
  list(
    'orders',
    (w.orders || []).map((o) => o.gift_name + ' × ' + o.quantity + ' · −' + o.total_cost + ' 积分'),
  )
}
export function openAccount() {
  showDialog('account')
  if (loggedIn()) void run(wallet)
}
el('accountOpen').onclick = openAccount
onForm('auth', async (data, action) => {
  await api('/api/auth/' + (action === 'login' ? 'login' : 'register'), 'POST', data)
  location.reload()
})
onForm('rename', async (data) => {
  await api('/api/auth/me', 'PATCH', data)
  location.reload()
})
el('logout').onclick = () =>
  void run(async () => {
    await api('/api/auth/logout', 'POST')
    location.reload()
  })
el('claim').onclick = () =>
  void run(async () => {
    const b = el<HTMLButtonElement>('claim')
    b.disabled = true
    try {
      const v = await api<{ claimed: boolean; balance: number }>('/api/wallet/checkin', 'POST')
      setBalance(v.balance)
      result(v.claimed ? '已领取签到积分' : '今日已签到')
      await wallet()
    } catch (e) {
      b.disabled = false
      throw e
    }
  })
