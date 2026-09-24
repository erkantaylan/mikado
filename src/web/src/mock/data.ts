// A made-up goal used to agree on the workflow before any real data exists.
// Repos, people and numbers are fictional.

// An item is either a GitHub issue or a card that lives only in mikado:
// a `wait` is something we are waiting on someone for, a `task` is a real step
// that is too small or too big to be worth an issue.
export type Kind = 'issue' | 'wait' | 'task'

export type Item = {
  id: string // issues: owner/repo#n, cards: card:<slug>
  kind: Kind
  title: string
  done: boolean
  assignee?: string // GitHub login (issues) or whoever owns the card
  waitingOn?: string // waits: who we are waiting for — may be outside the team
  since?: string // waits: when the wait started
  final?: boolean // the item whose completion means the goal is reached
  foundWhile?: string // id of the item this one was discovered from
  sideOf?: string // a side quest: optional polish on this card; never blocks it
  npc?: boolean // a purely visual badge — a joke, it changes nothing
  cancelled?: boolean // won't be done: stays on the map, blocks nothing, counts for nothing
  cancelReason?: string
  working?: boolean // someone is on it right now
  reason?: string // why it was added along the way
}

// `from` needs `to` before it can be done.
export type Need = { from: string; to: string }

export type LogEntry = {
  at: string
  text: string
  id?: string
  kind: 'create' | 'add' | 'remove' | 'assign' | 'done' | 'cancel'
}

const winterGoal = {
  title: 'The winter update ships to every player',
  doneWhen: 'studio/game#140',
}

const winterItems: Item[] = [
  { id: 'studio/game#140', kind: 'issue', title: 'Ship the winter update to every platform', done: false, assignee: 'ada', final: true },
  { id: 'studio/game#138', kind: 'issue', title: 'Winter map: snow tiles, then the frozen-lake level', done: false, assignee: 'bo' },
  { id: 'studio/saves#88', kind: 'issue', title: 'Save migration for the winter items — old saves upgraded on load, a backup kept beside each one, one migration per save slot', done: false, assignee: 'cyd' },
  { id: 'studio/net#93', kind: 'issue', title: 'Cloud-save sync client with retry', done: true, assignee: 'cyd' },
  { id: 'studio/build#22', kind: 'issue', title: 'Upload the release builds to every store', done: false, assignee: 'ada' },
  {
    id: 'studio/saves#95',
    kind: 'issue',
    title: 'Split-screen co-op',
    done: false,
    assignee: 'cyd',
    cancelled: true,
    cancelReason: 'Closed on GitHub as not planned — out of scope for this quest.',
  },
  {
    id: 'studio/saves#91',
    kind: 'issue',
    title: 'Old save files crash the loader',
    done: false,
    assignee: 'cyd',
    foundWhile: 'studio/saves#88',
    reason: 'Saves from the launch build crash on load; noticed while writing the migration.',
  },
  {
    id: 'studio/saves#45',
    kind: 'issue',
    title: 'The save format has no version field',
    done: false,
    foundWhile: 'studio/saves#91',
    reason: 'The loader cannot tell an old save from a new one, so it cannot know what to upgrade.',
  },
  {
    id: 'card:key-art',
    kind: 'wait',
    title: 'Final key art from the freelance artist',
    done: false,
    assignee: 'ada',
    waitingOn: 'freelance artist',
    npc: true,
    since: 'Sep 14',
    foundWhile: 'studio/build#22',
    reason: 'Placeholder art only; the stores reject a build page without final art.',
  },
  {
    id: 'card:feature-slot',
    kind: 'task',
    title: 'Book the store-page feature slot with the platform',
    done: false,
    assignee: 'ada',
    npc: true,
    working: true,
  },
]

