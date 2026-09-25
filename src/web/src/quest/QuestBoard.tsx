import type { CSSProperties, ReactNode } from 'react'
import { Ban, Check, ChevronRight, Hourglass, Sparkles, Trophy } from 'lucide-react'
import './quest.css'
import type { QuestState } from './model'
import { QuestStateChip } from './look'
import { Search } from './Search'
import { ThemeMenu, useTheme } from './theme'
import { VersionLine } from './VersionLine'

// One quest on the Quest Board, drawn as a row. `state`, `cancelled` and `inProgress` come from the API; the mock has none.
export type BoardQuest = {
  slug: string
  title: string
  main: { done: number; total: number }
  achievements: { done: number; total: number } // side quests
  available: number
  awaiting: number
  heroes: string[]
  repos: string[]
  lastActivity: string
  state?: QuestState
  cancelled?: number
  inProgress?: number
  archivedAt?: string // set while archived: kept off the shelves above
  blockedBy?: QuestLink[] // quests drawn on this one's chart as quest cards
  blocks?: QuestLink[] // quests with this one on their chart
}

/** Another quest, named on a board row. */
export type QuestLink = { slug: string; title: string; state: QuestState }

const cancelled = (q: BoardQuest) => q.state === 'cancelled'
// A quest that says what state it is in is believed; otherwise its main-quest count decides.
const mainDone = (q: BoardQuest) => (q.state ? q.state === 'complete' : q.main.done === q.main.total)
const perfect = (q: BoardQuest) => mainDone(q) && q.achievements.done === q.achievements.total

function Heroes({ names }: { names: string[] }) {
  return (
    <span className="flex -space-x-1.5">
      {names.map((n) => (
        <span
          key={n}
          title={n}
          className="grid size-7 place-items-center rounded-full bg-[var(--ink)] text-[12px] font-bold text-[var(--bg)] uppercase ring-2 ring-[var(--plate)]"
        >
          {n[0]}
        </span>
      ))}
    </span>
  )
}

// The row's columns, shared by every row so the shelves line up. Narrow screens wrap instead.
const columns = 'md:grid md:grid-cols-[auto_minmax(0,1fr)_170px_90px_170px_96px]'

/** One linked quest in a row's "Blocked by" / "Blocks" line: a finished one steps back. */
function LinkedQuest({ q }: { q: QuestLink }) {
  const over = q.state !== 'active'
  return (
    <a
      href={`/quest/${encodeURIComponent(q.slug)}`}
      title={`${q.title} (${q.slug})${q.state === 'complete' ? ' — fulfilled' : q.state === 'cancelled' ? ' — abandoned' : ''}`}
      className={`relative z-10 hover:text-[var(--ink)] hover:underline ${
        over ? 'text-[var(--ink-faint)]' : 'font-semibold text-[var(--ink)]'
      } ${q.state === 'cancelled' ? 'line-through' : ''}`}
    >
      {q.state === 'complete' && <Check size={12} strokeWidth={3} className="mr-0.5 inline align-[-1px]" />}
      {q.title}
    </a>
  )
}

/** Which quests this one waits on, and which wait on it: those drawn as quest cards on a chart. */
function Linked({ label, quests }: { label: string; quests?: QuestLink[] }) {
  if (!quests?.length) return null
  return (
    <span>
      {label}:{' '}
      {quests.map((l, k) => (
        <span key={l.slug}>
          {k > 0 && ', '}
          <LinkedQuest q={l} />
        </span>
      ))}
    </span>
  )
}

