import type { CSSProperties } from 'react'
import { Layers } from 'lucide-react'
import type { Item, Status } from './model'

// Every colour is a theme variable (quest.css).

export const words: Record<Status, string> = {
  done: 'Fulfilled',
  available: 'Open',
  locked: 'Sealed',
  awaiting: 'Awaiting reply',
  cancelled: 'Abandoned',
}

export const glow = (v: string, px: number, pct: number) => `0 0 ${px}px color-mix(in srgb, var(${v}) ${pct}%, transparent)`

// What is still to do catches the eye; what is fulfilled steps back (muted, no glow), so a chart
// full of finished deeds still points at the open ones.
const spentGold = 'color-mix(in srgb, var(--gold) 55%, var(--panel))'
export const medal: Record<Status, CSSProperties> = {
  done: { background: spentGold, borderColor: spentGold, color: 'var(--gold-ink)' },
  available: { background: 'var(--avail)', borderColor: 'var(--avail)', color: '#fff' },
  locked: { background: 'var(--locked-medal)', borderColor: 'var(--plate-border)', color: 'var(--ink)' },
  awaiting: { background: 'var(--await)', borderColor: 'var(--await)', color: '#fff' },
  cancelled: { background: 'var(--panel)', borderColor: 'var(--edge-off)', color: 'var(--ink-faint)' },
}
// The quest itself, once its crowning deed is fulfilled: the one win that keeps the full gold.
export const crownMedal: CSSProperties = { background: 'var(--gold)', borderColor: 'var(--gold)', color: 'var(--gold-ink)', boxShadow: glow('--gold', 16, 55) }
export const sideMedal: CSSProperties = { background: 'var(--panel)', borderColor: 'var(--side)', color: 'var(--side)' }

export const plate: Record<Status, CSSProperties> = {
  done: { background: 'var(--panel)', borderColor: 'var(--plate-border)' },
  available: { background: 'var(--avail-plate)', borderColor: 'var(--avail)', boxShadow: glow('--avail', 18, 45) },
  locked: { background: 'var(--plate)', borderColor: 'var(--plate-border)' },
  awaiting: { background: 'var(--await-plate)', borderColor: 'var(--await)' },
  cancelled: { background: 'var(--panel)', borderColor: 'var(--edge-off)', opacity: 0.75 },
}
export const sidePlate: CSSProperties = { background: 'var(--panel)', borderColor: 'color-mix(in srgb, var(--side) 60%, var(--panel))' }

export const stateColour: Record<Status, string> = {
  done: 'color-mix(in srgb, var(--gold) 70%, var(--ink))',
  available: 'var(--avail)',
  locked: 'var(--ink-soft)',
  awaiting: 'var(--await)',
  cancelled: 'var(--ink-faint)',
}

/** A fulfilled deed's own label: still gold, but quiet. The header counts keep stateColour.done. */
export const spentText = 'color-mix(in srgb, var(--gold) 55%, var(--ink-faint))'

export function Hero({ name }: { name?: string }) {
  if (!name)
    return (
      <span className="quest-nohero shrink-0 text-[12px] font-bold tracking-wide whitespace-nowrap text-[#e11d48] uppercase">no hero</span>
    )
  return (
    <span className="flex items-center gap-1 text-[13px] text-[var(--ink-soft)]">
      <span className="grid size-5 place-items-center rounded-full bg-[var(--ink)] text-[11px] font-bold text-[var(--bg)] uppercase">
        {name[0]}
      </span>
      {name}
    </span>
  )
}

// NPC changes nothing about a deed's status; it only makes the deed red on the chart.
export function Npc() {
  return (
    <span
      className="shrink-0 rounded border border-[var(--npc)] bg-[var(--npc)] px-1 text-[11px] font-bold tracking-wider text-[var(--npc-ink)]"
      title="NPC (right-click the deed on the chart to change)"
    >
      NPC
    </span>
  )
}

/** The deed's key (M142), the id people use for it, in front of everything else. */
export function Key({ id }: { id?: string }) {
  if (!id) return null
  return <span className="quest-key shrink-0 font-mono text-[12px] font-semibold text-[var(--ink-faint)]">{id}</span>
}

/** The kind tag in front of a title: the deed's key, then the issue's repo#n or what sort of deed it is. */
export function Label({ item, tag }: { item: Item; tag: string }) {
  const kind = item.crowns ? (
    // A quest card: named by the quest it stands for.
    <span className="flex min-w-0 items-center gap-1">
      <span className="flex shrink-0 items-center gap-0.5 text-[12px] font-bold tracking-wider uppercase" style={{ color: stateColour.done }}>
        <Layers size={12} /> Quest
      </span>
      <span className="quest-ref min-w-0 truncate rounded bg-[var(--chip)] px-1 font-mono text-[12px] text-[var(--ink-soft)]" title={item.crowns.slug}>
        {item.crowns.slug}
      </span>
    </span>
  ) : item.kind === 'wait' ? (
      <span className="quest-kind shrink-0 text-[12px] font-bold tracking-wider text-[var(--await)] uppercase">Petition</span>
    ) : item.kind === 'task' ? (
      <span className="quest-kind shrink-0 text-[12px] font-bold tracking-wider text-[var(--ink-soft)] uppercase">Errand</span>
    ) : (
      // A long repo name is what gives way when the row is tight, never the labels after it.
      <span className="quest-ref min-w-0 truncate rounded bg-[var(--chip)] px-1 font-mono text-[12px] text-[var(--ink-soft)]" title={tag}>
        {tag}
      </span>
    )
  if (!item.key) return kind
  return (
    <>
      <Key id={item.key} /> {kind}
    </>
  )
}

