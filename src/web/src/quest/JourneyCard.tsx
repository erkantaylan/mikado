import type { CSSProperties } from 'react'
import { Handle, Position } from '@xyflow/react'
import { Ban, Check, ExternalLink, Flag, Layers } from 'lucide-react'
import type { Item, JourneyCard, Status } from './model'
import { Gate, Key, Kinds, Working, glow, medal, plate, spentText, stateColour, words } from './look'
import { useWarTable } from './theme'

// A journey card: a quest on this chart that crowns another journey, drawn as one card standing for
// that whole journey, its own quests folded behind it. It counts as one quest here.

const CARD_WIDTH = 310 // as graph.tsx's quests
const STACK = 5 // how far each folded plate peeks out behind the card
const hidden = '!opacity-0'

export const journeyHref = (key: string) => `/journey/${encodeURIComponent(key)}`

/** The folded plates behind a journey card: its own quests, stacked. They step back like a fulfilled quest. */
function behind(status: Status, depth: number): CSSProperties {
  const base = plate[status]
  const done = status === 'done' || status === 'cancelled'
  return {
    background: `color-mix(in srgb, ${base.background} ${done ? 80 : 88 - depth * 10}%, var(--bg))`,
    borderColor: base.borderColor as string,
    opacity: done ? 0.55 : 1 - depth * 0.15,
    transform: `translate(${STACK * depth}px, ${STACK * depth}px)`,
  }
}

/** The other journey's main-quest progress, as the Atlas counts it. */
export function JourneyProgress({ q, done }: { q: JourneyCard; done: boolean }) {
  const pct = q.total ? Math.round((q.done / q.total) * 100) : 0
  return (
    <div className="flex items-center gap-2">
      <div className="h-2 flex-1 overflow-hidden rounded-full bg-[var(--chip)] ring-1 ring-[var(--plate-border)]">
        <div
          className="quest-progress h-full"
          style={{ width: `${pct}%`, background: done ? 'color-mix(in srgb, var(--gold) 55%, var(--panel))' : 'var(--gold)' }}
        />
      </div>
      <span className="shrink-0 text-[13px] font-semibold text-[var(--ink-soft)] tabular-nums">
        {q.done}/{q.total}
      </span>
    </div>
  )
}

/** The status word a journey card shows: the journey's own when it is over, else its crowning quest's. */
export function journeyCardWord(q: JourneyCard, status: Status): string {
  if (status === 'done') return 'Journey fulfilled'
  if (status === 'cancelled') return 'Journey abandoned'
  if (status === 'locked') return `${words.locked} · ${q.total - q.done} to go`
  return words[status]
}

type Props = { item: Item; q: JourneyCard; status: Status; dim: boolean; selected: boolean }

/** The journey card node. */
export function JourneyCardView({ item, q, status, dim, selected }: Props) {
  const wt = useWarTable()
  const done = status === 'done'
  const over = done || status === 'cancelled'
  const underway = q.underway > 0 && !over
  const working = <Working by={`${q.underway} ${q.underway === 1 ? 'quest' : 'quests'} in ${q.key}`} />
  const front: CSSProperties = { ...plate[status], ...(status === 'available' ? { boxShadow: glow('--avail', 18, 45) } : {}) }
  return (
    <div
      style={{ width: CARD_WIDTH + STACK * 2, paddingRight: STACK * 2, paddingBottom: STACK * 2 }}
      data-status={status}
      data-dim={dim || undefined}
      className={`quest-item quest-stack relative cursor-pointer transition-opacity ${dim ? 'opacity-25' : ''}`}
    >
      <Handle type="target" position={Position.Left} className={hidden} />
      {/* The journey's own quests, folded behind it. */}
      {[2, 1].map((depth) => (
        <div
          key={depth}
          aria-hidden
          style={{ ...behind(status, depth), right: STACK * 2, bottom: STACK * 2 }}
          className="quest-behind absolute top-0 left-0 rounded-lg border-2"
        />
      ))}
      {wt ? (
        // The war table's gate: can it be started? A seal counts the journey's quests still to go, as its label does.
        <Gate status={status} count={q.total - q.done} />
      ) : (
        <span
          style={medal[status]}
          data-mark={status}
          className={`quest-medal absolute -top-3 -left-3 z-10 grid size-9 place-items-center rounded-full border-2 ${status === 'available' ? 'quest-available' : ''}`}
        >
          {done ? <Check size={16} strokeWidth={3} /> : status === 'cancelled' ? <Ban size={16} /> : <Flag size={16} />}
        </span>
      )}
      {underway && !wt && <span className="absolute -top-3 right-5 z-10">{working}</span>}
      <div style={front} className={`quest-plate relative flex flex-col gap-1.5 rounded-lg border-2 py-2 pr-3 pl-5 ${selected ? 'quest-lit' : ''}`}>
        <div className="flex min-w-0 items-center gap-1.5 pl-2.5">
          <Key id={item.key} />
          <span
            className="flex shrink-0 items-center gap-1 text-[12px] font-bold tracking-[0.2em] uppercase"
            style={{ color: over ? spentText : stateColour.done }}
          >
            <Layers size={13} /> Journey
          </span>
          <span className="min-w-0 truncate rounded bg-[var(--chip)] px-1 font-mono text-[12px] text-[var(--ink-soft)]" title={q.key}>
            {q.key}
          </span>
          {q.archived && (
            <span className="shrink-0 text-[11px] font-semibold tracking-wide text-[var(--ink-faint)] uppercase">archived</span>
          )}
          {wt && <Kinds kinds={['journey']}>{underway && working}</Kinds>}
        </div>
        <span
          className={`quest-display quest-title leading-snug ${
            over ? 'text-[15px] font-medium text-[var(--ink-soft)]' : 'text-[15px] font-semibold'
          } ${status === 'cancelled' ? 'line-through' : ''}`}
        >
          {q.title}
        </span>
        <JourneyProgress q={q} done={over} />
        <div className="flex items-center justify-between">
          <span className="text-[12px] font-bold tracking-wider uppercase" style={{ color: done ? spentText : stateColour[status] }}>
            {journeyCardWord(q, status)}
          </span>
          <a
            href={journeyHref(q.key)}
            onClick={(e) => e.stopPropagation()}
            title={`Open journey ${q.key}`}
            className="flex shrink-0 items-center gap-0.5 text-[12px] font-semibold text-[var(--ink-soft)] hover:text-[var(--ink)] hover:underline"
          >
            open <ExternalLink size={12} />
          </a>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className={hidden} />
    </div>
  )
}