// Side quests: optional, they hang off a main card and never lock anything.
const winterSideQuests: Item[] = [
  {
    id: 'studio/game#141',
    kind: 'issue',
    title: 'Polish: snowfall particles on the title screen',
    done: false,
    assignee: 'bo',
    sideOf: 'studio/game#138',
  },
  {
    id: 'studio/saves#97',
    kind: 'issue',
    title: 'Polish: a friendlier message when a save is upgraded',
    done: false,
    sideOf: 'studio/saves#88',
  },
]

const winterNeeds: Need[] = [
  { from: 'studio/game#140', to: 'studio/game#138' },
  { from: 'studio/game#140', to: 'studio/saves#88' },
  { from: 'studio/game#140', to: 'studio/build#22' },
  { from: 'studio/game#140', to: 'card:feature-slot' },
  { from: 'studio/game#138', to: 'studio/saves#88' },
  { from: 'studio/saves#88', to: 'studio/net#93' },
  { from: 'studio/saves#88', to: 'studio/saves#91' },
  { from: 'studio/saves#88', to: 'studio/saves#95' },
  { from: 'studio/saves#91', to: 'studio/saves#45' },
  { from: 'studio/build#22', to: 'card:key-art' },
]

const winterLog: LogEntry[] = [
  { at: 'Sep 10', kind: 'create', text: 'Quest created — fulfilled when studio/game#140 closes' },
  { at: 'Sep 10', kind: 'add', id: 'studio/game#140', text: 'added as the crowning deed' },
  { at: 'Sep 10', kind: 'add', id: 'studio/game#138', text: 'added — opens #140' },
  { at: 'Sep 10', kind: 'add', id: 'studio/saves#88', text: 'added — opens #140 and #138' },
  { at: 'Sep 10', kind: 'add', id: 'studio/net#93', text: 'added — opens saves#88' },
  { at: 'Sep 10', kind: 'add', id: 'studio/build#22', text: 'added — opens #140' },
  { at: 'Sep 10', kind: 'add', id: 'card:feature-slot', text: 'errand added — opens #140' },
  { at: 'Sep 11', kind: 'add', id: 'studio/saves#95', text: 'added — split-screen co-op needs its own save slots' },
  { at: 'Sep 12', kind: 'assign', id: 'studio/saves#88', text: 'assigned to @cyd' },
  { at: 'Sep 14', kind: 'add', id: 'card:key-art', text: 'petition to the freelance artist — unearthed while on build#22' },
  { at: 'Sep 15', kind: 'done', id: 'studio/net#93', text: 'fulfilled (closed on GitHub)' },
  { at: 'Sep 16', kind: 'add', id: 'studio/saves#91', text: 'unearthed while on #88: old saves crash the loader' },
  { at: 'Sep 17', kind: 'add', id: 'studio/game#141', text: 'side quest on game#138 — nice to have, doesn’t block' },
  { at: 'Sep 18', kind: 'add', id: 'studio/saves#45', text: 'unearthed while on #91: the save format has no version field' },
  { at: 'Sep 19', kind: 'add', id: 'studio/saves#97', text: 'side quest on saves#88' },
  { at: 'Sep 19', kind: 'cancel', id: 'studio/saves#95', text: 'abandoned — closed on GitHub as not planned' },
]

// ---- a second map: the quest line of building mikado's first real slice ----
// mikado has no GitHub repo yet, so every step is an errand. Updated as the work lands.

const buildGoal = {
  title: 'mikado runs on real data: quests from the CLI, drawn on the map',
  doneWhen: 'card:commit-slice',
}

