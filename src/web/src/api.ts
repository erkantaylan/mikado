// The mikado JSON API, typed after the Go structs in internal/store/model.go.

export type Health = { status: string; version: string }

/** A quest's kind: only a petition (awaiting a reply from someone) has one. */
export type CardKind = 'awaiting'
export type CardStatus = 'cancelled' | 'done' | 'locked' | 'awaiting' | 'available'
export type JourneyState = 'active' | 'complete' | 'cancelled'

export type Progress = { done: number; total: number }

/** A region named on a journey (store.RegionRef): every journey lives in one. */
export type RegionRef = { key: string; name: string } // R2

/** A region as the region list has it (store.Region). */
export type Region = RegionRef & { createdAt: string; journeys: number }

/** One journey on the atlas (store.JourneySummary). */
export type JourneySummary = {
  key: string // J7
  title: string
  region: RegionRef
  main: Progress
  achievements: Progress
  state: JourneyState
  available: number
  awaiting: number
  cancelled: number
  inProgress: number
  heroes: string[]
  lastActivity: string // RFC 3339
  archivedAt?: string // RFC 3339, set while archived
  blockedBy: JourneyLink[] // journeys whose crowning quests are on this journey's chart as journey cards
  blocks: JourneyLink[] // journeys with this one's crowning quest on their chart
}

/** A journey named with its state, as the atlas links it (store.JourneyLink). */
export type JourneyLink = { key: string; title: string; state: JourneyState; archivedAt?: string }

/** A quest still to do in a journey, as its journey card lists it (store.OpenQuest). */
export type OpenQuest = { key: string; title: string; status: CardStatus; working: boolean }

/**
 * The journey a card crowns (store.Crowns). On a chart it is set on a card that stands for another
 * journey: that journey's state, its main-quest progress as the atlas counts it, and what is left in it.
 */
export type Crowns = {
  key: string // J7
  title: string
  state: JourneyState
  archivedAt?: string
  done: number
  total: number
  working: number // quests underway in it
  open: OpenQuest[]
}

/** Another journey the same card belongs to. */
export type JourneyRef = { key: string; title: string }

/** A card — a quest — with its computed status (store.Card). */
export type Card = {
  id: number
  key: string // the key people use (Q142)
  kind?: CardKind // set on a petition only
  title: string
  url?: string // any link
  mark?: string // a short free text shown by the key: #13, 234g45a, PROJ-88
  done: boolean
  owner?: string // the hero
  waitingOn?: string
  since?: string // awaiting: YYYY-MM-DD
  final: boolean
  sideOf?: number
  foundWhile?: number
  reason?: string
  npc: boolean
  cancelled: boolean
  cancelReason?: string
  working: boolean
  workingSince?: string
  workingBy?: string
  status: CardStatus
  openBefore: number
  alsoIn?: JourneyRef[]
  crowns?: Crowns // it crowns another journey and stands for that whole journey on this chart
}

/** `from` needs `to` done first. */
export type Need = { from: number; to: number }

/** One line of the chronicle (store.Event). */
export type LogEvent = {
  id: number
  at: string // RFC 3339
  kind: string
  cardId?: number
  text: string
}

export type JourneyInfo = {
  key: string // J7
  title: string
  finalCardId: number | null
  state: JourneyState
  archivedAt?: string // RFC 3339, set while archived
  region: RegionRef
}

/** Everything the war table needs (store.JourneyView). */
export type JourneyView = {
  journey: JourneyInfo
  cards: Card[]
  needs: Need[]
  log: LogEvent[]
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

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
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
  return body as T
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

/** Retitles a journey; its key, and so its links, stay as they are. */
export async function retitleJourney(key: string, title: string): Promise<JourneySummary> {
  return send<JourneySummary>('PATCH', `/api/journeys/${encodeURIComponent(key)}`, { title })
}

/** Marks a quest as an NPC, or not: it only changes how the quest looks on the chart. */
export async function setNpc(id: number, npc: boolean): Promise<Card> {
  return send<Card>('PATCH', `/api/cards/${id}`, { npc })
}

export async function fetchHealth(): Promise<Health> {
  return get<Health>('/api/health')
}

/** The atlas. */
export async function fetchJourneys(): Promise<JourneySummary[]> {
  return get<JourneySummary[]>('/api/journeys')
}

export async function renameRegion(key: string, name: string): Promise<Region> {
  return send<Region>('PATCH', `/api/regions/${encodeURIComponent(key)}`, { name })
}

export async function fetchRegions(): Promise<Region[]> {
  return get<Region[]>('/api/regions')
}

/** What the search popup finds (store.SearchResult). `exact` is the quest the query names by key. */
export type SearchResult = { journeys: JourneyInfo[]; quests: Card[]; exact?: Card }

export async function fetchSearch(q: string, signal?: AbortSignal): Promise<SearchResult> {
  return get<SearchResult>(`/api/search?q=${encodeURIComponent(q)}`, signal)
}

export async function fetchJourney(key: string): Promise<JourneyView> {
  return get<JourneyView>(`/api/journeys/${encodeURIComponent(key)}`)
}
