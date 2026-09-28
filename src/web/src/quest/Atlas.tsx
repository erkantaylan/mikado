import type { CSSProperties, ReactNode } from 'react'
import { Ban, Check, ChevronRight, Compass, Hourglass, Sparkles, Trophy } from 'lucide-react'
import './quest.css'
import type { JourneyState } from './model'
import { JourneyStateChip, RegionChip, Renamable, Tally } from './look'
import { Search } from './Search'
import { ThemeMenu, useTheme } from './theme'
import { VersionLine } from './VersionLine'

// One journey on the Atlas, drawn as a row. `state`, `cancelled` and `inProgress` come from the API; the mock has none.
export type AtlasJourney = {
  key: string // J7
  title: string
  main: { done: number; total: number }
  achievements: { done: number; total: number } // side quests
  available: number
  awaiting: number
  heroes: string[]
  repos: string[]
  lastActivity: string
  state?: JourneyState
  cancelled?: number
  inProgress?: number
  archivedAt?: string // set while archived: kept off the shelves above
  blockedBy?: JourneyLink[] // journeys drawn on this one's chart as journey cards
  blocks?: JourneyLink[] // journeys with this one on their chart
  region?: { key: string; name: string } // R2; the mock has none
}

/** Another journey, named on an atlas row. */
export type JourneyLink = { key: string; title: string; state: JourneyState }

const cancelled = (q: AtlasJourney) => q.state === 'cancelled'
// A journey that says what state it is in is believed; otherwise its main-quest count decides.
const mainDone = (q: AtlasJourney) => (q.state ? q.state === 'complete' : q.main.done === q.main.total)
const perfect = (q: AtlasJourney) => mainDone(q) && q.achievements.done === q.achievements.total

// The row's columns, shared by every row so the shelves line up. Narrow screens wrap instead.
const columns = 'md:grid md:grid-cols-[auto_minmax(0,1fr)_170px_90px_170px]'

/** One linked journey in a row's "Blocked by" / "Blocks" line: a finished one steps back. */
function LinkedJourney({ q }: { q: JourneyLink }) {
  const over = q.state !== 'active'
  return (
    <a
      href={`/journey/${encodeURIComponent(q.key)}`}
      title={`${q.title} (${q.key})${q.state === 'complete' ? ' — fulfilled' : q.state === 'cancelled' ? ' — abandoned' : ''}`}
      className={`relative z-10 hover:text-[var(--ink)] hover:underline ${
        over ? 'text-[var(--ink-faint)]' : 'font-semibold text-[var(--ink)]'
      } ${q.state === 'cancelled' ? 'line-through' : ''}`}
    >
      {q.state === 'complete' && <Check size={12} strokeWidth={3} className="mr-0.5 inline align-[-1px]" />}
      {q.title}
    </a>
  )
}

/** Which journeys this one waits on, and which wait on it: those drawn as journey cards on a chart. */
function Linked({ label, journeys }: { label: string; journeys?: JourneyLink[] }) {
  if (!journeys?.length) return null
  return (
    <span>
      {label}:{' '}
      {journeys.map((l, k) => (
        <span key={l.key}>
          {k > 0 && ', '}
          <LinkedJourney q={l} />
        </span>
      ))}
    </span>
  )
}

