import type { CSSProperties, ReactNode } from 'react'
import { Ban, Hourglass, Sparkles, Trophy } from 'lucide-react'
import './quest.css'
import type { QuestState } from './model'
import { Achievements, QuestStateChip } from './look'
import { ThemeMenu, useTheme } from './theme'

// One quest on the Quest Board. `state`, `cancelled` and `inProgress` come from the API; the mock has none.
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
}

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

function QuestCard({ q, href, noMap }: { q: BoardQuest; href?: string; noMap?: string }) {
  const pct = q.main.total ? Math.round((q.main.done / q.main.total) * 100) : 0
  const done = mainDone(q)
  const card: CSSProperties = cancelled(q)
    ? { borderColor: 'var(--edge-off)', background: 'var(--panel)', opacity: 0.8 }
    : perfect(q)
      ? { borderColor: 'var(--side)', background: 'var(--plate)' }
      : done
        ? { borderColor: 'var(--gold)', background: 'var(--done-plate)' }
        : { borderColor: 'var(--plate-border)', background: 'var(--plate)' }
  const medal: CSSProperties = cancelled(q)
    ? { background: 'var(--panel)', borderColor: 'var(--edge-off)', color: 'var(--ink-faint)' }
    : perfect(q)
      ? { background: 'var(--side)', borderColor: 'var(--side)', color: '#fff' }
      : done
        ? { background: 'var(--gold)', borderColor: 'var(--gold)', color: 'var(--gold-ink)' }
        : { borderColor: 'var(--gold)', color: 'var(--gold)' }

  const body = (
    <div
      style={card}
      className={`quest-plate flex h-full flex-col gap-3 rounded-xl border-2 p-4 transition ${
        href ? 'hover:-translate-y-0.5 hover:shadow-lg' : ''
      }`}
    >
      <div className="flex items-start gap-3">
        <span style={medal} className="grid size-12 shrink-0 place-items-center rounded-full border-2">
          {cancelled(q) ? <Ban size={22} /> : <Trophy size={22} />}
        </span>
        <div className="min-w-0 flex-1">
          <div className={`quest-display text-[17px] leading-snug font-semibold ${cancelled(q) ? 'line-through decoration-1' : ''}`}>
            {q.title}
          </div>
          <div className="mt-0.5 text-[13px] text-[var(--ink-soft)]">
            {[...q.repos, `last move ${q.lastActivity}`].join(' · ')}
          </div>
        </div>
        <QuestStateChip state={q.state} />
      </div>

      <div>
        <div className="mb-1 flex justify-between text-[14px] text-[var(--ink-soft)]">
          <span>Main quest</span>
          <span className="font-semibold text-[var(--ink)]">
            {q.main.done} / {q.main.total}
          </span>
        </div>
        <div className="h-2.5 overflow-hidden rounded-full bg-[var(--chip)] ring-1 ring-[var(--plate-border)]">
          <div className="h-full bg-[var(--gold)]" style={{ width: `${pct}%` }} />
        </div>
      </div>

      <Achievements done={q.achievements.done} total={q.achievements.total} />

      <div className="mt-auto flex items-center justify-between gap-2">
        <div className="flex flex-wrap gap-x-3 text-[14px] font-medium">
          {!!q.inProgress && (
            <span className="flex items-center gap-1.5 text-[var(--avail)]">
              <span className="quest-working-dot size-2 rounded-full bg-[var(--avail)]" /> {q.inProgress} underway
            </span>
          )}
          {q.available > 0 && (
            <span className="flex items-center gap-1 text-[var(--avail)]">
              <Sparkles size={15} /> {q.available} open
            </span>
          )}
          {q.awaiting > 0 && (
            <span className="flex items-center gap-1 text-[var(--await)]">
              <Hourglass size={15} /> {q.awaiting} awaiting reply
            </span>
          )}
          {!!q.cancelled && (
            <span className="flex items-center gap-1 text-[var(--ink-faint)]">
              <Ban size={15} /> {q.cancelled} abandoned
            </span>
          )}
          {!href && noMap && <span className="font-normal text-[var(--ink-faint)]">{noMap}</span>}
        </div>
        <Heroes names={q.heroes} />
      </div>
    </div>
  )
  return href ? (
    <a href={href} className="block">
      {body}
    </a>
  ) : (
    body
  )
}

function Section({ title, hint, children }: { title: string; hint: string; children: ReactNode }) {
  return (
    <section>
      <div className="mb-3 flex items-baseline gap-3">
        <h2 className="quest-display text-[13px] font-bold tracking-[0.2em] uppercase">{title}</h2>
        <span className="text-[14px] text-[var(--ink-soft)]">{hint}</span>
      </div>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(360px,1fr))] gap-4">{children}</div>
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
}

export default function QuestBoard({ quests, hrefOf, eyebrow, noMap, banner, empty }: QuestBoardProps) {
  const [theme, setTheme] = useTheme()
  const active = quests.filter((q) => !cancelled(q) && !mainDone(q))
  const bonus = quests.filter((q) => !cancelled(q) && mainDone(q) && !perfect(q))
  const hundred = quests.filter((q) => !cancelled(q) && perfect(q))
  const gone = quests.filter(cancelled)
  const sum = (f: (q: BoardQuest) => number) => active.reduce((n, q) => n + f(q), 0)
  const card = (q: BoardQuest) => <QuestCard key={q.slug} q={q} href={hrefOf(q)} noMap={noMap} />

  return (
    <div data-theme={theme} className="quest-theme min-h-screen">
      <header className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-[var(--panel-border)] bg-[var(--panel)] px-6 py-4">
        <div>
          <div className="text-[12px] font-bold tracking-[0.2em] text-[var(--ink-soft)] uppercase">{eyebrow}</div>
          <h1 className="quest-display text-2xl font-semibold">Quest Board</h1>
        </div>
        <ThemeMenu theme={theme} onChange={setTheme} />
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
            {active.map(card)}
          </Section>
          <Section title="Main quest fulfilled" hint="finished — achievements still there if you're into it">
            {bonus.map(card)}
          </Section>
          <Section title="100%" hint="main quest and every achievement">
            {hundred.map(card)}
          </Section>
          {gone.length > 0 && (
            <Section title="Abandoned" hint="the crowning deed won't be done, so neither will the quest">
              {gone.map(card)}
            </Section>
          )}
        </main>
      )}
    </div>
  )
}
