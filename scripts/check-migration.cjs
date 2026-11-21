// Use two free ports and a dedicated backend stack. Both gateways are owned here.
const { chromium } = require('playwright')
const { spawn } = require('node:child_process')
const { once } = require('node:events')
const net = require('node:net')
const assert = require('node:assert/strict')
const { setTimeout: sleep } = require('node:timers/promises')

async function main() {
  const ports = [
    Number(process.env.GATEWAY_A_PORT || 38984),
    Number(process.env.GATEWAY_B_PORT || 38985),
  ]
  const origins = ports.map((port) => 'http://127.0.0.1:' + port)
  const gateways = []
  let browser, proxy
  try {
    for (const [i, port] of ports.entries()) {
      const child = spawn(process.env.GATEWAY_BIN, ['-port', String(port)], {
        stdio: 'ignore',
      })
      gateways.push(child)
      await once(child, 'spawn')
      let ready = false
      for (let n = 0; n < 100; n++) {
        if (child.exitCode !== null) throw Error('gateway exited')
        ready = await fetch(origins[i] + '/readyz')
          .then((r) => r.ok)
          .catch(() => false)
        if (ready) break
        await sleep(100)
      }
      assert(ready)
    }
    let target = ports[0]
    proxy = net
      .createServer((socket) => {
        const upstream = net.connect(target, '127.0.0.1')
        socket.on('error', () => upstream.destroy())
        upstream.on('error', () => socket.destroy())
        socket.on('close', () => upstream.destroy())
        upstream.on('close', () => socket.destroy())
        socket.pipe(upstream).pipe(socket)
      })
      .listen(0, '127.0.0.1')
    await once(proxy, 'listening')
    browser = await chromium.launch({
      executablePath: process.env.CHROMIUM_PATH,
    })
    const viewer = await browser.newPage()
    const sender = await browser.newPage()
    const query = '/?room=roll&session=' + Date.now()
    await viewer.goto('http://127.0.0.1:' + proxy.address().port + query)
    await sender.goto(origins[1] + query)
    for (const page of [viewer, sender])
      await page.waitForFunction(
        () => !document.querySelector('#send').disabled,
      )
    async function send(text) {
      await sender.locator('#message').fill(text)
      await sender.locator('#send').click()
      await sender.waitForFunction(
        (t) => document.querySelector('#messages')?.textContent.includes(t),
        text,
      )
    }
    await send('before')
    await viewer.getByText('before', { exact: true }).first().waitFor()
    // Hold the candidate's HTTP snapshot after it was read on the server.
    let release, captured
    const gate = new Promise((resolve) => (release = resolve))
    const snapshot = new Promise((resolve) => (captured = resolve))
    let failOnce = true
    await viewer.route('**/recent?**', async (route) => {
      if (failOnce) {
        failOnce = false
        await route.fulfill({ status: 503, body: 'temporarily busy' })
        return
      }
      const response = await route.fetch()
      captured()
      await gate
      await route.fulfill({ response })
    })
    target = ports[1]
    const exit = once(gateways[0], 'exit')
    gateways[0].kill('SIGTERM')
    await Promise.race([
      snapshot,
      sleep(8000).then(() => {
        throw Error('handoff timeout')
      }),
    ])
    assert.equal((await fetch(origins[0] + '/readyz')).status, 503)
    const cookies = await viewer.context().cookies()
    const admission = await fetch(
      origins[0] +
        '/api/rooms/roll/' +
        new URLSearchParams(query.slice(2)).get('session') +
        '/live?migrate=1',
      {
        headers: {
          Cookie: cookies.map((c) => c.name + '=' + c.value).join('; '),
        },
      },
    )
    assert.equal(admission.status, 503)
    await send('during')
    await viewer
      .getByText('during', { exact: true })
      .first()
      .waitFor({ timeout: 3000 })
    release()
    await exit
    await send('after')
    await viewer.getByText('after', { exact: true }).first().waitFor()
    assert.deepEqual(
      await viewer
        .locator('#messages li')
        .evaluateAll((rows) => rows.map((r) => r.lastChild.textContent)),
      ['after', 'during', 'before'],
    )
    console.log(
      'Chromium: old socket delivered during held snapshot; complete ordered list after handoff',
    )
  } finally {
    await browser?.close()
    proxy?.close()
    for (const child of gateways) child.kill('SIGKILL')
  }
}
main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
