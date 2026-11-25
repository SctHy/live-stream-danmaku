export const el = <T extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as T
const params = new URLSearchParams(location.search)
export const room = params.get('room') || '6'
export const session = params.get('session') || 'screening-01'
export const roomPath = '/api/rooms/' + encodeURIComponent(room)
export const base = roomPath + '/' + encodeURIComponent(session)
export interface Controls {
  notice: string
  mute_all: boolean
  slow_mode_seconds: number
  status: string
}
export interface Room extends Controls {
  slug: string
  title: string
  description: string
  active_session: string
  bilibili_room_id: string
  role: string
}
export interface Order {
  id: number
  sender_name: string
  gift_name: string
  quantity: number
  total_cost: number
  pinned_until: string | null
  message: string
}
export interface Contributor {
  nickname: string
  total_cost: number
}
export function enter(r: Room) {
  location.search = new URLSearchParams({ room: r.slug, session: r.active_session || 'live' }).toString()
}
export async function api<T = void>(url: string, method = 'GET', data?: unknown): Promise<T> {
  const response = await fetch(url, {
    method,
    signal: AbortSignal.timeout(8000),
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!response.ok) {
    const reasons: Record<number, string> = {
      400: '请检查输入内容',
      401: '请登录或核对账号密码',
      403: '当前账号没有操作权限',
      404: '未找到记录',
      409: '记录已存在',
      503: '服务暂时不可用，请稍后重试',
    }
    if (response.status === 409 && url.endsWith('/gifts')) throw Error('积分不足，可在账号与钱包签到')
    throw Error(reasons[response.status] || '请求失败')
  }
  return response.status === 204 ? (undefined as T) : response.json()
}
export function result(text: string) {
  const target = document.querySelector('dialog[open] .result') || el('feedback')
  target.textContent = text
}
export async function run(fn: () => Promise<void>) {
  try {
    await fn()
  } catch (error) {
    result(error instanceof Error ? error.message : '请求失败')
  }
}
export function onForm(
  id: string,
  action: (fields: Record<string, string>, button: string) => Promise<void>,
) {
  const form = el<HTMLFormElement>(id)
  let busy = false
  form.onsubmit = (event) => {
    event.preventDefault()
    if (busy) return
    busy = true
    const fields: Record<string, string> = {}
    new FormData(form).forEach((value, key) => (fields[key] = String(value)))
    void run(() => action(fields, (event.submitter as HTMLButtonElement)?.value)).finally(
      () => (busy = false),
    )
  }
}
export function button(label: string, fn: () => Promise<void>) {
  const b = document.createElement('button')
  b.textContent = label
  b.onclick = () => {
    b.disabled = true
    void run(fn).finally(() => (b.disabled = false))
  }
  return b
}
export function list(id: string, rows: string[], append = false) {
  const parent = el(id)
  if (!append) parent.replaceChildren()
  for (const text of rows) {
    const li = document.createElement('li')
    li.textContent = text
    parent.append(li)
  }
  if (!parent.children.length) parent.textContent = '暂无记录'
}
export function showDialog(id: string) {
  const dialog = el<HTMLDialogElement>(id)
  dialog.querySelector('.result')!.textContent = ''
  dialog.showModal()
}
document
  .querySelectorAll<HTMLButtonElement>('[data-close]')
  .forEach((b) => (b.onclick = () => b.closest('dialog')!.close()))
