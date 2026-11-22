// Run against a dedicated gateway B and its consumer. A is owned by this test.
// playwright is a test-only dependency, resolvable locally or via NODE_PATH.
const { chromium } = require('playwright')
const net = require('node:net')
const { spawn } = require('node:child_process')
const { once } = require('node:events')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
const { setTimeout: sleep } = require('node:timers/promises')

async function check() {
  const { GATEWAY_BIN, DANMAKU_MOD_TOKEN, DANMAKU_MOD_ROOM } = process.env
  assert(
    GATEWAY_BIN && DANMAKU_MOD_TOKEN && DANMAKU_MOD_ROOM,
    'set gateway executable and moderator configuration',
  )
  const a = Number(process.env.GATEWAY_A_PORT || 38701)
  const b = Number(process.env.GATEWAY_B_PORT || 38502)
  assert.notEqual(a, b)
  const origin = 'http://127.0.0.1:' + b
  const session = randomUUID().slice(0, 18)
  const search = new URLSearchParams({ room: DANMAKU_MOD_ROOM, session })
  const path = '/api/rooms/' + DANMAKU_MOD_ROOM + '/' + session
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH,
  })
  const proxies = []
  let gateway
  try {
    assert.equal((await fetch(origin + '/readyz')).status, 200)
    const available = net.createServer().listen(a, '127.0.0.1')
    await once(available, 'listening')
    await new Promise((resolve) => available.close(resolve))
    gateway = spawn(GATEWAY_BIN, ['-port', String(a)], { stdio: 'ignore' })
    const exit = once(gateway, 'exit')
    for (let i = 0; i < 100; i++) {
      if (gateway.exitCode !== null)
        throw Error('gateway A exited during startup')
      if (
        await fetch('http://127.0.0.1:' + a + '/readyz')
          .then((r) => r.ok)
          .catch(() => false)
      )
        break
      await sleep(100)
    }
    // Two stable browser origins, each switched to B at a different time.
    for (let i = 0; i < 2; i++) {
      const proxy = { target: a }
      proxy.server = net
        .createServer((socket) => {
          if (!proxy.target) return socket.destroy()
          const upstream = net.connect(proxy.target, '127.0.0.1')
          socket.on('error', () => upstream.destroy())
          upstream.on('error', () => socket.destroy())
          socket.on('close', () => upstream.destroy())
          upstream.on('close', () => socket.destroy())
          socket.pipe(upstream).pipe(socket)
        })
        .listen(0, '127.0.0.1')
      proxies.push(proxy)
      await once(proxy.server, 'listening')
      proxy.page = await browser.newPage()
      await proxy.page.goto(
        'http://127.0.0.1:' + proxy.server.address().port + '/?' + search,
      )
      await proxy.page.waitForFunction(
        () => !document.querySelector('#send').disabled,
      )
    }
    const sender = await browser.newPage()
    await sender.goto(origin + '/?' + search)
    await sender.waitForFunction(
      () => !document.querySelector('#send').disabled,
    )
    const send = async (text) => {
      await sender.locator('#message').fill(text)
      await sender.locator('#send').click()
      await sender.waitForFunction(
        (t) =>
          Array.from(document.querySelectorAll('#messages li')).some(
            (e) => e.lastChild.textContent === t,
          ),
        text,
      )
    }
    await send('withdraw-me')
    for (const proxy of proxies) {
      await proxy.page
        .getByText('withdraw-me', { exact: true })
        .first()
        .waitFor()
      proxy.target = 0
    }
    const before = await (await fetch(origin + path + '/recent')).json()
    const target = before.events.find(
      (p) => p.data.content === 'withdraw-me',
    ).data
    const id = target.event_id
    gateway.kill('SIGKILL')
    await exit
    const response = await fetch(origin + path + '/moderate', {
      method: 'POST',
      headers: {
        Authorization: 'Bearer ' + DANMAKU_MOD_TOKEN,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ action: 'withdraw', event_id: id }),
    })
    assert.equal(response.status, 204)
    async function verify(proxy, reset, expected) {
      const recent = await (
        await fetch(
          origin + path + '/recent?after=' + encodeURIComponent(before.pos),
        )
      ).json()
      assert.equal(recent.reset, reset)
      assert(!recent.events.some((p) => p.data.content === 'withdraw-me'))
      const start = Date.now()
      proxy.target = b
      await proxy.page.waitForFunction(
        (t) =>
          !document.querySelector('#send').disabled &&
          document.querySelector('#connection').textContent === '已连接' &&
          document.querySelector('#messages li')?.lastChild.textContent === t,
        expected.at(-1),
      )
      const texts = await proxy.page
        .locator('#messages li')
        .evaluateAll((rows) => rows.map((r) => r.lastChild.textContent))
      assert.deepEqual(texts, expected.slice(-100).reverse())
      console.log(
        JSON.stringify({
          reset,
          visible: texts.length,
          reconnect_ms: Date.now() - start,
        }),
      )
    }
    await send('retained')
    await verify(proxies[0], false, ['retained'])
    await proxies[0].page.close()
    const expected = ['retained']
    for (let i = 0; i < 360; i++) {
      const text = 'roll-' + i
      await send(text)
      expected.push(text)
      await sleep(145)
    }
    await verify(proxies[1], true, expected)
  } finally {
    gateway?.kill('SIGTERM')
    await browser.close()
    for (const p of proxies) p.server.close()
  }
}
check().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
