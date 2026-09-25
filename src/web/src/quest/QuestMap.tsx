import { useCallback, useEffect, useEffectEvent, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import {
  Background,
  ControlButton,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  getViewportForBounds,
  useNodesState,
  useReactFlow,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Ban, Check, Crown, ExternalLink, Eye, EyeOff, Gem, Hourglass, Layers, Map as MapIcon, PanelRightClose, PanelRightOpen, Pencil, Sparkles, Trophy } from 'lucide-react'
import './quest.css'
import { titleOf, type Item, type QuestCard, type QuestModel, type QuestState, type Status } from './model'
import { ArchivedChip, Gate, Hero, Key, KindMark, Label, Npc, Pill, QuestStateChip, Working, medal, sideMedal, stateColour, words } from './look'
import { glossary } from './glossary'
import {
  AlsoIn,
  FocusWhenPlaced,
  LayoutWhenMeasured,
  buildGraph,
  edgeTypes,
  hiddenDeeds,
  miniClass,
  nodeTypes,
  stroke,
  type Hide,
  type Placed,
  type QuestNode,
} from './graph'
import { Search } from './Search'
import { ThemeContext, ThemeMenu, useTheme } from './theme'
import { VersionLine } from './VersionLine'
import { QuestProgress, questCardWord, questHref } from './QuestCard'

// ---- side panel ------------------------------------------------------------

const logIcon: Record<string, string> = {
  create: '★',
  add: '+',
  remove: '−',
  assign: '⚔',
  unassign: '⚔',
  done: '✓',
  undone: '↺',
  cancel: '⦸',
  uncancel: '↺',
  start: '▸',
  stop: '■',
  need: '→',
  unneed: '↛',
  final: '♛',
  edit: '✎',
}
const logColour: Record<string, string> = {
  create: 'var(--gold)',
  add: 'var(--avail)',
  remove: '#e11d48',
  assign: 'var(--ink-faint)',
  done: 'var(--gold)',
  cancel: 'var(--ink-faint)',
  start: 'var(--avail)',
  final: 'var(--gold)',
}

// How the chronicle names a deed: an issue by repo#n, another deed by its key. A deed that has
// left the quest is only known by its id.
function logName(m: QuestModel, id: string): string {
  const i = m.byId.get(id)
  if (!i) return id.startsWith('card:') ? 'deed' : m.short(id)
  return m.refOf(i) ? m.short(id) : (i.key ?? 'deed')
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="quest-section">
      <h2 className="quest-display quest-section-title mb-2 text-[12px] font-bold tracking-[0.2em] text-[var(--ink-soft)] uppercase">{title}</h2>
      {children}
    </section>
  )
}

function Row({ m, item, onPick, right }: { m: QuestModel; item: Item; onPick: (id: string) => void; right: ReactNode }) {
  return (
    <li>
      <button
        onClick={() => onPick(item.id)}
        className="quest-row flex w-full items-center justify-between gap-2 rounded-md border border-[var(--panel-border)] bg-[var(--plate)] px-2.5 py-2 text-left text-[14px] hover:border-[var(--ink-faint)]"
      >
        {/* Two spans so the war table can set the labels and the title on lines of their own; elsewhere they run inline as one. */}
        <span className="quest-row-text min-w-0 truncate">
          <span className="quest-row-meta">
            <Label item={item} tag={m.short(item.id)} />
          </span>{' '}
          <span className="quest-row-title">{titleOf(item).replace(/^Polish: /, '')}</span>
        </span>
        {right}
      </button>
    </li>
  )
}

function Legend() {
  const line = (style: CSSProperties) => (
    <svg width="36" height="10" className="shrink-0">
      <line x1="0" y1="5" x2="36" y2="5" style={style} />
    </svg>
  )
  const dot = (style: CSSProperties, icon: ReactNode) => (
    <span style={style} className="grid size-7 shrink-0 place-items-center rounded-full border-2">
      {icon}
    </span>
  )
  return (
    <ul className="space-y-2.5 text-[14px] text-[var(--ink-soft)]">
      <li className="flex items-center gap-2">{dot(medal.done, <Check size={14} strokeWidth={3} />)} Fulfilled: finished</li>
      <li className="flex items-center gap-2">{dot(medal.available, <Sparkles size={14} />)} Open: everything it requires is fulfilled</li>
      <li className="flex items-center gap-2">
        {dot(medal.locked, <span className="text-[12px] font-bold">2</span>)} Sealed: the badge counts the deeds it still requires
      </li>
      <li className="flex items-center gap-2">{dot(medal.awaiting, <Hourglass size={14} />)} Awaiting reply: a petition waiting on someone</li>
      <li className="flex items-center gap-2">
        <Working /> someone has taken it up right now
      </li>
      <li className="flex items-center gap-2">{dot(sideMedal, <Gem size={14} />)} Side quest: optional, earns an achievement</li>
      <li className="flex items-center gap-2">{dot(medal.available, <Crown size={14} />)} Crowning deed: fulfil it and the quest is fulfilled</li>
      <li className="flex items-center gap-2">
        <Npc /> a red deed: an NPC; right-click a deed to mark or unmark it
      </li>
      <li className="flex items-center gap-2">{line(stroke.done)} Powered: a fulfilled deed opening one you can do now</li>
      <li className="flex items-center gap-2">{line(stroke.held)} Powered, but its deed still waits on others</li>
      <li className="flex items-center gap-2">{line(stroke.spent)} Spent: between two fulfilled deeds</li>
      <li className="flex items-center gap-2">{line(stroke.locked)} Not powered yet: its deed is not fulfilled</li>
      <li className="flex items-center gap-2">{line(stroke.side)} Side quest, hung on its deed</li>
      <li className="flex items-center gap-2">{line(stroke.bridge)} Bridge: the deeds between are hidden</li>
      <li className="flex items-center gap-2">
        <span className="h-5 w-9 shrink-0 rounded border-2 border-dashed border-[var(--ink-faint)]" /> Dashed plate: unearthed on the way
      </li>
      <li className="flex items-center gap-2">
        {dot(medal.cancelled, <Ban size={14} />)} Abandoned: won't be done — blocks nothing, counts for nothing
      </li>
    </ul>
  )
}

/**
 * The war table's legend: the card read spot by spot, then the lines between cards. Every mark is
 * the one the cards wear (the same components and quest.css rules), and the lines take the edges'
 * own styles and class, so the legend follows any restyle of either.
 */