// `chip`: say fulfilled or abandoned on the row; off on a shelf whose heading already says it.
function JourneyRow({ q, href, noMap, chip }: { q: AtlasJourney; href?: string; noMap?: string; chip: boolean }) {
  const pct = q.main.total ? Math.round((q.main.done / q.main.total) * 100) : 0
  const done = mainDone(q)
  const medal: CSSProperties = cancelled(q)
    ? { background: 'var(--panel)', borderColor: 'var(--edge-off)', color: 'var(--ink-faint)' }
    : perfect(q)
      ? { background: 'var(--side)', borderColor: 'var(--side)', color: '#fff' }
      : done
        ? { background: 'var(--gold)', borderColor: 'var(--gold)', color: 'var(--gold-ink)' }
        : { borderColor: 'var(--gold)', color: 'var(--gold)' }

  const body = (
    <div
      style={{ opacity: cancelled(q) || q.archivedAt ? 0.8 : undefined }}
      className={`relative flex flex-wrap items-center gap-x-4 gap-y-2 bg-[var(--plate)] px-4 py-2.5 ${columns} ${
        href ? 'transition-colors hover:bg-[var(--panel)]' : ''
      }`}
    >
      <span style={medal} className="grid size-9 shrink-0 place-items-center rounded-full border-2">
        {cancelled(q) ? <Ban size={17} /> : <Trophy size={17} />}
      </span>
      {/* On a narrow screen the title takes the medal's line; the rest wraps below it. */}
      <div className="min-w-0 basis-[calc(100%-3.25rem)] md:basis-auto">
        <div className="flex items-center gap-2">
          {/* The title is the row's link, stretched over the whole row; the journey links below sit above it. */}
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
          {chip && <JourneyStateChip state={q.state} />}
        </div>
        <div className="truncate text-[13px] text-[var(--ink-soft)]">
          <span className="font-mono">{q.key}</span>
          {[...q.repos, `last move ${q.lastActivity}`].map((part) => ` · ${part}`)}
        </div>
        {(!!q.blockedBy?.length || !!q.blocks?.length) && (
          <div className="flex gap-3 truncate text-[13px] text-[var(--ink-soft)]">
            <Linked label="Blocked by" journeys={q.blockedBy} />
            <Linked label="Blocks" journeys={q.blocks} />
          </div>
        )}
      </div>

      <div className="flex min-w-[150px] flex-1 items-center gap-2 md:w-[170px] md:flex-none" title="Main quest: quests fulfilled">
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
      <div className="quest-shelf-title mb-3 flex items-baseline gap-3">
        <h2 className="quest-display text-[13px] font-bold tracking-[0.2em] uppercase">{title}</h2>
        <span className="text-[14px] text-[var(--ink-soft)]">{hint}</span>
      </div>
      {children.length > 0 && <Rows>{children}</Rows>}
    </section>
  )
}

/** A set of journeys on their shelves: underway, main quest fulfilled, 100%, abandoned. An empty shelf is left out. */
function Shelves({ journeys, row }: { journeys: AtlasJourney[]; row: (q: AtlasJourney) => ReactNode }) {
  const active = journeys.filter((q) => !cancelled(q) && !mainDone(q))
  const bonus = journeys.filter((q) => !cancelled(q) && mainDone(q) && !perfect(q))
  const hundred = journeys.filter((q) => !cancelled(q) && perfect(q))
  const gone = journeys.filter(cancelled)
  return (
    <>
      {active.length > 0 && (
        <Section title="Journeys underway" hint="the main quest still has quests to fulfil">
          {active.map(row)}
        </Section>
      )}
      {bonus.length > 0 && (
        <Section title="Main quest fulfilled" hint="finished — achievements still there if you're into it">
          {bonus.map(row)}
        </Section>
      )}
      {hundred.length > 0 && (
        <Section title="100%" hint="main quest and every achievement">
          {hundred.map(row)}
        </Section>
      )}
      {gone.length > 0 && (
        <Section title="Abandoned" hint="the crowning quest won't be done, so neither will the journey">
          {gone.map(row)}
        </Section>
      )}
    </>
  )
}

/** A region on the Atlas's home page (store.Region with its journeys). */
export type AtlasRegion = { key: string; name: string; journeys: AtlasJourney[] }