function QuestRow({ q, href, noMap }: { q: BoardQuest; href?: string; noMap?: string }) {
  const pct = q.main.total ? Math.round((q.main.done / q.main.total) * 100) : 0
  const done = mainDone(q)
  const accent = cancelled(q) ? 'var(--edge-off)' : perfect(q) ? 'var(--side)' : done ? 'var(--gold)' : 'var(--plate-border)'
  const medal: CSSProperties = cancelled(q)
    ? { background: 'var(--panel)', borderColor: 'var(--edge-off)', color: 'var(--ink-faint)' }
    : perfect(q)
      ? { background: 'var(--side)', borderColor: 'var(--side)', color: '#fff' }
      : done
        ? { background: 'var(--gold)', borderColor: 'var(--gold)', color: 'var(--gold-ink)' }
        : { borderColor: 'var(--gold)', color: 'var(--gold)' }

  const body = (
    <div
      style={{ borderLeftColor: accent, opacity: cancelled(q) || q.archivedAt ? 0.8 : undefined }}
      className={`relative flex flex-wrap items-center gap-x-4 gap-y-2 border-l-4 bg-[var(--plate)] px-4 py-2.5 ${columns} ${
        href ? 'transition-colors hover:bg-[var(--panel)]' : ''
      }`}
    >
      <span style={medal} className="grid size-9 shrink-0 place-items-center rounded-full border-2">
        {cancelled(q) ? <Ban size={17} /> : <Trophy size={17} />}
      </span>
      {/* On a narrow screen the title takes the medal's line; the rest wraps below it. */}
      <div className="min-w-0 basis-[calc(100%-3.25rem)] md:basis-auto">
        <div className="flex items-center gap-2">
          {/* The title is the row's link, stretched over the whole row; the quest links below sit above it. */}
          {href ? (
            <a
              href={href}
              className={`quest-display truncate text-[16px] leading-snug font-semibold after:absolute after:inset-0 after:content-[''] ${
                cancelled(q) ? 'line-through decoration-1' : ''
              }`}
              title={q.title}
            >
              {q.title}
            </a>
          ) : (
            <span
              className={`quest-display truncate text-[16px] leading-snug font-semibold ${cancelled(q) ? 'line-through decoration-1' : ''}`}
              title={q.title}
            >
              {q.title}
            </span>
          )}
          <QuestStateChip state={q.state} />
        </div>
        <div className="truncate text-[13px] text-[var(--ink-soft)]">
          {[...q.repos, `last move ${q.lastActivity}`].join(' · ')}
        </div>
        {(!!q.blockedBy?.length || !!q.blocks?.length) && (
          <div className="flex gap-3 truncate text-[13px] text-[var(--ink-soft)]">
            <Linked label="Blocked by" quests={q.blockedBy} />
            <Linked label="Blocks" quests={q.blocks} />
          </div>
        )}
      </div>

      <div className="flex min-w-[150px] flex-1 items-center gap-2 md:w-[170px] md:flex-none" title="Main quest: deeds fulfilled">
        <div className="h-2 flex-1 overflow-hidden rounded-full bg-[var(--chip)] ring-1 ring-[var(--plate-border)]">
          <div className="h-full bg-[var(--gold)]" style={{ width: `${pct}%` }} />
        </div>
        <span className="w-12 text-right text-[14px] font-semibold tabular-nums">
          {q.main.done}/{q.main.total}
        </span>
      </div>

      <span
        className="text-[14px] whitespace-nowrap text-[var(--ink-soft)] tabular-nums"
        title="Achievements: side quests fulfilled"
      >
        <span style={{ color: q.achievements.done > 0 ? 'var(--side)' : 'var(--edge-off)' }}>★</span> {q.achievements.done}/
        {q.achievements.total}
      </span>

      <div className="flex flex-wrap gap-x-3 text-[14px] font-medium">
        {!!q.inProgress && (
          <span className="flex items-center gap-1.5 text-[var(--avail)]" title="underway">
            <span className="quest-working-dot size-2 rounded-full bg-[var(--avail)]" /> {q.inProgress}
          </span>
        )}
        {q.available > 0 && (
          <span className="flex items-center gap-1 text-[var(--avail)]" title="open">
            <Sparkles size={15} /> {q.available}
          </span>
        )}
        {q.awaiting > 0 && (
          <span className="flex items-center gap-1 text-[var(--await)]" title="awaiting reply">
            <Hourglass size={15} /> {q.awaiting}
          </span>
        )}
        {!!q.cancelled && (
          <span className="flex items-center gap-1 text-[var(--ink-faint)]" title="abandoned">
            <Ban size={15} /> {q.cancelled}
          </span>
        )}
        {!href && noMap && <span className="font-normal text-[var(--ink-faint)]">{noMap}</span>}
      </div>

      <span className="flex justify-end">
        <Heroes names={q.heroes} />
      </span>
    </div>
  )
  return body
}

/** A shelf's rows, one list with hairlines between them. */
function Rows({ children }: { children: ReactNode }) {
  return (
    <div className="quest-plate divide-y divide-[var(--plate-border)] overflow-hidden rounded-xl border-2 border-[var(--plate-border)]">
      {children}
    </div>
  )
}

function Section({ title, hint, children }: { title: string; hint: string; children: ReactNode[] }) {
  return (
    <section>
      <div className="mb-3 flex items-baseline gap-3">
        <h2 className="quest-display text-[13px] font-bold tracking-[0.2em] uppercase">{title}</h2>
        <span className="text-[14px] text-[var(--ink-soft)]">{hint}</span>
      </div>
      {children.length > 0 && <Rows>{children}</Rows>}
    </section>
  )
}

