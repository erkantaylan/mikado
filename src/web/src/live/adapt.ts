// Turns the API's JSON into the shapes the shared pages draw.
import type { Card, JourneySummary, JourneyView, LogEvent } from '../api'
import type { AtlasJourney } from '../quest/Atlas'
import type { Item, JourneyData, Kind } from '../quest/model'

const kinds: Record<Card['kind'], Kind> = { issue: 'issue', errand: 'task', awaiting: 'wait' }

/** A node id for a card. */
export const cardId = (id: number) => `c${id}`

const shortDate = (d: Date) => d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })

/** "Sep 24" for an RFC 3339 time or a YYYY-MM-DD date, read as a local date. */
export function dayLabel(s: string): string {
  const d = localDay(s) ?? new Date(s)
  return Number.isNaN(d.getTime()) ? s : shortDate(d)
}

function localDay(s: string): Date | undefined {
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : undefined
}

/** Whole calendar days from a YYYY-MM-DD date to today. */
export function daysSince(ymd: string, now = new Date()): number {
  const d = localDay(ymd)
  if (!d) return 0
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  return Math.max(0, Math.round((today.getTime() - d.getTime()) / 86_400_000))
}

export function toItem(c: Card): Item {
  return {
    id: cardId(c.id),
    key: c.key,
    kind: kinds[c.kind],
    title: c.title,
    done: c.done,
    ref: c.ref,
    url: c.url,
    // The hero is whoever GitHub has it assigned to, else the card's owner.
    assignee: c.assignees[0] ?? (c.owner || undefined),
    waitingOn: c.waitingOn,
    since: c.since ? dayLabel(c.since) : undefined,
    sinceDays: c.since ? daysSince(c.since) : undefined,
    final: c.final,
    foundWhile: c.foundWhile != null ? cardId(c.foundWhile) : undefined,
    sideOf: c.sideOf != null ? cardId(c.sideOf) : undefined,
    npc: c.npc,
    cancelled: c.cancelled,
    cancelReason: c.cancelReason,
    working: c.working,
    workingBy: c.workingBy,
    reason: c.reason,
    alsoIn: c.alsoIn,
    status: c.status,
    openBefore: c.openBefore,
    crowns: c.crowns && {
      key: c.crowns.key,
      title: c.crowns.title,
      state: c.crowns.state,
      archived: !!c.crowns.archivedAt,
      done: c.crowns.done,
      total: c.crowns.total,
      underway: c.crowns.working,
      open: c.crowns.open.map((d) => ({ key: d.key, title: d.title, status: d.status, working: d.working })),
    },
  }
}

const toLog = (e: LogEvent) => ({ at: dayLabel(e.at), text: e.text, kind: e.kind, id: e.cardId != null ? cardId(e.cardId) : undefined })

export function toJourneyData(v: JourneyView): JourneyData {
  const items = v.cards.map(toItem)
  return {
    goal: { title: v.journey.title, doneWhen: v.journey.finalCardId != null ? cardId(v.journey.finalCardId) : '' },
    items: items.filter((i) => !i.sideOf),
    sideQuests: items.filter((i) => i.sideOf),
    needs: v.needs.map((n) => ({ from: cardId(n.from), to: cardId(n.to) })),
    log: v.log.map(toLog),
  }
}

/** A journey on the atlas; repos are shown by name, as the atlas has room for. */
export function toAtlasJourney(q: JourneySummary): AtlasJourney {
  return {
    ...q,
    repos: [...new Set(q.repos.map((r) => r.slice(r.indexOf('/') + 1)))],
    lastActivity: dayLabel(q.lastActivity),
  }
}