const buildItems: Item[] = [
  { id: 'card:stack', kind: 'task', title: 'Pick the stack: Go binary, React Flow + ELK, Tailwind', done: true, assignee: 'claude' },
  { id: 'card:mock', kind: 'task', title: 'Agree the workflow on a mock: Quest Board and chart', done: true, assignee: 'erkan' },
  { id: 'card:commit-mock', kind: 'task', title: 'Commit the mock as the agreed reference', done: true, assignee: 'claude' },
  { id: 'card:storage', kind: 'task', title: 'Decide storage: SQLite, owned by mikado serve, CLI over HTTP', done: true, assignee: 'erkan' },
  {
    id: 'card:port',
    kind: 'task',
    title: 'Move the default port to 47291',
    done: true,
    assignee: 'claude',
    foundWhile: 'card:storage',
    reason: 'Every standard port on this machine is already taken.',
  },
  { id: 'card:store', kind: 'task', title: 'SQLite store: schema, migrations and the chronicle', done: true, assignee: 'claude' },
  { id: 'card:github', kind: 'task', title: 'GitHub reads through gh: batched, cached, validated on add', done: true, assignee: 'claude' },
  { id: 'card:api', kind: 'task', title: 'JSON API: quests, deeds, requirements, assignees', done: true, assignee: 'claude' },
  { id: 'card:cli', kind: 'task', title: 'CLI: quest new/show, add, errand, petition, require, fulfil, assign', done: true, assignee: 'claude' },
  { id: 'card:tests', kind: 'task', title: 'Store unit tests and an end-to-end smoke run', done: true, assignee: 'claude' },
  {
    id: 'card:props',
    kind: 'task',
    title: 'Make the chart page take its quest as data instead of reading the mock',
    done: false,
    assignee: 'claude',
    working: true,
    foundWhile: 'card:this-map',
    reason: 'The mock reads module-level data; real quests arrive from the API.',
  },
  { id: 'card:this-map', kind: 'task', title: 'Put this build on the mock as its own quest line', done: true, assignee: 'claude' },
  { id: 'card:real-map', kind: 'task', title: 'Quest Board and chart read the real API', done: false, assignee: 'claude', working: true },
  {
    id: 'card:start-stop',
    kind: 'task',
    title: '`mikado take-up` / `set-down` so "underway" is real, not mock-only',
    done: true,
    assignee: 'claude',
    foundWhile: 'card:this-map',
    reason: 'The underway indicator needs a way to be set; GitHub has no reliable signal for it.',
  },
  {
    id: 'card:cancel',
    kind: 'task',
    title: 'Abandoned deeds: won’t-do stays on the chart, like GitHub’s “not planned”',
    done: true,
    assignee: 'claude',
    foundWhile: 'card:side-orphans',
    reason: 'Striking is not the only way out: some deeds are simply not going to happen.',
  },
  {
    id: 'card:side-orphans',
    kind: 'wait',
    title: 'Decide what happens to side quests when their deed is struck',
    done: true,
    assignee: 'claude',
    waitingOn: 'erkan',
    since: 'Sep 24',
    foundWhile: 'card:store',
    reason: 'Striking a deed leaves its side quests pointing at nothing.',
  },
  {
    id: 'card:review',
    kind: 'wait',
    title: 'Your review of slice 1',
    done: false,
    assignee: 'claude',
    waitingOn: 'erkan',
    since: 'Sep 24',
  },
  { id: 'card:commit-slice', kind: 'task', title: 'Commit slice 1', done: false, assignee: 'claude', final: true },
]

const buildSideQuests: Item[] = [
  { id: 'card:panel-memory', kind: 'task', title: 'Polish: the side panel remembers whether it was open', done: false, sideOf: 'card:real-map' },
  { id: 'card:ascii-map', kind: 'task', title: 'Polish: `mikado quest show` draws a small text chart', done: false, sideOf: 'card:cli' },
  { id: 'card:bulb', kind: 'task', title: 'Polish: the selected deed glows from its edges', done: true, assignee: 'claude', sideOf: 'card:real-map' },
]

