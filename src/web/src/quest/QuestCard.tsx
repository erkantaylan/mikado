import type { CSSProperties } from 'react'
import { Handle, Position } from '@xyflow/react'
import { Ban, Check, ExternalLink, Flag, Layers } from 'lucide-react'
import type { Item, QuestCard, Status } from './model'
import { Key, Working, glow, medal, plate, spentText, stateColour, words } from './look'

// A quest card: a deed on this chart that crowns another quest, drawn as one card standing for
// that whole quest, its own deeds folded behind it. It counts as one deed here.

const CARD_WIDTH = 310 // as graph.tsx's deeds
const STACK = 5 // how far each folded plate peeks out behind the card
const hidden = '!opacity-0'

export const questHref = (slug: string) => `/quest/${encodeURIComponent(slug)}`

/** The folded plates behind a quest card: its own deeds, stacked. They step back like a fulfilled deed. */
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

/** The other quest's main-quest progress, as the Quest Board counts it. */
export function QuestProgress({ q, done }: { q: QuestCard; done: boolean }) {
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

/** The status word a quest card shows: the quest's own when it is over, else its crowning deed's. */
export function questCardWord(q: QuestCard, status: Status): string {
  if (status === 'done') return 'Quest fulfilled'
  if (status === 'cancelled') return 'Quest abandoned'
  if (status === 'locked') return `${words.locked} · ${q.total - q.done} to go`
  return words[status]
}

type Props = { item: Item; q: QuestCard; status: Status; dim: boolean; selected: boolean }

/** The quest card node. */
export function QuestCardView({ item, q, status, dim, selected }: Props) {
  const done = status === 'done'
  const over = done || status === 'cancelled'
  const front: CSSProperties = { ...plate[status], ...(status === 'available' ? { boxShadow: glow('--avail', 18, 45) } : {}) }
  return (
    <div
      style={{ width: CARD_WIDTH + STACK * 2, paddingRight: STACK * 2, paddingBottom: STACK * 2 }}
      data-status={status}
      data-dim={dim || undefined}
      className={`quest-deed quest-stack relative cursor-pointer transition-opacity ${dim ? 'opacity-25' : ''}`}
    >
      <Handle type="target" position={Position.Left} className={hidden} />
      {/* The quest's own deeds, folded behind it. */}
      {[2, 1].map((depth) => (
        <div
          key={depth}
          aria-hidden
          style={{ ...behind(status, depth), right: STACK * 2, bottom: STACK * 2 }}
          className="quest-behind absolute top-0 left-0 rounded-lg border-2"
        />
      ))}
      <span
        style={medal[status]}
        data-mark={status}
        className={`quest-medal absolute -top-3 -left-3 z-10 grid size-9 place-items-center rounded-full border-2 ${status === 'available' ? 'quest-available' : ''}`}
      >
        {done ? <Check size={16} strokeWidth={3} /> : status === 'cancelled' ? <Ban size={16} /> : <Flag size={16} />}
      </span>
      {q.underway > 0 && !over && (
        <span className="absolute -top-3 right-5 z-10">
          <Working by={`${q.underway} ${q.underway === 1 ? 'deed' : 'deeds'} in ${q.slug}`} />
        </span>
      )}
      <div style={front} className={`quest-plate relative flex flex-col gap-1.5 rounded-lg border-2 py-2 pr-3 pl-5 ${selected ? 'quest-lit' : ''}`}>
        <div className="flex min-w-0 items-center gap-1.5 pl-2.5">
          <Key id={item.key} />
          <span
            className="flex shrink-0 items-center gap-1 text-[12px] font-bold tracking-[0.2em] uppercase"
            style={{ color: over ? spentText : stateColour.done }}
          >
            <Layers size={13} /> Quest
          </span>
          <span className="min-w-0 truncate rounded bg-[var(--chip)] px-1 font-mono text-[12px] text-[var(--ink-soft)]" title={q.slug}>
            {q.slug}
          </span>
          {q.archived && (
            <span className="shrink-0 text-[11px] font-semibold tracking-wide text-[var(--ink-faint)] uppercase">archived</span>
          )}
        </div>
        <span
          className={`quest-display quest-title leading-snug ${
            over ? 'text-[15px] font-medium text-[var(--ink-soft)]' : 'text-[15px] font-semibold'
          } ${status === 'cancelled' ? 'line-through' : ''}`}
        >
          {q.title}
        </span>
        <QuestProgress q={q} done={over} />
        <div className="flex items-center justify-between">
          <span className="text-[12px] font-bold tracking-wider uppercase" style={{ color: done ? spentText : stateColour[status] }}>
            {questCardWord(q, status)}
          </span>
          <a
            href={questHref(q.slug)}
            onClick={(e) => e.stopPropagation()}
            title={`Open quest ${q.slug}`}
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
