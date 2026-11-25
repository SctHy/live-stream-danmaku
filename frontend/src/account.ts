import { api, button, el, list, onForm, result, run, showDialog, type Order } from './business'
interface Account {
  authenticated: boolean
  user_id: number
  nickname: string
  balance: number | null
}
let me: Account | undefined
export const accountID = () => me?.user_id
export const loggedIn = () => !!me?.authenticated
export function setBalance(value: number | null) {
  if (!me) return
  me.balance = value
  const amount = value === null ? '余额暂不可用' : value + ' 积分'
  el('identity').textContent = me.authenticated ? me.nickname + ' · ' + amount : '访客 #' + String(me.user_id).slice(-4).padStart(4,'0')
  el('identity').title = String(me.user_id)
  el('balance').textContent = amount
}
let accountVersion = 0
export async function loadAccount() {
  const version = ++accountVersion
  const account = await api<Account>('/api/auth/me')
  if (version !== accountVersion) return
  me = account
  setBalance(me.balance)
  el('accountOpen').textContent = loggedIn() ? '账号与钱包' : '登录 / 注册'
  el('wallet').hidden = !loggedIn()
  el('auth').hidden = loggedIn()
  el<HTMLFormElement>('rename').querySelector('input')!.value = me.nickname
}
interface Recharge {
  order_no: string
  coins: number
  provider: string
  status: 'pending' | 'paid' | 'closed' | 'expired'
}
interface LedgerEntry {
  biz_type: string
  delta: number
  balance_after: number
}
const businessNames: Record<string, string> = { checkin: '签到', recharge: '充值', voucher: '兑换', gift_debit: '送礼', gift_income: '礼物收益' }
const statuses = { pending: '待支付', paid: '已支付', closed: '已关闭', expired: '已过期' }
let walletVersion = 0
async function wallet() {
  const version = ++walletVersion
  const w = await api<{ balance: number; earnings: number; checked_in_today: boolean; orders: Order[]; recharges: Recharge[]; ledger: LedgerEntry[] }>('/api/wallet')
  if (version !== walletVersion) return
  setBalance(w.balance)
  el('earnings').textContent = '主播收益：' + w.earnings + ' 积分'
  list('ledger', (w.ledger || []).map(e =>
    (businessNames[e.biz_type] || e.biz_type) + ' · ' + (e.delta > 0 ? '+' : '') + e.delta + (e.biz_type === 'gift_income' ? ' · 累计收益 ' : ' · 余额 ') + e.balance_after))
  const recharges = w.recharges || []
  list('recharges', recharges.map(o => o.order_no))
  recharges.forEach((o, index) => {
    const row = el('recharges').children[index]
    const code = document.createElement('code'), status = document.createElement('span')
    code.textContent = code.title = o.order_no
    status.textContent = o.coins + ' 积分 · ' + statuses[o.status]
    row.replaceChildren(code, status)
    if (o.provider !== 'sandbox' || o.status !== 'pending') return
    for (const [action, label] of [['pay', '模拟支付成功'], ['close', '关闭订单']]) {
      row.append(button(label, async () => {
        try {
          await api('/api/wallet/recharges/' + encodeURIComponent(o.order_no) + '/simulate', 'POST', { action })
        } finally {
          await wallet()
        }
      }))
    }
  })
  const claim = el<HTMLButtonElement>('claim')
  claim.disabled = w.checked_in_today
  claim.textContent = w.checked_in_today ? '今日已签到' : '每日签到领积分'
  list(
    'orders',
    (w.orders || []).map((o) => o.gift_name + ' × ' + o.quantity + ' · −' + o.total_cost + ' 积分'),
  )
}
async function packages() {
  const data = await api<{ packages: { package_code: string; amount_cents: number; coins: number }[] }>('/api/wallet/recharge-packages')
  el<HTMLSelectElement>('packages').replaceChildren(...data.packages.map(p =>
    new Option('¥' + (p.amount_cents / 100).toFixed(2) + ' · ' + p.coins + ' 积分', p.package_code)))
}
let attempt: { code: string; id: string } | undefined
onForm('recharge', async ({ package_code }) => {
  if (attempt?.code !== package_code) attempt = { code: package_code, id: crypto.randomUUID() }
  await api('/api/wallet/recharges', 'POST', { package_code, client_order_id: attempt.id, provider: 'sandbox' })
  attempt = undefined
  await wallet()
})
onForm('redeem', async ({ code }) => {
  const response = await api<{ duplicate: boolean }>('/api/wallet/redeem', 'POST', { code: code.trim() })
  await wallet()
  result(response.duplicate ? '此兑换码已领取，积分未重复发放' : '兑换成功')
})
export function openAccount() {
  showDialog('account')
  if (loggedIn()) {
    void run(packages)
    void run(wallet)
  }
}
el('accountOpen').onclick = openAccount
async function refreshIdentity() {
  ++walletVersion
  attempt = undefined
  await loadAccount()
  el<HTMLFormElement>('auth').reset()
  window.dispatchEvent(new Event('accountchange'))
  if (loggedIn()) await Promise.all([wallet(), packages()])
}
onForm('auth', async (data, action) => {
  await api('/api/auth/' + (action === 'login' ? 'login' : 'register'), 'POST', data)
  await refreshIdentity()
})
onForm('rename', async (data) => {
  await api('/api/auth/me', 'PATCH', data)
  await refreshIdentity()
})
el('logout').onclick = () =>
  void run(async () => {
    await api('/api/auth/logout', 'POST')
    await refreshIdentity()
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