const buildNeeds: Need[] = [
  { from: 'card:mock', to: 'card:stack' },
  { from: 'card:commit-mock', to: 'card:mock' },
  { from: 'card:storage', to: 'card:commit-mock' },
  { from: 'card:store', to: 'card:storage' },
  { from: 'card:github', to: 'card:storage' },
  { from: 'card:api', to: 'card:store' },
  { from: 'card:api', to: 'card:github' },
  { from: 'card:api', to: 'card:port' },
  { from: 'card:cli', to: 'card:api' },
  { from: 'card:tests', to: 'card:cli' },
  { from: 'card:this-map', to: 'card:commit-mock' },
  { from: 'card:real-map', to: 'card:api' },
  { from: 'card:real-map', to: 'card:props' },
  { from: 'card:review', to: 'card:tests' },
  { from: 'card:review', to: 'card:real-map' },
  { from: 'card:real-map', to: 'card:start-stop' },
  { from: 'card:start-stop', to: 'card:api' },
  { from: 'card:review', to: 'card:side-orphans' },
  { from: 'card:real-map', to: 'card:cancel' },
  { from: 'card:cancel', to: 'card:side-orphans' },
  { from: 'card:commit-slice', to: 'card:review' },
]

const buildLog: LogEntry[] = [
  { at: 'Sep 24', kind: 'create', text: 'Quest created — fulfilled when slice 1 is committed' },
  { at: 'Sep 24', kind: 'done', id: 'card:stack', text: 'stack picked' },
  { at: 'Sep 24', kind: 'done', id: 'card:mock', text: 'workflow agreed on the mock' },
  { at: 'Sep 24', kind: 'done', id: 'card:commit-mock', text: 'mock committed (02020d2)' },
  { at: 'Sep 24', kind: 'done', id: 'card:storage', text: 'SQLite, local first; auth and users later' },
  { at: 'Sep 24', kind: 'add', id: 'card:port', text: 'unearthed while deciding storage: standard ports all taken' },
  { at: 'Sep 24', kind: 'done', id: 'card:port', text: 'port 47291 (bd8a021)' },
  { at: 'Sep 24', kind: 'assign', id: 'card:store', text: 'store, GitHub, API, CLI and tests taken up by a background agent' },
  { at: 'Sep 24', kind: 'add', id: 'card:props', text: 'unearthed while putting this build on the chart' },
  { at: 'Sep 24', kind: 'done', id: 'card:this-map', text: 'this quest is on the Quest Board' },
  { at: 'Sep 24', kind: 'add', id: 'card:start-stop', text: 'unearthed while adding the underway indicator' },
  { at: 'Sep 24', kind: 'add', id: 'card:bulb', text: 'side quest: try a light-bulb glow for the selected deed' },
  { at: 'Sep 24', kind: 'done', id: 'card:store', text: 'SQLite store with migrations and the chronicle' },
  { at: 'Sep 24', kind: 'done', id: 'card:github', text: 'batched GraphQL reads, 60 s cache, refs validated' },
  { at: 'Sep 24', kind: 'done', id: 'card:api', text: 'JSON API, loopback-only, host and content-type guarded' },
  { at: 'Sep 24', kind: 'done', id: 'card:cli', text: 'CLI fulfilled, plus `set` and `assignees`' },
  { at: 'Sep 24', kind: 'done', id: 'card:tests', text: 'store and API tests pass; smoke run against two real, read-only issues' },
  { at: 'Sep 24', kind: 'add', id: 'card:side-orphans', text: 'petition to erkan — unearthed while reviewing the store' },
  { at: 'Sep 24', kind: 'done', id: 'card:side-orphans', text: 'erkan: strike and abandon both exist, like GitHub' },
  { at: 'Sep 24', kind: 'add', id: 'card:cancel', text: 'unearthed while deciding: abandoned means won’t do' },
  { at: 'Sep 24', kind: 'done', id: 'card:bulb', text: 'edge glow, no flicker' },
  { at: 'Sep 24', kind: 'done', id: 'card:start-stop', text: '`mikado take-up` / `set-down`, cleared on fulfil or abandon' },
  { at: 'Sep 24', kind: 'done', id: 'card:cancel', text: 'abandon and strike both cascade to side quests; GitHub “not planned” reads as abandoned' },
  { at: 'Sep 24', kind: 'assign', id: 'card:real-map', text: 'real pages taken up by a background agent' },
]

