// Run against a dedicated gateway with Redis, PostgreSQL and Kafka.
const {chromium} = require('playwright')
const assert = require('node:assert/strict')
const {randomUUID} = require('node:crypto')

async function main() {
  const browser = await chromium.launch({executablePath:process.env.CHROMIUM_PATH})
  const origin = process.env.GATEWAY_ORIGIN || 'http://127.0.0.1:38502'
  const password = randomUUID()
  try {
    const users = []
    for (const theme of ['rose','ocean']) {
      const page = await browser.newPage({baseURL:origin,viewport:{width:1280,height:800}})
      const name = 'ux'+randomUUID().slice(0,8)
      await page.goto('/')
      await page.locator('#accountOpen').click()
      await page.evaluate(() => { window.originalVideo = document.querySelector('#video') })
      async function auth(label) {
        await page.locator('#auth [name=username]').fill(name)
        await page.locator('#auth [name=password]').fill(password)
        await page.locator('#auth').getByRole('button',{name:label,exact:true}).click()
        await page.locator('#logout').waitFor()
        assert(await page.evaluate(() => window.originalVideo === document.querySelector('#video')))
      }
      await auth('注册')
      await page.locator('#logout').click()
      await page.locator('#auth').waitFor()
      await auth('登录')
      await page.locator('#claim').click()
      await page.locator('#claim:disabled').waitFor()
      await page.locator('#recharge button').click()
      const pending = page.locator('#recharges li').first()
      await pending.getByText(/待支付/).waitFor()
      assert.equal(await pending.locator('span').evaluate(e=>getComputedStyle(e).whiteSpace),'nowrap')
      assert.equal(await pending.evaluate(e=>getComputedStyle(e).listStyleType),'none')
      await page.locator('#account [data-close]').click()
      await page.locator('#plazaOpen').click()
      assert(await page.locator('#createToggle').evaluate(b => b.closest('.dialoghead') !== null))
      await page.locator('#createToggle').click()
      await page.locator('#create [name=slug]').fill(name)
      await page.locator('#create [name=title]').fill(name)
      await page.locator('#create [name=cover_theme]').selectOption(theme)
      await page.locator('#create button').click()
      await page.waitForURL('**/?room='+name+'&session=*')
      await page.locator('#staffOpen').waitFor()
      assert(await page.locator('#moderation-panel').isHidden())
      await page.waitForFunction(() => document.querySelector('#connection').textContent === '已连接')
      const bottom = await page.locator('#compose').evaluate(e => e.getBoundingClientRect().bottom)
      assert(bottom <= 800,'composer bottom '+bottom)
      await page.locator('#inkColor').selectOption('#34d399')
      const mode = theme === 'rose' ? 'top' : 'bottom'
      await page.locator('#inkMode').selectOption(mode)
      await page.locator('#message').fill(name)
      await page.locator('#send').click()
      const row = page.locator('#messages li').filter({hasText:name}).first()
      await row.locator('button').first().waitFor()
      assert.equal(await row.locator('button').count(),1)
      assert.equal(await row.evaluate(li => li.lastChild.textContent),name)
      await page.waitForFunction(mode => {
        const canvas = document.querySelector('#overlay')
        const y = mode === 'top' ? 0 : Math.floor(canvas.height * .75)
        const {data} = canvas.getContext('2d').getImageData(0,y,canvas.width,Math.floor(canvas.height/4))
        for (let i = 0; i < data.length; i += 4) {
          if (data[i+1] > 170 && data[i] < 90 && data[i+2] > 110 && data[i+3] > 150) return true
        }
        return false
      }, mode)
      const session = await page.locator('#session').inputValue()
      const recent = await (await page.request.get('/api/rooms/'+name+'/'+session+'/recent')).json()
      assert(recent.events.some(e => e.data.content===name && e.data.mode===mode && e.data.color==='#34d399'))
      await page.locator('#staffOpen').click()
      const coverSelect = page.locator('#editRoom [name=cover_theme]')
      await page.waitForFunction(theme => document.querySelector('#editRoom [name=cover_theme]').value === theme,theme)
      assert.equal(await coverSelect.locator('option').count(),6)
      assert.equal(await page.locator('#muteForm input').getAttribute('role'),'switch')
      await coverSelect.selectOption('amber')
      await page.locator('#editRoom button').click()
      await page.waitForFunction(() => !document.querySelector('#staff').open)
      assert.equal((await (await page.request.get('/api/rooms/'+name)).json()).cover_theme,'amber')
      users.push({page,name,url:page.url()})
    }
    for (const [i,{page}] of users.entries()) {
      const peer = users[1-i]
      await page.goto(peer.url)
      await page.locator('#follow').click()
      await page.locator('#follow').filter({hasText:'已关注 · 1 粉丝'}).waitFor()
      await page.locator('#plazaOpen').click()
      await page.locator('#rooms .cover').first().waitFor()
      assert(await page.locator('#rooms').evaluate(e=>getComputedStyle(e).gridTemplateColumns.split(' ').length>=2))
      await page.locator('#sort').selectOption('following')
      const cards = page.locator('#rooms button')
      await page.waitForFunction(() => document.querySelectorAll('#rooms button').length===1)
      assert((await cards.innerText()).includes(peer.name))
      assert.equal(await cards.locator('.cover').getAttribute('data-theme'),'amber')
      await page.locator('#plaza [data-close]').click()
      await page.locator('#follow').click()
      await page.locator('#follow').filter({hasText:/^关注 · 0/}).waitFor()
      await page.getByText('送礼物 / 醒目留言',{exact:true}).click()
      await page.locator('#giftForm button').click()
      await page.locator('#feedback').filter({hasText:'礼物已送达'}).waitFor()
    }
    for (const {page} of users) {
      await page.locator('#accountOpen').click()
      await page.locator('#ledger li').filter({hasText:/礼物收益 · \+[1-9]/}).waitFor()
      assert.match(await page.locator('#earnings').innerText(),/主播收益：[1-9]/)
    }
    console.log('Room layout, account changes, styled live chat, plaza, follows and bilateral earnings passed')
  } finally {
    await browser.close()
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
