// A journey as the war table and the atlas draw it, whether it comes from the mock data or
// from the API. Pages build a JourneyModel from JourneyData once per version of
// the data; everything that used to read module-level mock state reads the model.

// An item is either a GitHub issue or a card that lives only in mikado:
// a `wait` is something we are waiting on someone for, a `task` is a real step
// that is too small or too big to be worth an issue.
export type Kind = 'issue' | 'wait' | 'task'

// Completed: done. Available: everything before it is done. Locked: something
// before it is still open. Awaiting: an available card that waits on someone else.
// Cancelled: won't be done — it stays on the map but blocks nothing and counts for nothing.
export type Status = 'done' | 'available' | 'locked' | 'awaiting' | 'cancelled'

export type Item = {
  id: string // the node id: mock issues use owner/repo#n, mock cards card:<name>, API cards c<id>
  key?: string // the key people use for the quest (Q142)
  kind: Kind
  title: string
  done: boolean
  ref?: string // issues: owner/repo#n, when the id is not the ref itself
  url?: string // issues: the GitHub page
  assignee?: string // GitHub login (issues) or whoever owns the card
  waitingOn?: string // waits: who we are waiting for — may be outside the team
  since?: string // waits: when the wait started, as shown ("Sep 14")
  sinceDays?: number // waits: whole days since then
  final?: boolean // the item whose completion means the goal is reached
  foundWhile?: string // id of the item this one was discovered from
  sideOf?: string // a side quest: optional polish on this card; never blocks it
  npc?: boolean // only looks: an NPC quest is drawn red; its status is unchanged
  cancelled?: boolean // won't be done: stays on the map, blocks nothing, counts for nothing
  cancelReason?: string
  working?: boolean // someone is on it right now
  workingBy?: string
  reason?: string // why it was added along the way
  alsoIn?: JourneyRef[] // other journeys the same card belongs to
  status?: Status // computed by the server; computed here when absent (mock data)
  openBefore?: number // likewise: how many cards it needs are still open
  crowns?: JourneyCard // it crowns another journey: drawn as that journey's card, standing for all of it
}

export type JourneyRef = { key: string; title: string }

/** Another journey, as its card on this chart shows it: its state, its progress and what is left in it. */
export type JourneyCard = {
  key: string // J7
  title: string
  state: JourneyState
  archived: boolean
  done: number
  total: number
  underway: number // quests underway in it
  open: { key: string; title: string; status: Status; working: boolean }[] // its main quests still to do
}

/** How a quest is titled: a journey card by its journey's title, any other quest by its own. */
export const titleOf = (i: Item) => i.crowns?.title ?? i.title

// `from` needs `to` before it can be done.
export type Need = { from: string; to: string }

export type LogEntry = {
  at: string // as shown ("Sep 24")
  text: string
  id?: string // the item it is about, which may no longer be on the journey
  kind: string // create, add, remove, assign, done, cancel, … — unknown kinds get a plain bullet
}

export type Goal = {
  title: string
  doneWhen: string // id of the final item; empty while the journey has none
}

export type JourneyState = 'active' | 'complete' | 'cancelled'

export type JourneyData = { goal: Goal; items: Item[]; sideQuests: Item[]; needs: Need[]; log: LogEntry[] }

export type JourneyModel = JourneyData & {
  byId: Map<string, Item>
  statusOf: (i: Item) => Status
  openBefore: (i: Item) => number
  /** Everything on the way from `id` to the goal: the cards that (transitively) need it. */
  pathToGoal: (id: string) => Set<string>
  /** How an item is named in running text: repo#n for issues, the title for cards. */
  short: (id: string) => string
  /** The issue ref to show on an item, if it is an issue. */
  refOf: (i: Item) => string | undefined
  /** The issue's GitHub page: the server's url, else built from the ref (the mock has none). */
  urlOf: (i: Item) => string | undefined
  daysSince: (i: Item) => number
  counted: (i: Item) => boolean
}

const afterOwner = (ref: string) => ref.slice(ref.indexOf('/') + 1)

export function journeyModel(data: JourneyData): JourneyModel {
  const { items, sideQuests, needs } = data
  const byId = new Map([...items, ...sideQuests].map((i) => [i.id, i]))

  const settled = (i?: Item) => !!i && (i.done || !!i.cancelled)
  const openBefore = (i: Item) => i.openBefore ?? needs.filter((n) => n.from === i.id && !settled(byId.get(n.to))).length
  const counted = (i: Item) => !i.cancelled

  function statusOf(i: Item): Status {
    if (i.status) return i.status
    if (i.cancelled) return 'cancelled'
    if (i.done) return 'done'
    if (openBefore(i) > 0) return 'locked'
    return i.kind === 'wait' ? 'awaiting' : 'available'
  }

  function pathToGoal(id: string): Set<string> {
    const seen = new Set<string>([id, 'goal'])
    const parent = byId.get(id)?.sideOf
    if (parent) seen.add(parent)
    const stack = [parent ?? id]
    while (stack.length) {
      const cur = stack.pop()!
      for (const n of needs) if (n.to === cur && !seen.has(n.from)) (seen.add(n.from), stack.push(n.from))
    }
    return seen
  }

  const refOf = (i: Item) => i.ref ?? (i.kind === 'issue' ? i.id : undefined)

  function urlOf(i: Item): string | undefined {
    if (i.url) return i.url
    const m = refOf(i)?.match(/^([^/\s]+)\/([^#\s]+)#(\d+)$/)
    return m ? `https://github.com/${m[1]}/${m[2]}/issues/${m[3]}` : undefined
  }

  function short(id: string): string {
    const i = byId.get(id)
    if (!i) return id.includes('/') ? afterOwner(id) : id
    const ref = refOf(i)
    return ref ? afterOwner(ref) : i.title
  }

  return { ...data, byId, statusOf, openBefore, pathToGoal, short, refOf, urlOf, daysSince: (i) => i.sinceDays ?? 0, counted }
}