export type QuestData = { goal: typeof winterGoal; items: Item[]; sideQuests: Item[]; needs: Need[]; log: LogEntry[] }

export const maps: Record<string, QuestData> = {
  'winter-update': { goal: winterGoal, items: winterItems, sideQuests: winterSideQuests, needs: winterNeeds, log: winterLog },
  'mikado-slice-1': { goal: buildGoal, items: buildItems, sideQuests: buildSideQuests, needs: buildNeeds, log: buildLog },
}

export const hasMap = (slug: string) => slug in maps

const summarise = (slug: string, heroes: string[], repos: string[]): QuestSummary => {
  const q = maps[slug]
  const byId = new Map([...q.items, ...q.sideQuests].map((i) => [i.id, i]))
  const open = (i: Item) => q.needs.some((n) => n.from === i.id && !byId.get(n.to)?.done && !byId.get(n.to)?.cancelled)
  return {
    slug,
    title: q.goal.title,
    main: { done: q.items.filter((i) => i.done && !i.cancelled).length, total: q.items.filter((i) => !i.cancelled).length },
    achievements: { done: q.sideQuests.filter((i) => i.done).length, total: q.sideQuests.length },
    available: q.items.filter((i) => !i.done && i.kind !== 'wait' && !open(i)).length,
    awaiting: q.items.filter((i) => !i.done && i.kind === 'wait' && !open(i)).length,
    heroes,
    repos,
    lastActivity: 'Sep 24',
  }
}

// The quest board: every quest at a glance. Only the quests in `maps` open a map.
export type QuestSummary = {
  slug: string
  title: string
  main: { done: number; total: number }
  achievements: { done: number; total: number } // side quests
  available: number
  awaiting: number
  heroes: string[]
  repos: string[]
  lastActivity: string
}

export const quests: QuestSummary[] = [
  summarise('mikado-slice-1', ['claude', 'erkan'], ['mikado']),
  {
    slug: 'winter-update',
    title: 'The winter update ships to every player',
    main: { done: 1, total: 9 },
    achievements: { done: 0, total: 2 },
    available: 2,
    awaiting: 1,
    heroes: ['ada', 'bo', 'cyd'],
    repos: ['game', 'saves', 'net', 'build'],
    lastActivity: 'Sep 19',
  },
  {
    slug: 'controller-support',
    title: 'Controller support on every platform',
    main: { done: 7, total: 12 },
    achievements: { done: 1, total: 4 },
    available: 3,
    awaiting: 0,
    heroes: ['bo', 'dex'],
    repos: ['input', 'game'],
    lastActivity: 'Sep 23',
  },
  {
    slug: 'level-editor',
    title: 'The level editor goes public',
    main: { done: 5, total: 5 },
    achievements: { done: 3, total: 5 },
    available: 2,
    awaiting: 0,
    heroes: ['cyd'],
    repos: ['editor', 'build'],
    lastActivity: 'Sep 21',
  },
  {
    slug: 'crash-reports',
    title: 'Crash reports reach us within a minute',
    main: { done: 2, total: 6 },
    achievements: { done: 0, total: 1 },
    available: 0,
    awaiting: 2,
    heroes: ['ada'],
    repos: ['build', 'net', 'saves'],
    lastActivity: 'Sep 12',
  },
  {
    slug: 'achievement-sync',
    title: 'Achievements sync across devices',
    main: { done: 4, total: 4 },
    achievements: { done: 2, total: 2 },
    available: 0,
    awaiting: 0,
    heroes: ['cyd', 'dex'],
    repos: ['net'],
    lastActivity: 'Sep 02',
  },
]
