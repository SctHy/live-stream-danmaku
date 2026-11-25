// Run against a dedicated gateway with a >=125s MP4 under REPLAY_STORAGE_DIR.
// REPLAY_FIXTURE names that file relative to the gateway's storage root.
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
const { chromium } = require('playwright')

async function main() {
  const origin = process.env.GATEWAY_ORIGIN || 'http://127.0.0.1:38502'
  const file = process.env.REPLAY_FIXTURE
  assert(file, 'set REPLAY_FIXTURE to an unused test MP4')
  const room = 'ui' + randomUUID().slice(0, 8)
  const base = '/api/rooms/' + room
  const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH })
  try {
    const page = await browser.newPage({ baseURL: origin })
    async function post(path, data, headers = {}) {
      const res = await page.request.post(path, { data, headers })
      assert(res.ok(), path + ' returned ' + res.status())
      return res
    }
    await post('/api/auth/register', { username: room, password: randomUUID() })
    const ownerRoom = await (await post('/api/rooms', { slug: room, title: room })).json()
    await page.goto('/?room=' + room + '&session=' + ownerRoom.active_session)
    await page.locator('#staffOpen').click()
    await page.waitForFunction(() => document.querySelector('#obsKey').value.length > 0)
    const key = page.locator('#obsKey')
    const original = await key.inputValue()
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write'], {origin})
    await page.locator('#copyServer').click()
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), await page.locator('#obsServer').inputValue())
    await page.locator('#copyKey').click()
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()) === original, true)
    assert.equal(await key.getAttribute('type'), 'password')
    await page.locator('#showKey').click()
    assert.equal(await key.getAttribute('type'), 'text')
    page.once('dialog', dialog => dialog.accept())
    await page.locator('#rotateKey').click()
    await page.waitForFunction(old => document.querySelector('#obsKey').value !== old, original)
    assert.equal(await key.getAttribute('type'), 'password')
    await page.locator('#staff [data-close]').click()
    const headers = process.env.STREAM_WEBHOOK_SECRET ? {'X-Webhook-Secret': process.env.STREAM_WEBHOOK_SECRET} : {}
    const live = await (await post('/internal/stream/ready', {room_slug: room, source_id: room}, headers)).json()
    await page.waitForFunction(id => document.querySelector('#session').value === id, live.session_id)
    if (process.env.CHECK_OBS_HLS) {
      await page.waitForFunction(() => document.querySelector('#source-status').textContent === 'OBS 直播中 · HLS' && document.querySelector('#video').currentTime > 0)
    }
    const path = base + '/replays/' + live.replay_id
    await post('/internal/stream/ended', { room_slug: room, replay_id: live.replay_id, video_file: file, duration_ms: 128000 }, headers)
    await page.waitForFunction(() => document.querySelector('#source-status').textContent === '本地弹幕')
    async function add(content, video_offset_ms) {
      return (await post(path + '/danmaku', { content, video_offset_ms })).json()
    }
    const range = await page.request.get('/api/replays/' + live.replay_id + '/video', { headers: {Range: 'bytes=0-79'} })
    assert.equal(range.status(), 206)
    assert.equal((await range.body()).length, 80)
    const removed = await add('withdrawn', 1000)
    await post(base + '/' + live.session_id + '/moderate', {action: 'withdraw', event_id: removed.event_id})
    await add('opening', 0)
    for (let i = 0; i < 501; i += 10) {
      await Promise.all(Array.from({length: Math.min(10, 501-i)}, (_, n) => add('page-' + (i+n), 30000)))
    }
    await add('target', 120000)
    async function open() {
      await page.locator('#replaysOpen').click()
      await page.locator('#recordings').getByRole('button', {name: room, exact: true}).click()
    }
    await open()
    await page.waitForFunction(() => document.querySelectorAll('#timeline li').length === 502)
    assert(!(await page.locator('#timeline').innerText()).includes('withdrawn'))
    const video = page.locator('#video')
    await page.waitForFunction(() => document.querySelector('#video').currentTime > 0.15)
    await video.evaluate(v => v.pause())
    const sprite = page.locator('#replayInk span').filter({hasText: 'opening'})
    const position = await sprite.evaluate(s => s.style.transform)
    await page.waitForTimeout(250)
    assert.equal(await sprite.evaluate(s => s.style.transform), position)
    await page.locator('#overlayEnabled').uncheck()
    assert.equal(await sprite.isVisible(), false)
    await page.locator('#overlayEnabled').check()
    await page.locator('#rate').selectOption('2')
    assert.equal(await video.evaluate(v => v.playbackRate), 2)
    await video.evaluate(v => v.play())
    await page.waitForFunction(pos => document.querySelector('#replayInk span')?.style.transform !== pos, position)
    await video.evaluate(v => { v.pause(); v.currentTime = 121 })
    await page.locator('#replayInk span').filter({hasText:'target'}).waitFor()
    assert.equal(await sprite.count(), 0)
    await page.locator('#timeline button').filter({hasText:'target'}).click()
    await page.waitForFunction(() => document.querySelector('#video').currentTime === 120)
    await page.locator('#message').fill('persisted')
    await page.locator('#send').click()
    await page.locator('#replayInk span').filter({hasText:'persisted'}).waitFor()
    await page.reload()
    await open()
    await video.evaluate(async v => {
      if (v.readyState < 1) await new Promise(resolve => v.addEventListener('loadedmetadata', resolve, {once:true}))
      v.currentTime = 120.5
    })
    await page.locator('#timeline button').filter({hasText:'persisted'}).waitFor()
    await page.locator('#backLive').click()
    await page.waitForFunction(() => !document.querySelector('#messages').hidden && document.querySelector('#connection').textContent === '已连接')
    assert.equal(await page.locator('#replayInk span').count(), 0)
    await page.locator('#message').fill('back in live chat')
    await page.locator('#send').click()
    await page.locator('#messages li').getByText('back in live chat', {exact:true}).waitFor()
    console.log('Chromium replay, OBS credentials, session switch and live return passed')
  } finally {
    await browser.close()
  }
}
main().catch(err => { console.error(err); process.exitCode = 1 })