/** Side quests as achievements: stars you can collect after, or instead of, finishing the main quest. */
export function Achievements({ done, total }: { done: number; total: number }) {
  return (
    <div className="flex items-center gap-1 text-[13px] whitespace-nowrap text-[var(--ink-soft)]" title="Side quests fulfilled">
      {/* A star per achievement reads well up to a handful; past that, one star and the count. */}
      {total <= 5 ? (
        Array.from({ length: total }, (_, k) => (
          <span key={k} style={{ color: k < done ? 'var(--side)' : 'var(--edge-off)' }}>
            ★
          </span>
        ))
      ) : (
        <span style={{ color: done > 0 ? 'var(--side)' : 'var(--edge-off)' }}>★</span>
      )}
      <span className="ml-1">
        {done}/{total} achievements
      </span>
    </div>
  )
}

/** Underway: someone is on it right now — a live dot, separate from the computed status. */
export function Working({ compact = false, by }: { compact?: boolean; by?: string }) {
  return (
    <span
      title={by ? `taken up by ${by}` : undefined}
      className={`${compact ? '' : 'quest-working'} inline-flex shrink-0 items-center gap-1 rounded-full font-bold tracking-wider whitespace-nowrap uppercase ${
        compact ? 'text-[11px] text-[var(--avail)]' : 'bg-[var(--avail)] px-2 py-0.5 text-[11px] text-white shadow'
      }`}
    >
      <span className={`quest-working-dot size-2 rounded-full ${compact ? 'bg-[var(--avail)]' : 'bg-white'}`} />
      underway
    </span>
  )
}

/** Which coin a count is struck on in the war-table theme (quest.css); other themes ignore it. */
export type Coin = 'gold' | 'enamel' | 'bronze' | 'silver' | 'wax' | 'iron' | 'crimson'

/** A slim count for the chart header: the number and its word on one line; zeros step back. */
export function Pill({ n, label, colour, coin }: { n: number; label: string; colour: string; coin?: Coin }) {
  return (
    <span
      data-coin={coin}
      className={`quest-pill inline-flex items-baseline gap-1.5 rounded-full border border-[var(--panel-border)] bg-[var(--plate)] px-2.5 py-1 whitespace-nowrap ${
        n === 0 ? 'opacity-45' : ''
      }`}
    >
      <span className="quest-coin text-[15px] leading-none font-bold" style={{ color: n === 0 ? 'var(--ink-faint)' : colour }}>
        {n}
      </span>
      <span className="text-[12px] font-semibold tracking-wide text-[var(--ink-soft)] uppercase">{label}</span>
    </span>
  )
}

export function Stat({ n, label, colour }: { n: number; label: string; colour: string }) {
  return (
    <div className="rounded-md border border-[var(--panel-border)] bg-[var(--plate)] px-3 py-1.5 text-center">
      <div className="text-xl leading-none font-bold" style={{ color: colour }}>
        {n}
      </div>
      <div className="text-[11px] font-semibold tracking-wider text-[var(--ink-soft)] uppercase">{label}</div>
    </div>
  )
}

/** A quest that is over, one way or the other: shown on the Quest Board and in the chart header. */
export function QuestStateChip({ state }: { state?: string }) {
  if (state !== 'complete' && state !== 'cancelled') return null
  const complete = state === 'complete'
  return (
    <span
      className="inline-flex shrink-0 items-center rounded-full border-2 px-2 py-0.5 text-[11px] font-bold tracking-[0.15em] uppercase"
      style={
        complete
          ? { background: 'var(--gold)', borderColor: 'var(--gold)', color: 'var(--gold-ink)' }
          : { background: 'var(--panel)', borderColor: 'var(--edge-off)', color: 'var(--ink-faint)' }
      }
    >
      {complete ? 'Fulfilled' : 'Abandoned'}
    </span>
  )
}

/** An archived quest: put away off the Quest Board's shelves, otherwise as it was. */
export function ArchivedChip() {
  return (
    <span
      className="inline-flex shrink-0 items-center rounded-full border-2 border-[var(--edge-off)] px-2 py-0.5 text-[11px] font-bold tracking-[0.15em] text-[var(--ink-soft)] uppercase"
      title="Archived: off the Quest Board's shelves (mikado quest unarchive brings it back)"
    >
      Archived
    </span>
  )
}