function WarLegend() {
  const row = (mark: ReactNode, text: ReactNode) => (
    <li className="flex items-center gap-2.5">
      <span className="wt-legend-mark">{mark}</span>
      <span>{text}</span>
    </li>
  )
  const line = (flow: keyof typeof stroke, live = false) => (
    <svg width="36" height="10" className="overflow-visible">
      <path d="M0 5H36" fill="none" className="react-flow__edge-path" style={stroke[flow]} />
      {live && <path d="M0 5H36" fill="none" stroke="var(--gold-ink)" strokeWidth={2} className="quest-flow" />}
    </svg>
  )
  return (
    <div className="wt-legend space-y-5 text-[14px] text-[var(--ink-soft)]">
      <Panel title="Top-left: can it be started?">
        <ul className="space-y-2.5">
          {row(<Gate status="locked" count={2} />, 'Sealed: the whole seal counts the deeds it still requires')}
          {row(<Gate status="available" />, 'Open: the seal is broken; everything it requires is fulfilled')}
          {row(<Gate status="awaiting" />, 'Awaiting reply: a petition waiting on someone')}
          {row(<Gate status="done" />, 'Fulfilled: a flag planted, the paper steps back')}
          {row(<span className="wt-scrap" data-torn />, "Abandoned: torn paper, no mark — won't be done, blocks nothing, counts for nothing")}
        </ul>
      </Panel>
      <Panel title="Top-right: what sort of card?">
        <ul className="space-y-2.5">
          {row(<KindMark kind="side" />, 'Side quest: optional, earns an achievement')}
          {row(<KindMark kind="unearthed" />, 'Unearthed: found along the way')}
          {row(<KindMark kind="quest" />, 'Quest card: stands for another quest, with its own chart')}
          {row(<KindMark kind="npc" />, 'NPC: right-click a deed to mark or unmark it')}
          {row(<Working />, 'Underway: someone has taken it up right now')}
        </ul>
        <p className="mt-2 text-[13px] text-[var(--ink-faint)]">A plain deed has no mark here.</p>
      </Panel>
      <Panel title="Along the bottom">
        <ul className="space-y-2.5">
          {row(
            <span className="text-[12px] font-bold tracking-wider uppercase" style={{ color: stateColour.available }}>
              {words.available}
            </span>,
            'Bottom-left: the status, in words',
          )}
          {row(<Hero />, 'Bottom-right: the hero on it, or no hero yet')}
        </ul>
      </Panel>
      <Panel title="The quest">
        <ul className="space-y-2.5">
          {row(<span className="wt-crown" />, 'Crowning deed: fulfil it and the quest is fulfilled')}
        </ul>
      </Panel>
      <Panel title="Lines between cards">
        <ul className="space-y-2.5">
          {row(line('done', true), 'Powered: a fulfilled deed opening one you can do now')}
          {row(line('held'), 'Powered, but its deed still waits on others')}
          {row(line('spent'), 'Spent: between two fulfilled deeds')}
          {row(line('locked'), 'Not powered yet: its deed is not fulfilled')}
          {row(line('side'), 'Side quest, hung on its deed')}
          {row(line('bridge'), 'Bridge: the deeds between are hidden')}
        </ul>
      </Panel>
    </div>
  )
}

/** Every word mikado uses, with what it means and the command behind it (glossary.ts). */
function Glossary() {
  return (
    <dl className="space-y-3">
      {glossary.map((t) => (
        <div key={t.id}>
          <dt className="flex flex-wrap items-baseline gap-x-2">
            <span className="text-[15px] font-semibold text-[var(--ink)]">{t.term}</span>
            {t.also && <span className="text-[13px] text-[var(--ink-soft)]">or {t.also}, the same thing</span>}
          </dt>
          <dd className="text-[14px] leading-snug text-[var(--ink-soft)]">{t.text}</dd>
          {t.cli && <dd className="mt-0.5 font-mono text-[12px] break-words text-[var(--ink-faint)]">{t.cli}</dd>}
        </div>
      ))}
    </dl>
  )
}

type DetailsProps = { m: QuestModel; item: Item; onPick: (id: string) => void; onClose: () => void }

// Every deed named in the details is a way to it: picking one does what picking a side-panel row does.
function DeedRow({ m, i, onPick }: { m: QuestModel; i: Item; onPick: (id: string) => void }) {
  return (
    <li>
      <button
        onClick={() => onPick(i.id)}
        className="flex w-full items-start gap-2 rounded-md px-1.5 py-1 text-left hover:bg-[var(--chip)]"
      >
        <span className="min-w-0 flex-1">
          <span className="whitespace-nowrap">
            <Label item={i} tag={m.short(i.id)} />
          </span>{' '}
          {titleOf(i)}
        </span>{' '}
        <span className="shrink-0 pt-px text-[12px] font-bold uppercase" style={{ color: stateColour[m.statusOf(i)] }}>
          {words[m.statusOf(i)]}
        </span>
      </button>
    </li>
  )
}

function Details(props: DetailsProps) {
  return props.item.crowns ? <QuestDetails {...props} q={props.item.crowns} /> : <DeedDetails {...props} />
}