/** One region as a row on the Atlas's home page: its journeys, and how far the main quests underway are. */
function RegionRow({ region, href }: { region: AtlasRegion; href: string }) {
  const shelved = region.journeys.filter((q) => !q.archivedAt)
  const underway = shelved.filter((q) => !cancelled(q) && !mainDone(q))
  const finished = shelved.filter((q) => !cancelled(q) && mainDone(q))
  const done = underway.reduce((n, q) => n + q.main.done, 0)
  const total = underway.reduce((n, q) => n + q.main.total, 0)
  const pct = total ? Math.round((done / total) * 100) : 0
  const open = underway.reduce((n, q) => n + q.available, 0)
  const awaiting = underway.reduce((n, q) => n + q.awaiting, 0)
  const working = underway.reduce((n, q) => n + (q.inProgress ?? 0), 0)
  const archived = region.journeys.length - shelved.length
  return (
    <div className={`relative flex flex-wrap items-center gap-x-4 gap-y-2 bg-[var(--plate)] px-4 py-2.5 transition-colors hover:bg-[var(--panel)] ${columns}`}>
      <span className="grid size-9 shrink-0 place-items-center rounded-full border-2" style={{ borderColor: 'var(--gold)', color: 'var(--gold)' }}>
        <Compass size={17} />
      </span>
      <div className="min-w-0 basis-[calc(100%-3.25rem)] md:basis-auto">
        {/* The name is the row's link, stretched over the whole row. */}
        <a href={href} className="quest-display block truncate text-[16px] leading-snug font-semibold after:absolute after:inset-0 after:content-['']" title={region.name}>
          {region.name}
        </a>
        <div className="truncate text-[13px] text-[var(--ink-soft)]">
          <span className="font-mono">{region.key}</span>
          {(region.journeys.length === 0
            ? ['no journeys yet']
            : [
                `${underway.length} underway`,
                finished.length > 0 && `${finished.length} fulfilled`,
                archived > 0 && `${archived} archived`,
              ].filter(Boolean)
          ).map((part) => ` · ${part}`)}
        </div>
      </div>
      <div className="flex min-w-[150px] flex-1 items-center gap-2 md:w-[170px] md:flex-none" title="Main quests of the journeys underway: quests fulfilled">
        <div className="h-2 flex-1 overflow-hidden rounded-full bg-[var(--chip)] ring-1 ring-[var(--plate-border)]">
          <div className="h-full bg-[var(--gold)]" style={{ width: `${pct}%` }} />
        </div>
        <span className="w-12 text-right text-[14px] font-semibold tabular-nums">
          {done}/{total}
        </span>
      </div>
      <span />
      <div className="flex flex-wrap gap-x-3 text-[14px] font-medium">
        {working > 0 && (
          <span className="flex items-center gap-1.5 text-[var(--avail)]" title="underway">
            <span className="quest-working-dot size-2 rounded-full bg-[var(--avail)]" /> {working}
          </span>
        )}
        {open > 0 && (
          <span className="flex items-center gap-1 text-[var(--avail)]" title="open">
            <Sparkles size={15} /> {open}
          </span>
        )}
        {awaiting > 0 && (
          <span className="flex items-center gap-1 text-[var(--await)]" title="awaiting reply">
            <Hourglass size={15} /> {awaiting}
          </span>
        )}
      </div>
    </div>
  )
}

export type AtlasProps = {
  journeys: AtlasJourney[]
  hrefOf: (q: AtlasJourney) => string | undefined // undefined: the journey has no chart to open
  eyebrow: ReactNode // the small line above the title
  title?: string // the page's title: "Atlas" unless it shows one region
  regionKey?: string // set on a region's page: its title is the region's name, marked as a region
  onRename?: (name: string) => Promise<void> // renames the region, on its page
  regions?: AtlasRegion[] // set on the home page: the regions as cards instead of the journeys' shelves
  regionHref?: (r: AtlasRegion) => string
  noMap?: string // what a journey without a chart says about it
  banner?: ReactNode // e.g. a GitHub warning, shown under the header
  empty?: ReactNode // shown instead of the shelves when there are no journeys at all
  search?: boolean // the live atlas searches the API; the mock has none
  version?: string // the running server's version, shown small at the bottom; the mock has none
}

