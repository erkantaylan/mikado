import type { CSSProperties, ReactNode } from 'react'
import { Handle, Position } from '@xyflow/react'
import { Ban, Check, ExternalLink, Flag } from 'lucide-react'
import type { JourneyCard, JourneyState, Status } from './model'
import { Cover, Gate, Working, medal, stateColour, words } from './look'
import { useWarTable } from './theme'
import { IdTag } from './wood'

// A journey's card, as the chart draws it twice: the chart's own journey (the goal, at the right end), and
// a quest on this chart that crowns another journey, standing for that whole journey. Both are one face
// (JourneyFace, .quest-journey in quest.css), so they cannot drift apart; only the goal wears the trophy
// (and on the war table the crown and castle), and only the other journey's card the marks of a quest.

const hidden = '!opacity-0'

export const journeyHref = (key: string) => `/journey/${encodeURIComponent(key)}`

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

const journeyWord: Record<JourneyState, string> = { active: 'Journey', complete: 'Journey fulfilled', cancelled: 'Journey abandoned' }

type FaceProps = {
  id?: string // J7; the mock's journey has none
  title: string
  state: JourneyState
  done: number
  total: number
  archived?: boolean
  top?: ReactNode // on its top edge: the goal's trophy
  bottom?: ReactNode // under the progress: the goal's achievements, the other journey's status and link
  className?: string
  style?: CSSProperties
  data?: Record<`data-${string}`, string | boolean | undefined>
  children?: ReactNode // the node's handles
}

/**
 * A journey's card face: its id and state, its title and its main-quest progress, on the journey's own
 * paper: a sheet with a gold rule inset from its edge (paper-journey.webp on the war table, a thin CSS rule
 * in the other themes). The sheet keeps its proportions, so the rule always sits where the padding expects
 * it and everything written stays inside it; a long title is cut short, in full on hover. Its sizes and
 * colours are in quest.css (.quest-journey), per theme and per status.
 */
export function JourneyFace({ id, title, state, done, total, archived, top, bottom, className = '', style, data, children }: FaceProps) {
  const pct = total ? Math.round((done / total) * 100) : 0
  return (
    <div
      {...data}
      style={style}
      className={`quest-journey relative flex flex-col items-center justify-center gap-1.5 rounded-2xl text-center ${className}`}
    >
      <span aria-hidden className="quest-journey-paper" />
      {children}
      {top}
      <span className="flex max-w-full flex-wrap items-center justify-center gap-x-1.5 gap-y-1">
        {id && <IdTag id={id} />}
        <span
          className="quest-journey-word text-[12px] font-bold"
          style={{ color: state === 'cancelled' ? stateColour.cancelled : stateColour.done }}
        >
          {journeyWord[state]}
        </span>
        {archived && <span className="text-[11px] font-semibold text-[var(--ink-faint)]">· Archived</span>}
      </span>
      <span className="quest-display quest-journey-title line-clamp-2 text-[15px] leading-tight font-semibold" title={title}>
        {title}
      </span>
      <div className="flex w-full flex-col items-center gap-1">
        <span className="quest-journey-count text-[13px] leading-none whitespace-nowrap text-[var(--ink-soft)]">
          Main quest {done}/{total}
        </span>
        <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--chip)] ring-1 ring-[var(--plate-border)]">
          <div className="quest-progress h-full bg-[var(--gold)]" style={{ width: `${pct}%` }} />
        </div>
      </div>
      {bottom}
    </div>
  )
}

const stateOf = (status: Status): JourneyState => (status === 'done' ? 'complete' : status === 'cancelled' ? 'cancelled' : 'active')

type Props = { q: JourneyCard; status: Status; selected: boolean; covered?: boolean }

/**
 * A journey this one waits on: the journey's own face, with the marks every quest wears (the state badge
 * or the war table's gate on its corner, underway on the other) and its status and a link to its chart
 * along the bottom. It counts as one quest here.
 */
export function JourneyCardView({ q, status, selected, covered }: Props) {
  const wt = useWarTable()
  const done = status === 'done'
  const over = done || status === 'cancelled'
  const underway = q.underway > 0 && !over
  return (
    <div data-status={status} data-covered={covered || undefined} className="quest-item relative cursor-pointer">
      <Handle type="target" position={Position.Left} className={hidden} />
      {covered && <Cover status={status} name={q.key} style={{ borderRadius: '1rem' }} />}
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
      {underway && (
        <span className="absolute -top-3 right-5 z-10">
          <Working by={`${q.underway} ${q.underway === 1 ? 'quest' : 'quests'} in ${q.key}`} />
        </span>
      )}
      <JourneyFace
        id={q.key}
        title={q.title}
        state={stateOf(status)}
        done={q.done}
        total={q.total}
        archived={q.archived}
        className={selected ? 'quest-lit' : ''}
        data={{ 'data-status': status }}
        bottom={
          <div className="flex w-full items-center justify-between gap-2">
            <span className="quest-status text-[12px] font-bold" style={{ color: stateColour[status] }}>
              {!over && journeyCardWord(q, status)}
            </span>
            <a
              href={journeyHref(q.key)}
              onClick={(e) => e.stopPropagation()}
              title={`Open journey ${q.key}`}
              className="quest-journey-link nodrag nopan flex shrink-0 items-center gap-0.5 text-[12px] font-semibold text-[var(--ink-soft)] hover:text-[var(--ink)] hover:underline"
            >
              chart <ExternalLink size={12} />
            </a>
          </div>
        }
      />
      <Handle type="source" position={Position.Right} className={hidden} />
    </div>
  )
}