/** The details of a quest card: the quest it stands for, its progress and what is left in it. */
function QuestDetails({ m, item, q, onPick, onClose }: DetailsProps & { q: QuestCard }) {
  const status = m.statusOf(item)
  const before = m.needs.filter((n) => n.from === item.id).flatMap((n) => m.byId.get(n.to) ?? [])
  const opens = m.needs.filter((n) => n.to === item.id).flatMap((n) => m.byId.get(n.from) ?? [])
  const over = status === 'done' || status === 'cancelled'
  const others = (item.alsoIn ?? []).filter((r) => r.slug !== q.slug)
  const heading = 'mb-1 text-[12px] font-bold tracking-wider text-[var(--ink-faint)] uppercase'
  const crownedBy = m.refOf(item) ? m.short(item.id) : (item.key ?? item.title)
  return (
    <section className="quest-details space-y-3 rounded-lg border-2 border-[var(--panel-border)] bg-[var(--plate)] p-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-1 font-mono text-[12px] text-[var(--ink-faint)]">
            <Key id={item.key} /> · <Layers size={12} /> quest · {q.slug}
            {q.archived && ' · archived'}
          </div>
          <div className="quest-display text-[16px] font-semibold">{q.title}</div>
          <a
            href={questHref(q.slug)}
            className="mt-2 inline-flex items-center gap-1.5 rounded-md border-2 border-[var(--ink)] bg-[var(--ink)] px-2.5 py-1 text-[13px] font-semibold text-[var(--bg)] hover:opacity-85"
          >
            Open quest <ExternalLink size={14} />
          </a>
        </div>
        <button onClick={onClose} className="text-[var(--ink-faint)] hover:text-[var(--ink)]" aria-label="Close details">
          ✕
        </button>
      </div>
      <div className="flex items-center gap-3">
        <span className="text-[13px] font-bold tracking-wider uppercase" style={{ color: stateColour[status] }}>
          {questCardWord(q, status)}
        </span>
        {q.underway > 0 && !over && <Working compact />}
        <Hero name={item.assignee} />
      </div>
      <div>
        <QuestProgress q={q} done={over} />
        <div className="mt-1 text-[13px] text-[var(--ink-soft)]">
          {q.done} of {q.total} deeds fulfilled{q.underway > 0 && !over && ` · ${q.underway} underway`}
        </div>
      </div>
      <p className="rounded bg-[var(--chip)] p-2 text-[14px]">
        A quest of its own, crowned by <b className="font-mono text-[13px]">{crownedBy}</b>. Here it counts as one deed;{' '}
        {status === 'done'
          ? 'it was fulfilled when that quest was.'
          : status === 'cancelled'
            ? 'it was abandoned with that quest.'
            : 'it is fulfilled when that quest is.'}
      </p>
      {q.open.length > 0 && (
        <div>
          <div className={heading}>Still to do in it · {q.open.length}</div>
          <ul className="space-y-0.5 text-[14px]">
            {q.open.map((d) => (
              <li key={d.key} className="flex items-start gap-2 px-1.5 py-1">
                <span className="min-w-0 flex-1">
                  <Key id={d.key} /> {d.title}
                </span>
                {d.working && <Working compact />}
                <span className="shrink-0 pt-px text-[12px] font-bold uppercase" style={{ color: stateColour[d.status] }}>
                  {words[d.status]}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
      {others.length > 0 && (
        <div>
          <div className={heading}>Also in</div>
          <AlsoIn quests={others} label={false} />
        </div>
      )}
      {before.length > 0 && (
        <div>
          <div className={heading}>Requires</div>
          <ul className="space-y-0.5 text-[14px]">{before.map((i) => <DeedRow key={i.id} m={m} i={i} onPick={onPick} />)}</ul>
        </div>
      )}
      <div>
        <div className={heading}>Opens</div>
        <ul className="space-y-0.5 text-[14px]">
          {opens.length ? opens.map((i) => <DeedRow key={i.id} m={m} i={i} onPick={onPick} />) : <li className="font-semibold">The quest itself: this is its crowning deed.</li>}
        </ul>
      </div>
    </section>
  )
}

function DeedDetails({ m, item, onPick, onClose }: DetailsProps) {
  const status = m.statusOf(item)
  const before = m.needs.filter((n) => n.from === item.id).flatMap((n) => m.byId.get(n.to) ?? [])
  const opens = m.needs.filter((n) => n.to === item.id).flatMap((n) => m.byId.get(n.from) ?? [])
  const ref = m.refOf(item)
  const url = ref ? m.urlOf(item) : undefined
  return (
    <section className="quest-details space-y-3 rounded-lg border-2 border-[var(--panel-border)] bg-[var(--plate)] p-3">
      <div className="flex items-start justify-between gap-2">
        <div>
          <div className="quest-ref font-mono text-[12px] text-[var(--ink-faint)]">
            {item.key && (
              <>
                <Key id={item.key} />
                {' · '}
              </>
            )}
            {!ref ? (
              item.kind === 'wait' ? 'Petition' : 'Errand'
            ) : url ? (
              <a href={url} target="_blank" rel="noopener noreferrer" className="hover:text-[var(--ink)] hover:underline">
                {ref}
              </a>
            ) : (
              ref
            )}
          </div>
          <div className="text-[16px] font-semibold">{item.title}</div>
          {url && (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-2 inline-flex items-center gap-1.5 rounded-md border-2 border-[var(--ink)] bg-[var(--ink)] px-2.5 py-1 text-[13px] font-semibold text-[var(--bg)] hover:opacity-85"
            >
              Open on GitHub <ExternalLink size={14} />
            </a>
          )}
        </div>
        <button onClick={onClose} className="text-[var(--ink-faint)] hover:text-[var(--ink)]" aria-label="Close details">
          ✕
        </button>
      </div>
      <div className="flex items-center gap-3">
        <span className="text-[13px] font-bold tracking-wider uppercase" style={{ color: stateColour[status] }}>
          {words[status]}
        </span>
        <Hero name={item.assignee} />
      </div>
      {item.kind === 'wait' &&
        (status === 'awaiting' || status === 'locked' ? (
          <p className="rounded bg-[var(--await-plate)] p-2 text-[14px]">
            Awaiting reply from <b>{item.waitingOn}</b> since {item.since} ({m.daysSince(item)} days).
            {item.assignee && ` ${item.assignee} is chasing it.`}
          </p>
        ) : (
          <p className="rounded bg-[var(--chip)] p-2 text-[14px]">
            Petitioned <b>{item.waitingOn}</b> on {item.since}.
          </p>
        ))}
      {item.foundWhile && (
        <p className="rounded bg-[var(--chip)] p-2 text-[14px]">
          Unearthed while on{' '}
          {m.byId.has(item.foundWhile) ? (
            <button onClick={() => onPick(item.foundWhile!)} className="font-bold underline decoration-dotted underline-offset-2 hover:decoration-solid">
              {m.short(item.foundWhile)}
            </button>
          ) : (
            <b>{m.short(item.foundWhile)}</b>
          )}
          : {item.reason}
        </p>
      )}
      {!!item.alsoIn?.length && (
        <div>
          <div className="mb-1 text-[12px] font-bold tracking-wider text-[var(--ink-faint)] uppercase">Also in</div>
          <AlsoIn quests={item.alsoIn} label={false} />
        </div>
      )}
      {before.length > 0 && (
        <div>
          <div className="mb-1 text-[12px] font-bold tracking-wider text-[var(--ink-faint)] uppercase">Requires</div>
          <ul className="space-y-0.5 text-[14px]">{before.map((i) => <DeedRow key={i.id} m={m} i={i} onPick={onPick} />)}</ul>
        </div>
      )}
      <div>
        <div className="mb-1 text-[12px] font-bold tracking-wider text-[var(--ink-faint)] uppercase">Opens</div>
        <ul className="space-y-0.5 text-[14px]">
          {item.sideOf ? (
            <li>Nothing — optional polish on {m.short(item.sideOf)}.</li>
          ) : opens.length ? (
            opens.map((i) => <DeedRow key={i.id} m={m} i={i} onPick={onPick} />)
          ) : (
            <li className="font-semibold">The quest itself: this is its crowning deed.</li>
          )}
        </ul>
      </div>
    </section>
  )
}

// ---- page ------------------------------------------------------------------

/** The quest's title, renamed in place: Enter or leaving the field saves, Esc puts it back. */
function QuestTitle({ title, slug, onRetitle }: { title: string; slug?: string; onRetitle?: (title: string) => Promise<void> }) {
  const [draft, setDraft] = useState<string | null>(null) // null: not editing
  const [error, setError] = useState<string>()
  const saving = useRef(false)
  const save = async () => {
    if (draft === null || saving.current) return
    const t = draft.trim()
    if (t === title) return setDraft(null)
    if (!t) return setError('A quest needs a title')
    saving.current = true
    try {
      await onRetitle!(t)
      setDraft(null)
      setError(undefined)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      saving.current = false
    }
  }
  if (draft === null)
    return (
      <span className="group flex min-w-0 items-center gap-1">
        <span className="truncate" title={title}>
          {title}
        </span>
        {onRetitle && (
          <button
            onClick={() => {
              setDraft(title)
              setError(undefined)
            }}
            aria-label="Rename quest"
            title={`Rename quest (its slug${slug ? `, ${slug},` : ''} and links stay)`}
            className="shrink-0 rounded p-1 text-[var(--ink-faint)] opacity-0 group-hover:opacity-100 hover:text-[var(--ink)] focus:opacity-100"
          >
            <Pencil size={15} />
          </button>
        )}
      </span>
    )
  return (
    <span className="flex min-w-0 flex-1 items-center gap-2">
      <input
        autoFocus
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onFocus={(e) => e.target.select()}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void save()
          if (e.key === 'Escape') {
            setDraft(null)
            setError(undefined)
          }
        }}
        onBlur={() => void save()}
        aria-label="Quest title"
        aria-invalid={!!error}
        className="quest-display w-[min(40rem,100%)] min-w-0 rounded border-2 border-[var(--avail)] bg-[var(--plate)] px-2 py-0.5 text-xl font-semibold outline-none"
      />
      {error && <span className="shrink-0 font-sans text-[13px] font-normal text-[#e11d48]">{error}</span>}
    </span>
  )
}

export type QuestMapProps = {
  model: QuestModel
  state?: QuestState // from the API; the mock has none
  archived?: boolean
  slug?: string // the live chart's slug: turns on search (the mock has none)
  onRetitle?: (title: string) => Promise<void> // renames the quest; its slug stays. The mock has none
  onSetNpc?: (item: Item, npc: boolean) => Promise<void> // turns on the deed's right-click menu. The mock has none
  boardHref: string
  banner?: ReactNode // e.g. a GitHub warning, shown under the header
  version?: string // the running server's version, shown small under the side panel; the mock has none
  lineTypes?: typeof edgeTypes // other line designs to draw instead; only the mock tries them
}

/**
 * A deed's right-click menu, at the cursor and kept on screen. Esc, a click elsewhere or moving the
 * chart closes it; a failed save stays open with the reason.
 */
function DeedMenu({
  item,
  x,
  y,
  onSetNpc,
  onClose,
}: {
  item: Item
  x: number
  y: number
  onSetNpc: (item: Item, npc: boolean) => Promise<void>
  onClose: () => void
}) {
  const box = useRef<HTMLDivElement>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string>()
  useLayoutEffect(() => {
    const el = box.current
    if (!el) return
    const margin = 8
    el.style.left = `${Math.max(margin, Math.min(x, innerWidth - el.offsetWidth - margin))}px`
    el.style.top = `${Math.max(margin, Math.min(y, innerHeight - el.offsetHeight - margin))}px`
  }, [x, y, error])
  useEffect(() => {
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !box.current?.contains(e.target as Node)) onClose()
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', close)
    addEventListener('resize', onClose)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', close)
      removeEventListener('resize', onClose)
    }
  }, [onClose])
  const toggle = async () => {
    setSaving(true)
    try {
      await onSetNpc(item, !item.npc)
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setSaving(false)
    }
  }
  return (
    <div
      ref={box}
      role="menu"
      aria-label={`Deed ${item.key ?? ''}`}
      onContextMenu={(e) => e.preventDefault()}
      style={{ left: x, top: y }}
      className="fixed z-40 w-52 rounded-lg border border-[var(--panel-border)] bg-[var(--panel)] p-1 shadow-lg"
    >
      <div className="truncate px-3 pt-1 pb-1.5 text-[12px] text-[var(--ink-faint)]" title={titleOf(item)}>
        {item.key && <span className="font-mono font-semibold">{item.key}</span>} {titleOf(item)}
      </div>
      <button
        autoFocus
        role="menuitem"
        disabled={saving}
        onClick={() => void toggle()}
        className="flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-left text-[14px] text-[var(--ink)] hover:bg-[var(--chip)] disabled:opacity-60"
      >
        <span className={`size-2.5 shrink-0 rounded-full ${item.npc ? 'border-2 border-[var(--npc)]' : 'bg-[var(--npc)]'}`} />
        {saving ? 'Saving…' : item.npc ? 'No longer an NPC' : 'Mark as NPC'}
      </button>
      {error && <div className="px-3 py-1 text-[13px] text-[#e11d48]">{error}</div>}
    </div>
  )
}

// The side panel lives outside <ReactFlow>, so the provider wraps the whole page
// to let it move the map.
export default function QuestMap(props: QuestMapProps) {
  return (
    <ReactFlowProvider>
      <QuestMapInner {...props} />
    </ReactFlowProvider>
  )
}

type PanelTab = 'quest' | 'chronicle' | 'legend' | 'glossary'
const panelTabs: PanelTab[] = ['quest', 'chronicle', 'legend', 'glossary']

/** The side panel's tab, remembered per browser. */
function usePanelTab(): [PanelTab, (t: PanelTab) => void] {
  const [tab, setTab] = useState<PanelTab>(() => {
    try {
      const saved = localStorage.getItem('mikado.panel.tab')
      if (saved === 'log') return 'chronicle' // the tab's earlier name
      if (panelTabs.includes(saved as PanelTab)) return saved as PanelTab
    } catch {
      // storage may be unavailable; the default is fine
    }
    return 'quest'
  })
  const choose = useCallback((t: PanelTab) => {
    setTab(t)
    try {
      localStorage.setItem('mikado.panel.tab', t)
    } catch {
      // not remembering is fine
    }
  }, [])
  return [tab, choose]
}

function PanelTabs({ tab, onChange, logCount }: { tab: PanelTab; onChange: (t: PanelTab) => void; logCount: number }) {
  const tabs: [PanelTab, string][] = [
    ['quest', 'Quest'],
    ['chronicle', `Chronicle · ${logCount}`],
    ['legend', 'Legend'],
    ['glossary', 'Glossary'],
  ]
  return (
    <div role="tablist" className="quest-tabs flex shrink-0 gap-1 border-b border-[var(--panel-border)] px-3 pt-3">
      {tabs.map(([id, label]) => (
        <button
          key={id}
          role="tab"
          aria-selected={tab === id}
          onClick={() => onChange(id)}
          className={`quest-tab -mb-px rounded-t-md border px-3 py-1.5 text-[13px] font-semibold tracking-wide transition ${
            tab === id
              ? 'border-[var(--panel-border)] border-b-[var(--panel)] bg-[var(--panel)] text-[var(--ink)]'
              : 'border-transparent text-[var(--ink-soft)] hover:text-[var(--ink)]'
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  )
}

const showAll: Hide = { done: false, cancelled: false }
const GLIDE_MS = 300

/**
 * Which finished deeds the chart leaves off, remembered per quest (the mock has no slug).
 * `adjust` may change what was remembered for this visit only, without saving it.
 */
function useHide(slug: string | undefined, adjust: (h: Hide) => Hide): [Hide, (h: Hide) => void] {
  const key = `mikado.hide.${slug ?? 'mock'}`
  const [hide, setHide] = useState<Hide>(() => {
    try {
      const saved = JSON.parse(localStorage.getItem(key) ?? 'null') as Partial<Hide> | null
      return adjust({ done: !!saved?.done, cancelled: !!saved?.cancelled })
    } catch {
      // storage may be unavailable or hold something else; show everything
      return adjust(showAll)
    }
  })
  const choose = useCallback(
    (h: Hide) => {
      setHide(h)
      try {
        localStorage.setItem(key, JSON.stringify(h))
      } catch {
        // not remembering is fine
      }
    },
    [key],
  )
  return [hide, choose]
}

/** `hide` with whatever keeps deed `id` off the chart cleared: its own status, or that of the deed it hangs on. */
function revealing(m: QuestModel, id: string, hide: Hide): Hide {
  const i = m.byId.get(id)
  const parent = i?.sideOf ? m.byId.get(i.sideOf) : undefined
  const next = { ...hide }
  for (const x of [i, parent]) {
    if (!x) continue
    const s = m.statusOf(x)
    if (s === 'done') next.done = false
    if (s === 'cancelled') next.cancelled = false
  }
  return next
}

/** The chart's "Show" menu, beside its controls: leave fulfilled or abandoned deeds off the chart. */
function ShowMenu({
  hide,
  counts,
  onChange,
  onClose,
}: {
  hide: Hide
  counts: Record<keyof Hide, number>
  onChange: (h: Hide) => void
  onClose: () => void
}) {
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !box.current?.contains(e.target as Node)) onClose()
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', close)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', close)
    }
  }, [onClose])
  const row = (k: keyof Hide, label: string) => (
    <label className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-[14px] text-[var(--ink)] hover:bg-[var(--chip)]">
      <input type="checkbox" checked={hide[k]} onChange={() => onChange({ ...hide, [k]: !hide[k] })} className="accent-[var(--avail)]" />
      <span className="flex-1">{label}</span>
      <span className="text-[12px] text-[var(--ink-faint)]">{counts[k]}</span>
    </label>
  )
  return (
    <div
      ref={box}
      role="dialog"
      aria-label="Show"
      className="absolute bottom-[15px] left-[58px] z-20 w-56 rounded-lg border border-[var(--panel-border)] bg-[var(--panel)] p-1 shadow-lg"
    >
      <div className="px-2 pt-1 pb-1 text-[12px] font-bold tracking-wider text-[var(--ink-faint)] uppercase">Show</div>
      {row('done', 'Hide fulfilled')}
      {row('cancelled', 'Hide abandoned')}
      <div className="px-2 pt-1 pb-1.5 text-[12px] leading-snug text-[var(--ink-faint)]">The crowning deed always stays.</div>
    </div>
  )
}

/** The deed named by `?deed=` (or `?task=`, or the older `?card=`) — M142, m-142, c142 or 142 — if it is on this chart. */
function cardFromUrl(m: QuestModel): string | null {
  const q = new URLSearchParams(location.search)
  const raw = q.get('deed') ?? q.get('task') ?? q.get('card')
  if (!raw) return null
  const n = raw.trim().toLowerCase().replace(/^[mc]-?/, '')
  if (!/^\d+$/.test(n)) return null
  return [...m.items, ...m.sideQuests].find((i) => i.key?.toLowerCase() === `m${n}`)?.id ?? null
}

function QuestMapInner({ model: m, state, archived, slug, onRetitle, onSetNpc, boardHref, banner, version, lineTypes }: QuestMapProps) {
  const { goal, items, sideQuests, log } = m
  const [theme, setTheme] = useTheme()
  // `?deed=M142` (as `mikado open M142` links) opens the chart with that deed selected.
  const [wanted] = useState(() => cardFromUrl(m))
  const [selected, setSelected] = useState<string | null>(wanted)
  const [tab, setTab] = usePanelTab()
  const [panelOpen, setPanelOpen] = useState(true)
  // A deed the URL asks for is shown even if it is one the chart hides; what was saved stays.
  const [hide, setHide] = useHide(slug, (h) => (wanted ? revealing(m, wanted, h) : h))
  const [showOpen, setShowOpen] = useState(false)
  const closeShow = useCallback(() => setShowOpen(false), [])
  const [miniOpen, setMiniOpen] = useState(() => {
    try {
      return localStorage.getItem('mikado.mock.minimap') !== 'hidden'
    } catch {
      return true
    }
  })
  const toggleMini = () =>
    setMiniOpen((open) => {
      try {
        localStorage.setItem('mikado.mock.minimap', open ? 'hidden' : 'shown')
      } catch {
        // not remembering is fine
      }
      return !open
    })

  // A new width or font means new sizes: the map is mounted afresh, measured and laid out again.
  // Everything tied to one mount is tagged with this epoch so nothing from the last one leaks in.
  const epoch = `${panelOpen}-${theme}`
  const [placedAt, setPlacedAt] = useState<{ epoch: string; map: Placed } | null>(null)
  const positions = placedAt?.epoch === epoch ? placedAt.map : null
  const onPlaced = useCallback((map: Placed) => setPlacedAt({ epoch, map }), [epoch])

  // A deed that was selected and then left the quest is no longer selected.
  const sel = selected ? m.byId.get(selected) : undefined
  const selectedId = sel ? sel.id : null
  const hidden = useMemo(() => hiddenDeeds(m, hide), [m, hide])
  const graph = useMemo(() => buildGraph(m, selectedId, state, hidden), [m, selectedId, state, hidden])
  const hideable = useMemo(
    () => ({ done: hiddenDeeds(m, { done: true, cancelled: false }).size, cancelled: hiddenDeeds(m, { done: false, cancelled: true }).size }),
    [m],
  )

  // Nodes live in React Flow state so their measured sizes come back to us. New data only
  // restyles: each node keeps its measured size and position. A new card is kept out of
  // sight until the next layout has placed it.
  //
  // A card already on show that a new layout moves glides there rather than jumping. The glide
  // moves the nodes themselves, frame by frame, so the edges follow them exactly.
  const [nodes, setNodes, onNodesChange] = useNodesState<QuestNode>([])
  const [nodesEpoch, setNodesEpoch] = useState<string | null>(null)
  const lastEpoch = useRef<string | null>(null)
  const drawnAt = useRef<Placed>(new Map()) // where each card on show is drawn right now
  useEffect(() => {
    const fresh = lastEpoch.current !== epoch
    lastEpoch.current = epoch
    if (fresh) drawnAt.current = new Map()
    setNodesEpoch(epoch)
    // Nothing glides (or fades) on a mount's first layout, nor for someone who asked for less motion.
    const moving = drawnAt.current.size > 0 && !matchMedia('(prefers-reduced-motion: reduce)').matches
    const glides = new Map<string, { from: { x: number; y: number }; to: { x: number; y: number } }>()
    const appearing = new Set<string>()
    const drawn: Placed = new Map()
    for (const n of graph.nodes) {
      const from = drawnAt.current.get(n.id)
      const to = positions?.get(n.id)
      if (!to) continue
      if (moving && !from) appearing.add(n.id)
      if (moving && from && (from.x !== to.x || from.y !== to.y)) glides.set(n.id, { from, to })
      drawn.set(n.id, (moving && from) || to)
    }
    drawnAt.current = drawn
    setNodes((ns) => {
      const old = new Map((fresh ? [] : ns).map((n) => [n.id, n]))
      return graph.nodes.map((n) => {
        const o = old.get(n.id)
        const p = drawn.get(n.id)
        const style: CSSProperties | undefined = positions && !p ? { visibility: 'hidden' } : undefined
        const className = appearing.has(n.id) ? 'quest-appear' : o?.className
        return (o ? { ...o, data: n.data, position: p ?? o.position, style, className } : { ...n, position: p ?? n.position, style, className }) as QuestNode
      })
    })
    if (!glides.size) return
    const start = performance.now()
    let frame = requestAnimationFrame(function step(now) {
      const t = Math.min(1, (now - start) / GLIDE_MS)
      const k = 1 - (1 - t) ** 3 // ease out
      const at: Placed = new Map()
      for (const [id, { from, to }] of glides) {
        const p = { x: from.x + (to.x - from.x) * k, y: from.y + (to.y - from.y) * k }
        at.set(id, p)
        drawnAt.current.set(id, p)
      }
      setNodes((ns) => ns.map((n) => (at.has(n.id) ? { ...n, position: at.get(n.id)! } : n)))
      if (t < 1) frame = requestAnimationFrame(step)
    })
    // A newer layout (or new data) mid-glide starts its own from wherever the cards are now.
    return () => cancelAnimationFrame(frame)
  }, [epoch, graph, positions, setNodes])
  // Nodes kept from a previous mount carry that mount's sizes; the new one must measure its own.
  const flowNodes = nodesEpoch === epoch ? nodes : []
  const edges = useMemo(() => {
    const unplaced = (id: string) => !!positions && !positions.has(id)
    return graph.edges.map((e) => (unplaced(e.source) || unplaced(e.target) ? { ...e, hidden: true } : e))
  }, [graph, positions])

  const placed =
    !!positions &&
    flowNodes.length > 0 &&
    flowNodes.every((n) => {
      const p = positions.get(n.id)
      return !!p && p.x === n.position.x && p.y === n.position.y
    })
  // Once shown, a mount stays shown: a later re-layout moves cards in place instead of blanking the map.
  const [shownEpoch, setShownEpoch] = useState<string | null>(null)
  useEffect(() => {
    if (placed) setShownEpoch(epoch)
  }, [placed, epoch])
  const shown = placed || shownEpoch === epoch

  const statusOf = m.statusOf
  const count = (s: Status) => items.filter((i) => statusOf(i) === s).length
  const available = items.filter((i) => statusOf(i) === 'available')
  const awaiting = items.filter((i) => statusOf(i) === 'awaiting')
  const heroless = items.filter((i) => !i.done && !i.assignee).length
  const working = items.filter((i) => i.working && !i.done).length
  const openNow = useMemo(() => m.items.filter((i) => m.statusOf(i) === 'available' || (i.working && !i.done)).map((i) => i.id), [m])
  const frontier = useMemo(() => (wanted ? [wanted] : openNow), [openNow, wanted])

  // Picking a deed in the side panel selects it and flies the chart to it: to where the layout
  // puts it, even while it is still gliding there.
  const { getInternalNode, setCenter, setViewport, getViewport } = useReactFlow()
  const centre = (id: string, zoom?: number) => {
    const n = getInternalNode(id)
    if (!n) return
    const { x, y } = positions?.get(id) ?? n.internals.positionAbsolute
    setCenter(x + (n.measured.width ?? 0) / 2, y + (n.measured.height ?? 0) / 2, { zoom: zoom ?? getViewport().zoom, duration: 500 })
  }

  // The layout fits the chart once it has placed exactly the deeds on it; until then a deed just
  // shown is not placed and a hidden one still holds its old place.
  const laidOut = !!positions && positions.size === graph.nodes.length && graph.nodes.every((n) => positions.has(n.id))
  // What to look at once the chart is laid out again: after a toggle, the frontier; after a hidden
  // deed was asked for, that deed.
  const afterLayout = useRef<{ fit: true } | { deed: string } | null>(null)
  const chooseHide = (h: Hide) => {
    afterLayout.current = { fit: true }
    setHide(h)
  }
  const onLaidOut = useEffectEvent(() => {
    const next = afterLayout.current
    afterLayout.current = null
    if (!next || !positions) return
    if ('deed' in next) return centre(next.deed, 1)
    // Like the first view (FocusWhenPlaced), but onto where the cards are going, not where they are.
    const ids = openNow.filter((id) => positions.has(id))
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity
    for (const id of ids.length ? ids : ['goal']) {
      const p = positions.get(id)
      const size = getInternalNode(id)?.measured
      if (!p || !size) continue
      x0 = Math.min(x0, p.x)
      y0 = Math.min(y0, p.y)
      x1 = Math.max(x1, p.x + (size.width ?? 0))
      y1 = Math.max(y1, p.y + (size.height ?? 0))
    }
    const box = mapRef.current?.getBoundingClientRect()
    if (!box || x0 === Infinity) return
    setViewport(getViewportForBounds({ x: x0, y: y0, width: x1 - x0, height: y1 - y0 }, box.width, box.height, 0.8, 1, 0.35), { duration: 500 })
  })
  useEffect(() => {
    if (laidOut) onLaidOut()
  }, [laidOut, positions, graph])

  // Selecting a deed from anywhere shows its details and flies the chart to it. A deed the chart
  // hides is shown first, then flown to once the layout has placed it.
  const focus = (id: string) => {
    setSelected(id)
    setTab('quest')
    if (!hidden.has(id)) return centre(id, 1)
    afterLayout.current = { deed: id }
    setHide(revealing(m, id, hide))
  }
  // A deed clicked on the chart is selected; if it is partly out of view (say, at the
  // edge next to the panel), the chart slides it in, keeping the zoom.
  const mapRef = useRef<HTMLElement>(null)
  const reveal = (id: string) => {
    const n = getInternalNode(id)
    const box = mapRef.current?.getBoundingClientRect()
    if (!n || !box) return
    const { x, y, zoom } = getViewport()
    const left = n.internals.positionAbsolute.x * zoom + x
    const top = n.internals.positionAbsolute.y * zoom + y
    const right = left + (n.measured.width ?? 0) * zoom
    const bottom = top + (n.measured.height ?? 0) * zoom
    const margin = 24
    if (left < margin || top < margin || right > box.width - margin || bottom > box.height - margin) centre(id)
  }
  const dark = theme === 'midnight'
  // The deed right-clicked on the chart, and where; drawn from the current model so it follows saves.
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null)
  const menuItem = menu ? m.byId.get(menu.id) : undefined
  const closeMenu = useCallback(() => setMenu(null), [])

  return (
    <ThemeContext value={theme}>
      <div data-theme={theme} className="quest-theme flex h-screen flex-col">
        <header className="quest-header flex items-center gap-x-5 border-b border-[var(--panel-border)] bg-[var(--panel)] px-5 py-3">
          <div className="flex min-w-0 items-center gap-3">
            <span className="quest-emblem grid size-11 shrink-0 place-items-center rounded-full border-2" style={{ borderColor: 'var(--gold)', color: 'var(--gold)' }}>
              <Trophy size={20} />
            </span>
            <div className="min-w-0">
              <a href={boardHref} className="text-[12px] font-bold tracking-[0.2em] uppercase hover:underline" style={{ color: stateColour.done }}>
                ← Quest Board
              </a>
              <h1 className="quest-display flex min-w-0 items-center gap-2 text-xl font-semibold">
                <QuestTitle title={goal.title} slug={slug} onRetitle={onRetitle} />
                <QuestStateChip state={state} />
                {archived && <ArchivedChip />}
              </h1>
              <div className="truncate text-[14px] text-[var(--ink-soft)]">
                {goal.doneWhen ? (
                  <>
                    Crowned by{' '}
                    <button
                      onClick={() => focus(goal.doneWhen!)}
                      title={m.byId.get(goal.doneWhen)?.title}
                      className="font-mono font-semibold text-[var(--ink)] underline decoration-dotted underline-offset-2 hover:text-[var(--avail)]"
                    >
                      {logName(m, goal.doneWhen)}
                    </button>
                  </>
                ) : (
                  'No crowning deed yet'
                )}{' '}
                · {plural(items.filter(m.counted).length, 'deed')} · {plural(sideQuests.length, 'side quest')}
              </div>
            </div>
          </div>
          <div className="ml-auto flex shrink-0 items-center gap-1.5">
            <Pill n={count('done')} label="fulfilled" colour={stateColour.done} coin="gold" />
            <Pill n={count('available')} label="open" colour="var(--avail)" coin="enamel" />
            <Pill n={working} label="underway" colour="var(--avail)" coin="bronze" />
            <Pill n={count('awaiting')} label="awaiting reply" colour="var(--await)" coin="silver" />
            <Pill n={count('locked')} label="sealed" colour="var(--ink)" coin="wax" />
            {count('cancelled') > 0 && <Pill n={count('cancelled')} label="abandoned" colour="var(--ink-faint)" coin="iron" />}
            <Pill n={heroless} label="no hero" colour="#e11d48" coin="crimson" />
            <span className="ml-2" />
            {slug && (
              <Search
                compact
                here={slug}
                select={(key) => {
                  const id = `c${key.slice(1)}`
                  if (!m.byId.has(id)) return false
                  focus(id)
                  return true
                }}
              />
            )}
            <ThemeMenu theme={theme} onChange={setTheme} />
            <button
              onClick={() => setPanelOpen((o) => !o)}
              className="ml-1 grid size-10 place-items-center rounded-md border border-[var(--panel-border)] text-[var(--ink-soft)] hover:text-[var(--ink)]"
              aria-label={panelOpen ? 'Hide side panel' : 'Show side panel'}
              title={panelOpen ? 'Hide side panel' : 'Show side panel'}
            >
              {panelOpen ? <PanelRightClose size={18} /> : <PanelRightOpen size={18} />}
            </button>
          </div>
        </header>
        {banner}

        <div className="flex min-h-0 flex-1">
          <main ref={mapRef} className="relative min-w-0 flex-1">
            {/* The war table's campaign map: a still backdrop, not part of the chart, so it stays put as the chart pans and zooms. */}
            {theme === 'wartable' && <div className="wt-map wt-map-table" aria-hidden />}
            {!shown && <div className="absolute inset-0 z-10 grid place-items-center text-[var(--ink-soft)]">Drawing the chart…</div>}
            {/* Rendered before it is laid out (invisibly) so the deeds can be measured first. */}
            <div className="h-full" style={{ opacity: shown ? 1 : 0 }}>
              <ReactFlow
                key={epoch} // a new width or font means new sizes: measure and lay out again
                colorMode={dark ? 'dark' : 'light'}
                nodes={flowNodes}
                edges={edges}
                onNodesChange={onNodesChange}
                nodeTypes={nodeTypes}
                edgeTypes={lineTypes ?? edgeTypes}
                onNodeClick={(_, n) => {
                  if (n.id === 'goal' || n.id === selectedId) return setSelected(null)
                  setSelected(n.id)
                  setTab('quest')
                  reveal(n.id)
                }}
                onNodeDoubleClick={(_, n) => (n.id === 'goal' ? centre('goal', 1) : focus(n.id))}
                zoomOnDoubleClick={false}
                onPaneClick={() => setSelected(null)}
                // Without onSetNpc (the mock) a right-click is the browser's own.
                onNodeContextMenu={
                  onSetNpc &&
                  ((e, n) => {
                    if (n.type !== 'card') return
                    e.preventDefault()
                    setMenu({ id: n.id, x: e.clientX, y: e.clientY })
                  })
                }
                onMoveStart={closeMenu}
                nodesDraggable={false}
                nodesConnectable={false}
                minZoom={0.2}
                proOptions={{ hideAttribution: true }}
                style={{ background: 'transparent' }}
              >
                {theme !== 'wartable' && <Background gap={24} size={1.5} color="var(--dots)" bgColor="transparent" />}
                <Controls showInteractive={false}>
                  <ControlButton
                    onClick={toggleMini}
                    title={miniOpen ? 'Hide the overview' : 'Show the overview'}
                    aria-label={miniOpen ? 'Hide the overview' : 'Show the overview'}
                    aria-pressed={miniOpen}
                  >
                    <MapIcon size={14} style={{ opacity: miniOpen ? 1 : 0.45 }} />
                  </ControlButton>
                  <ControlButton
                    onClick={() => setShowOpen((o) => !o)}
                    // Else the menu's click-outside would close it, and this click open it again.
                    onMouseDown={(e) => e.stopPropagation()}
                    title="Show or hide finished deeds"
                    aria-label="Show or hide finished deeds"
                    aria-expanded={showOpen}
                  >
                    {hidden.size ? <EyeOff size={14} /> : <Eye size={14} />}
                  </ControlButton>
                </Controls>
                <LayoutWhenMeasured onPlaced={onPlaced} />
                <FocusWhenPlaced placed={placed} frontier={frontier} />
                {miniOpen && (
                  <MiniMap
                    pannable
                    zoomable
                    nodeClassName={miniClass}
                    nodeBorderRadius={6}
                    maskColor="color-mix(in srgb, var(--bg) 55%, transparent)"
                    style={{ background: 'var(--panel)', border: '1px solid var(--panel-border)', borderRadius: 8 }}
                  />
                )}
              </ReactFlow>
            </div>
            {showOpen && <ShowMenu hide={hide} counts={hideable} onChange={chooseHide} onClose={closeShow} />}
            {shown && hidden.size > 0 && (
              <div className="absolute top-3 left-3 z-10 flex items-center gap-1.5 rounded-full border border-[var(--panel-border)] bg-[var(--panel)] py-1 pr-1 pl-2.5 text-[13px] text-[var(--ink-soft)] shadow-sm">
                <EyeOff size={14} />
                <span>{hidden.size} hidden</span>
                <span className="text-[var(--ink-faint)]">·</span>
                <button
                  onClick={() => chooseHide(showAll)}
                  className="rounded-full px-1.5 font-semibold text-[var(--ink)] underline decoration-dotted underline-offset-2 hover:text-[var(--avail)]"
                >
                  show all
                </button>
              </div>
            )}
            {menuItem && onSetNpc && <DeedMenu key={menuItem.id} item={menuItem} x={menu!.x} y={menu!.y} onSetNpc={onSetNpc} onClose={closeMenu} />}
          </main>

          {panelOpen && (
            <aside className="quest-ledger relative z-10 flex w-[400px] shrink-0 flex-col border-l border-[var(--panel-border)] bg-[var(--panel)] shadow-[-8px_0_16px_-10px_rgba(0,0,0,0.35)]">
              <PanelTabs tab={tab} onChange={setTab} logCount={log.length} />
              <div className="min-h-0 flex-1 space-y-6 overflow-y-auto p-4">
              {tab === 'quest' && (
                <>
              {sel && <Details m={m} item={sel} onPick={focus} onClose={() => setSelected(null)} />}

              <Panel title="Open now">
                <ul className="quest-rows space-y-1.5">
                  {available.map((i) => (
                    <Row
                      key={i.id}
                      m={m}
                      item={i}
                      onPick={focus}
                      right={
                        <span className="flex shrink-0 items-center gap-2">
                          {i.working && <Working compact by={i.workingBy} />}
                          <Hero name={i.assignee} />
                        </span>
                      }
                    />
                  ))}
                </ul>
              </Panel>

              <Panel title="Awaiting reply">
                <ul className="quest-rows space-y-1.5">
                  {awaiting.map((i) => (
                    <Row
                      key={i.id}
                      m={m}
                      item={i}
                      onPick={focus}
                      right={
                        <span className="shrink-0 text-[12px] font-semibold text-[var(--await)]">
                          {m.daysSince(i)}d · {i.waitingOn}
                        </span>
                      }
                    />
                  ))}
                </ul>
              </Panel>

              <Panel title="Side quests — achievements">
                <ul className="quest-rows space-y-1.5">
                  {sideQuests.map((q) => (
                    <Row key={q.id} m={m} item={q} onPick={focus} right={<Hero name={q.assignee} />} />
                  ))}
                </ul>
              </Panel>

                </>
              )}

              {tab === 'legend' && (theme === 'wartable' ? <WarLegend /> : <Legend />)}

              {tab === 'glossary' && <Glossary />}

              {tab === 'chronicle' && (
                <ol className="quest-chronicle space-y-2 border-l-2 border-[var(--panel-border)] pl-3">
                  {log.map((e, k) => (
                    // A new day is marked so the war table can set a little space above it; the other themes ignore it.
                    <li key={k} data-newday={k > 0 && e.at !== log[k - 1].at ? '' : undefined} className="text-[14px] leading-snug">
                      <span className="mr-1 inline-block w-3 font-bold" style={{ color: logColour[e.kind] ?? 'var(--ink-faint)' }}>
                        {logIcon[e.kind] ?? '·'}
                      </span>
                      <span className="text-[var(--ink-faint)]">{e.at}</span>{' '}
                      {e.id && (
                        <button
                          onClick={() => focus(e.id!)}
                          className={`quest-ref font-mono text-[13px] underline decoration-dotted underline-offset-2 hover:text-[var(--avail)] ${
                            e.kind === 'remove' ? 'line-through' : ''
                          }`}
                        >
                          {logName(m, e.id)}
                        </button>
                      )}{' '}
                      <span className="text-[var(--ink-soft)]">{e.text}</span>
                    </li>
                  ))}
                </ol>
              )}
              </div>
              {version && <VersionLine version={version} className="shrink-0 px-4 py-1.5" />}
            </aside>
          )}
        </div>
      </div>
    </ThemeContext>
  )
}