export default function Atlas({ journeys, hrefOf, eyebrow, title = 'Atlas', regionKey, onRename, regions, regionHref, noMap, banner, empty, search, version }: AtlasProps) {
  const [theme, setTheme] = useTheme()
  const shelved = journeys.filter((q) => !q.archivedAt)
  const archived = journeys.filter((q) => q.archivedAt)
  const active = shelved.filter((q) => !cancelled(q) && !mainDone(q))
  const sum = (f: (q: AtlasJourney) => number) => active.reduce((n, q) => n + f(q), 0)
  const row = (q: AtlasJourney) => <JourneyRow key={q.key} q={q} href={hrefOf(q)} noMap={noMap} chip={false} />
  // The archive mixes every state, so there each row says its own.
  const archivedRow = (q: AtlasJourney) => <JourneyRow key={q.key} q={q} href={hrefOf(q)} noMap={noMap} chip />

  return (
    <div data-theme={theme} className="quest-theme min-h-screen">
      <header className="quest-header flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-[var(--panel-border)] bg-[var(--panel)] px-6 py-4">
        <div className="quest-nameplate">
          <div className="quest-crumb text-[12px] font-bold tracking-[0.2em] text-[var(--ink-soft)] uppercase">{eyebrow}</div>
          <h1 className="quest-display flex min-w-0 items-center gap-2 text-2xl font-semibold">
            {regionKey ? <Renamable title={title} noun="region" idKey={regionKey} onRename={onRename} /> : title}
            {regionKey && <RegionChip regionKey={regionKey} />}
          </h1>
        </div>
        <div className="flex min-w-0 flex-1 items-center justify-end gap-4">
          {/* The main quests of every journey underway, as one tally. */}
          <Tally
            title={`${active.length} ${active.length === 1 ? 'journey' : 'journeys'} underway`}
            done={sum((q) => q.main.done)}
            total={sum((q) => q.main.total)}
            shares={[
              { key: 'done', n: sum((q) => q.main.done), label: 'fulfilled' },
              { key: 'open', n: sum((q) => q.available), label: 'open now' },
              { key: 'awaiting', n: sum((q) => q.awaiting), label: 'awaiting reply' },
              { key: 'sealed', n: sum((q) => Math.max(0, q.main.total - q.main.done - q.available - q.awaiting)), label: 'sealed' },
            ]}
          />
          {/* The buttons, racked beside the tally as on a journey's header. */}
          <div className="quest-tools grid shrink-0 grid-cols-1 gap-1.5">
            {search && <Search />}
            <ThemeMenu theme={theme} onChange={setTheme} />
          </div>
        </div>
      </header>
      {banner}

      {journeys.length === 0 && empty ? (
        <main className="mx-auto max-w-7xl p-6">{empty}</main>
      ) : (
        <main className={`mx-auto max-w-7xl space-y-8 p-6 ${theme === 'wartable' ? 'wt-sheet' : ''}`}>
          {/* The war table's campaign map, spread under the shelves. */}
          {theme === 'wartable' && <div className="wt-map" aria-hidden />}
          {regions ? (
            <Section title="Regions" hint="each holds its own journeys">
              {regions.map((r) => (
                <RegionRow key={r.key} region={r} href={regionHref?.(r) ?? '#'} />
              ))}
            </Section>
          ) : (
            <Shelves journeys={shelved} row={row} />
          )}
          {!regions && archived.length > 0 && (
            // Closed by default; the browser keeps it open across the 15 s refresh.
            <details className="group">
              <summary className="quest-shelf-title mb-3 flex cursor-pointer list-none items-baseline gap-3 select-none">
                <h2 className="quest-display text-[13px] font-bold tracking-[0.2em] whitespace-nowrap uppercase">
                  <ChevronRight size={14} className="mr-1 inline align-[-2px] transition-transform group-open:rotate-90" />
                  Archived ({archived.length})
                </h2>
                <span className="text-[14px] text-[var(--ink-soft)]">
                  put away with <span className="font-mono">mikado journey archive</span> — charts still open
                </span>
              </summary>
              <Rows>{archived.map(archivedRow)}</Rows>
            </details>
          )}
        </main>
      )}
      {version && <VersionLine version={version} className="mx-auto max-w-7xl px-6 pb-4" />}
    </div>
  )
}
