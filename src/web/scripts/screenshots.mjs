// Takes the README screenshots from a server filled by src/demo/seed.sh
// (make screenshots runs both). Usage: bun scripts/screenshots.mjs SERVER OUT_DIR
// Chrome: $CHROME, else /usr/bin/google-chrome, else Playwright's own Chromium.
import { existsSync } from 'node:fs'
import { chromium } from 'playwright-core'

const [server, out] = process.argv.slice(2)
if (!server || !out) {
  console.error('usage: bun scripts/screenshots.mjs SERVER OUT_DIR')
  process.exit(2)
}

const api = async (path) => {
  const res = await fetch(server + path)
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`)
  return res.json()
}
// Keys are looked up by title, so the shots survive changes to the seed.
const journeys = await api('/api/journeys')
const journey = (title) => {
  const j = journeys.find((j) => j.title === title)
  if (!j) throw new Error(`no demo journey "${title}": seed the server with src/demo/seed.sh`)
  return j.key
}
const quest = async (title) => {
  const { quests } = await api('/api/search?q=' + encodeURIComponent(title))
  const q = quests.find((q) => q.title === title)
  if (!q) throw new Error(`no demo quest "${title}"`)
  return q.key
}

const omelette = journey('Make an omelette')
const brunch = journey('Host brunch for four on Sunday')
const plugins = journey('mikado can hook in any task tool, not only GitHub')
const found = await quest('Buy a box of eggs from the corner shop')
const omeletteCrown = await quest('Serve the omelette on a warm plate while it is still soft in the middle')
const issue = await quest('Make GitHub one pluggable quest source, so any tool can be hooked in').catch(() =>
  quest('Write down the JSON protocol a source speaks'),
)

const shots = [
  { file: 'board.png', path: '/', theme: 'parchment', tab: 'journey' },
  { file: 'chart.png', path: `/journey/${omelette}?quest=${found}`, theme: 'parchment', tab: 'journey' },
  { file: 'glossary.png', path: `/journey/${omelette}?quest=${found}`, theme: 'parchment', tab: 'glossary' },
  { file: 'midnight.png', path: `/journey/${plugins}?quest=${issue}`, theme: 'midnight', tab: 'journey' },
  { file: 'medieval.png', path: `/journey/${brunch}?quest=${omeletteCrown}`, theme: 'wartable', tab: 'journey' },
]

const chrome = process.env.CHROME ?? (existsSync('/usr/bin/google-chrome') ? '/usr/bin/google-chrome' : undefined)
const browser = await chromium.launch({ executablePath: chrome, headless: true })
let failed = false
for (const s of shots) {
  const ctx = await browser.newContext({ viewport: { width: 1600, height: 900 } })
  await ctx.addInitScript(
    ([theme, tab]) => {
      localStorage.setItem('mikado.mock.theme', theme)
      localStorage.setItem('mikado.panel.tab', tab)
    },
    [s.theme, s.tab],
  )
  const page = await ctx.newPage()
  page.on('pageerror', (e) => {
    failed = true
    console.error(`${s.file}: ${e.message}`)
  })
  await page.goto(server + s.path, { waitUntil: 'networkidle' })
  await page.waitForTimeout(1500) // the chart lays itself out and settles
  await page.screenshot({ path: `${out}/${s.file}` })
  console.log(`${out}/${s.file}`)
  await ctx.close()
}
await browser.close()
process.exit(failed ? 1 : 0)