export type QuestBoardProps = {
  quests: BoardQuest[]
  hrefOf: (q: BoardQuest) => string | undefined // undefined: the quest has no chart to open
  eyebrow: string // the small line above the title
  noMap?: string // what a quest without a chart says about it
  banner?: ReactNode // e.g. a GitHub warning, shown under the header
  empty?: ReactNode // shown instead of the shelves when there are no quests at all
  search?: boolean // the live board searches the API; the mock has none
  version?: string // the running server's version, shown small at the bottom; the mock has none
}

export default function QuestBoard({ quests, hrefOf, eyebrow, noMap, banner, empty, search, version }: QuestBoardProps) {
  const [theme, setTheme] = useTheme()
  const shelved = quests.filter((q) => !q.archivedAt)
  const archived = quests.filter((q) => q.archivedAt)
  const active = shelved.filter((q) => !cancelled(q) && !mainDone(q))
  const bonus = shelved.filter((q) => !cancelled(q) && mainDone(q) && !perfect(q))
  const hundred = shelved.filter((q) => !cancelled(q) && perfect(q))
  const gone = shelved.filter(cancelled)
  const sum = (f: (q: BoardQuest) => number) => active.reduce((n, q) => n + f(q), 0)
  const row = (q: BoardQuest) => <QuestRow key={q.slug} q={q} href={hrefOf(q)} noMap={noMap} />

  return (
    <div data-theme={theme} className="quest-theme min-h-screen">
      <header className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-[var(--panel-border)] bg-[var(--panel)] px-6 py-4">
        <div>
          <div className="text-[12px] font-bold tracking-[0.2em] text-[var(--ink-soft)] uppercase">{eyebrow}</div>
          <h1 className="quest-display text-2xl font-semibold">Quest Board</h1>
        </div>
        <div className="flex gap-2">
          {search && <Search />}
          <ThemeMenu theme={theme} onChange={setTheme} />
        </div>
        <div className="ml-auto flex gap-2 text-center">
          {(
            [
              [active.length, 'active quests', 'var(--gold)'],
              [sum((q) => q.available), 'open now', 'var(--avail)'],
              [sum((q) => q.awaiting), 'awaiting reply', 'var(--await)'],
            ] as const
          ).map(([n, label, colour]) => (
            <div key={label} className="rounded-md border border-[var(--panel-border)] bg-[var(--plate)] px-3 py-1.5">
              <div className="text-xl leading-none font-bold" style={{ color: colour }}>
                {n}
              </div>
              <div className="text-[11px] font-semibold tracking-wider text-[var(--ink-soft)] uppercase">{label}</div>
            </div>
          ))}
        </div>
      </header>
      {banner}

      {quests.length === 0 && empty ? (
        <main className="mx-auto max-w-7xl p-6">{empty}</main>
      ) : (
        <main className="mx-auto max-w-7xl space-y-8 p-6">
          <Section title="Underway" hint="the main quest still has deeds to fulfil">
            {active.map(row)}
          </Section>
          <Section title="Main quest fulfilled" hint="finished — achievements still there if you're into it">
            {bonus.map(row)}
          </Section>
          <Section title="100%" hint="main quest and every achievement">
            {hundred.map(row)}
          </Section>
          {gone.length > 0 && (
            <Section title="Abandoned" hint="the crowning deed won't be done, so neither will the quest">
              {gone.map(row)}
            </Section>
          )}
          {archived.length > 0 && (
            // Closed by default; the browser keeps it open across the 15 s refresh.
            <details className="group">
              <summary className="mb-3 flex cursor-pointer list-none items-baseline gap-3 select-none">
                <h2 className="quest-display text-[13px] font-bold tracking-[0.2em] whitespace-nowrap uppercase">
                  <ChevronRight size={14} className="mr-1 inline align-[-2px] transition-transform group-open:rotate-90" />
                  Archived ({archived.length})
                </h2>
                <span className="text-[14px] text-[var(--ink-soft)]">
                  put away with <span className="font-mono">mikado quest archive</span> — charts still open
                </span>
              </summary>
              <Rows>{archived.map(row)}</Rows>
            </details>
          )}
        </main>
      )}
      {version && <VersionLine version={version} className="mx-auto max-w-7xl px-6 pb-4" />}
    </div>
  )
}
