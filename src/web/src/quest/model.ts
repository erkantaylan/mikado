// A journey as the war table and the atlas draw it, whether it comes from the mock data or
// from the API. Pages build a JourneyModel from JourneyData once per version of
// the data; everything that used to read module-level mock state reads the model.

// Every quest is one kind of thing; a `wait` (a petition) is one that awaits a reply from someone.
export type Kind = 'quest' | 'wait'

// Completed: done. Available: everything before it is done. Locked: something
// before it is still open. Awaiting: an available card that waits on someone else.
// Cancelled: won't be done — it stays on the map but blocks nothing and counts for nothing.
export type Status = 'done' | 'available' | 'locked' | 'awaiting' | 'cancelled'

export type Item = {
  id: string // the node id: API cards c<id>; the mock uses names of its own
  key?: string // the key people use for the quest (Q142)
  kind: Kind
  title: string
  done: boolean
  url?: string // any link: the details panel links the title to it, a card its mark
  mark?: string // a short free text shown after the key: #13, 234g45a, PROJ-88
  assignee?: string // the hero: whoever is responsible for it
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

/**
 * A quest's link as an href: an address with a scheme of its own is kept only when it is a web or mail
 * one (never javascript: or data:), and one without is taken for a web address.
 */
export function hrefOf(url?: string): string | undefined {
  const u = url?.trim()
  if (!u) return undefined
  const scheme = u.match(/^([a-z][a-z0-9+.-]*):/i)?.[1].toLowerCase()
  if (!scheme) return `https://${u}`
  return ['http', 'https', 'mailto'].includes(scheme) ? u : undefined
}

// `from` needs `to` before it can be done.
export type Need = { from: string; to: string }

export type LogEntry = {
  at: string // as shown ("Sep 24")
  text: string
  id?: string // the item it is about, which may no longer be on the journey
  kind: string // create, add, remove, done, cancel, … — unknown kinds get a plain bullet
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
  /** How an item is named in running text: its title (its id when it is not on the chart). */
  short: (id: string) => string
  daysSince: (i: Item) => number
  counted: (i: Item) => boolean
}

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

  const short = (id: string) => byId.get(id)?.title ?? id

  return { ...data, byId, statusOf, openBefore, short, daysSince: (i) => i.sinceDays ?? 0, counted }
}
