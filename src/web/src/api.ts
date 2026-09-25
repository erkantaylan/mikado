// The mikado JSON API, typed after the Go structs in internal/store/model.go.

export type Health = { status: string; version: string }

export type CardKind = 'issue' | 'errand' | 'awaiting'
export type CardStatus = 'cancelled' | 'done' | 'locked' | 'awaiting' | 'available'
export type QuestState = 'active' | 'complete' | 'cancelled'

export type Progress = { done: number; total: number }

/** One row of the quest board (store.QuestSummary). */
export type QuestSummary = {
  slug: string
  title: string
  main: Progress
  achievements: Progress
  state: QuestState
  available: number
  awaiting: number
  cancelled: number
  inProgress: number
  heroes: string[]
  repos: string[] // owner/repo
  lastActivity: string // RFC 3339
  archivedAt?: string // RFC 3339, set while archived
  blockedBy?: QuestLink[] // quests whose crowning deeds are on this quest's chart as quest cards; older servers send none
  blocks?: QuestLink[] // quests with this one's crowning deed on their chart
}

/** A quest named with its state, as the board links it (store.QuestLink). */
export type QuestLink = { slug: string; title: string; state: QuestState; archivedAt?: string }

/** A deed still to do in a quest, as its quest card lists it (store.OpenDeed). */
export type OpenDeed = { key: string; title: string; status: CardStatus; working: boolean }

/**
 * The quest a card crowns (store.Crowns). On a chart it is set on a card that stands for another
 * quest: that quest's state, its main-quest progress as the board counts it, and what is left in it.
 */
export type Crowns = {
  slug: string
  title: string
  state: QuestState
  archivedAt?: string
  done: number
  total: number
  working: number // deeds underway in it
  open: OpenDeed[]
}

/** Another quest the same card belongs to. */
export type QuestRef = { slug: string; title: string }

/** A card with what GitHub says about it and its computed status (store.Card). */
export type Card = {
  id: number
  key?: string // the id people use (M142); older servers do not send it
  kind: CardKind
  ref?: string // issues: owner/repo#n
  url?: string
  title: string
  done: boolean
  state?: string // issues: GitHub's open / closed
  assignees: string[]
  owner?: string
  waitingOn?: string
  since?: string // awaiting: YYYY-MM-DD
  final: boolean
  sideOf?: number
  foundWhile?: number
  reason?: string
  npc: boolean
  cancelled: boolean
  cancelReason?: string
  stateReason?: string
  working: boolean
  workingSince?: string
  workingBy?: string
  status: CardStatus
  openBefore: number
  alsoIn?: QuestRef[]
  crowns?: Crowns // it crowns another quest and stands for that whole quest on this chart
}

/** `from` needs `to` done first. */
export type Need = { from: number; to: number }

/** One line of the quest log (store.Event). */
export type LogEvent = {
  id: number
  at: string // RFC 3339
  kind: string
  cardId?: number
  text: string
}

export type QuestInfo = {
  slug: string
  title: string
  finalCardId: number | null
  state: QuestState
  archivedAt?: string // RFC 3339, set while archived
}

/** Everything the quest map needs (store.QuestView). */
export type QuestView = {
  quest: QuestInfo
  cards: Card[]
  needs: Need[]
  log: LogEvent[]
  github?: string // set when GitHub could not be reached and cached data is shown
}

/**
 * A failed request. `status` is 0 when the server could not be reached at all;
 * `down` is true then, and also when something other than the API answered
 * (a proxy's error page, say), so the page can say the API is down.
 */
export class ApiError extends Error {
  readonly status: number
  readonly down: boolean
  constructor(status: number, message: string, down: boolean) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.down = down
  }
}

async function get<T>(path: string, signal?: AbortSignal): Promise<{ body: T; res: Response }> {
  let res: Response
  try {
    res = await fetch(path, { headers: { Accept: 'application/json' }, signal })
  } catch (e) {
    if (signal?.aborted) throw e
    throw new ApiError(0, e instanceof Error ? e.message : String(e), true)
  }
  const text = await res.text()
  let body: unknown
  try {
    body = JSON.parse(text)
  } catch {
    throw new ApiError(res.status, `the API answered ${res.status} with something that is not JSON`, true)
  }
  if (!res.ok) {
    const msg = typeof body === 'object' && body !== null && 'error' in body ? String(body.error) : `HTTP ${res.status}`
    throw new ApiError(res.status, msg, false)
  }
  return { body: body as T, res }
}

/** A change: a JSON body (the API refuses anything else), the JSON answer back. */
async function send<T>(method: 'PATCH' | 'POST' | 'DELETE', path: string, body: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, { method, headers: { Accept: 'application/json', 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
  } catch (e) {
    throw new ApiError(0, e instanceof Error ? e.message : String(e), true)
  }
  const out: unknown = await res.json().catch(() => undefined)
  if (!res.ok) {
    const msg = typeof out === 'object' && out !== null && 'error' in out ? String(out.error) : `HTTP ${res.status}`
    throw new ApiError(res.status, msg, out === undefined)
  }
  return out as T
}

/** Retitles a quest; its slug, and so its links, stay as they are. */
export async function retitleQuest(slug: string, title: string): Promise<QuestSummary> {
  return send<QuestSummary>('PATCH', `/api/quests/${encodeURIComponent(slug)}`, { title })
}

/** Marks a deed as an NPC, or not: it only changes how the deed looks on the chart. */
export async function setNpc(id: number, npc: boolean): Promise<Card> {
  return send<Card>('PATCH', `/api/cards/${id}`, { npc })
}

export async function fetchHealth(): Promise<Health> {
  return (await get<Health>('/api/health')).body
}

/** The quest board, and GitHub's warning if its data is stale. */
export async function fetchQuests(): Promise<{ quests: QuestSummary[]; github?: string }> {
  const { body, res } = await get<QuestSummary[]>('/api/quests')
  return { quests: body, github: res.headers.get('X-Mikado-GitHub') ?? undefined }
}

/** What the search popup finds (store.SearchResult). `exact` is the deed the query names by id or issue. */
export type SearchResult = { quests: QuestInfo[]; deeds: Card[]; exact?: Card }

export async function fetchSearch(q: string, signal?: AbortSignal): Promise<SearchResult> {
  return (await get<SearchResult>(`/api/search?q=${encodeURIComponent(q)}`, signal)).body
}

export async function fetchQuest(slug: string): Promise<QuestView> {
  return (await get<QuestView>(`/api/quests/${encodeURIComponent(slug)}`)).body
}
