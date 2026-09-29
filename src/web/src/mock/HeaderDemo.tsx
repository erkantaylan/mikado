import { useState, type ReactNode } from 'react'
import { Trophy } from 'lucide-react'
import '../quest/quest.css'
import './headers.css'
import { Tally, stateColour } from '../quest/look'
import { THEMES, useTheme } from '../quest/theme'
import { PageTheme, WIDTHS, all, cases, initialWidth, keepWidth, pct, spelled, type Case, type Width } from './headerKit'
import { Bar, Choice, Figures, Item, Legend, Tools } from './HeaderParts'

// A throwaway page to choose the page header: eight named takes on it, side by side, each shown
// on a chart with a short title, a chart with a long one and the Atlas. All data is made up.

// ---- pieces the variants share ------------------------------------------------

function Emblem({ size = 'size-11' }: { size?: string }) {
  return (
    <span className={`quest-emblem grid ${size} shrink-0 place-items-center rounded-full border-2`} style={{ borderColor: 'var(--gold)', color: 'var(--gold)' }}>
      <Trophy size={20} />
    </span>
  )
}

/** The way back: to the region on a chart, the app's name on the Atlas. */
function Crumb({ c }: { c: Case }) {
  if (!c.chart) return <span className="quest-crumb shrink-0 text-[12px] font-bold text-[var(--ink-soft)]">{c.crumb}</span>
  return (
    <span className="quest-crumb shrink-0 cursor-pointer text-[12px] font-bold hover:underline" style={{ color: stateColour.done }}>
      ← {c.crumb}
      <span className="ml-2 font-mono text-[var(--ink-soft)]">{c.jkey}</span>
    </span>
  )
}

function Subtitle({ c }: { c: Case }) {
  return (
    <>
      Crowned by <span className="font-mono font-semibold text-[var(--ink)] underline decoration-dotted underline-offset-2">{c.crown}</span> · {c.holds}
    </>
  )
}

/**
 * The title's nameplate. sub: the subtitle under the title (as today), beside the crumb (to save a
 * line) or left out. children go under the title.
 */
function Plate({ c, sub = 'below', emblem = 'size-11', titleClass = 'text-xl', children }: { c: Case; sub?: 'below' | 'beside'; emblem?: string; titleClass?: string; children?: ReactNode }) {
  return (
    <div className="quest-nameplate flex min-w-0 items-center gap-3">
      {c.chart && <Emblem size={emblem} />}
      <div className="min-w-0 flex-1">
        {sub === 'beside' && c.crown ? (
          <div className="flex min-w-0 items-baseline gap-2 whitespace-nowrap">
            <Crumb c={c} />
            <span className="quest-subtitle hd-aside min-w-0 truncate text-[13px] text-[var(--ink-soft)]">
              <Subtitle c={c} />
            </span>
          </div>
        ) : (
          <div>
            <Crumb c={c} />
          </div>
        )}
        <h1 className={`quest-display truncate font-semibold ${titleClass}`} title={c.title}>
          {c.title}
        </h1>
        {sub === 'below' && c.crown && (
          <div className="quest-subtitle truncate text-[14px] text-[var(--ink-soft)]">
            <Subtitle c={c} />
          </div>
        )}
        {children}
      </div>
    </div>
  )
}

const bar = 'quest-header border-b border-[var(--panel-border)] bg-[var(--panel)]'

// ---- the variants ---------------------------------------------------------------

/** A: today's headers, from Atlas.tsx and JourneyMap.tsx. */
function Current({ c }: { c: Case }) {
  if (!c.chart)
    return (
      <header className={`${bar} flex flex-wrap items-center gap-x-6 gap-y-2 px-6 py-4`}>
        <div className="quest-nameplate">
          <div className="quest-crumb text-[12px] font-bold text-[var(--ink-soft)]">{c.crumb}</div>
          <h1 className="quest-display flex min-w-0 items-center gap-2 text-2xl font-semibold">{c.title}</h1>
        </div>
        <div className="flex min-w-0 flex-1 items-center justify-end gap-4">
          <Tally {...c.tally} />
          <Tools c={c} search="full" className="quest-tools grid shrink-0 grid-cols-1 gap-1.5" />
        </div>
      </header>
    )
  return (
    <header className={`${bar} flex items-center gap-x-5 px-5 py-3`}>
      <Plate c={c} />
      <div className="flex min-w-0 flex-1 items-center justify-end gap-4">
        <Tally {...c.tally} />
        <Tools c={c} search="full" className="quest-tools grid shrink-0 grid-cols-2 gap-1.5" />
      </div>
    </header>
  )
}

/** B: one row of one height; the legend joins the count line. */
function Level({ c }: { c: Case }) {
  return (
    <header className={`${bar} hd-b`}>
      <Plate c={c} sub="beside" emblem="size-9" titleClass="text-[19px] leading-6" />
      <div className="quest-tally rounded-md border border-[var(--panel-border)] bg-[var(--plate)]" title={spelled(c.tally)}>
        <div className="flex min-w-0 items-baseline gap-4">
          <Figures t={c.tally} />
          <Legend t={c.tally} className="hd-fit flex-1 justify-end" />
        </div>
        <Bar t={c.tally} />
      </div>
      <Tools c={c} search="chip" className="hd-tools" />
    </header>
  )
}

/** C: title and buttons on the top deck, a slim tally the full width below. */
function TwoDecks({ c }: { c: Case }) {
  return (
    <header className={`${bar} hd-c`}>
      <div className="hd-deck">
        <Plate c={c} sub="beside" emblem="size-9" />
        <Tools c={c} search="full" className="hd-tools" />
      </div>
      <div className="quest-tally rounded-md border border-[var(--panel-border)] bg-[var(--plate)]">
        <Figures t={c.tally} />
        <Bar t={c.tally} className="h-2 min-w-[120px] flex-1" />
        <Legend t={c.tally} className="shrink-0" />
      </div>
    </header>
  )
}

/** D: the title takes the row; the tally is a small block whose legend shows on hover. */
function TitleRow({ c }: { c: Case }) {
  return (
    <header className={`${bar} hd-d`}>
      <Plate c={c} />
      <div className="quest-tally rounded-md border border-[var(--panel-border)] bg-[var(--plate)]" tabIndex={0} aria-label={spelled(c.tally)}>
        <Figures t={c.tally} />
        <Bar t={c.tally} className="h-2 w-[160px]" />
        <div className="hd-pop quest-tally-legend text-[12px] font-semibold text-[var(--ink-soft)]">
          {all(c.tally).map((s) => (
            <Item key={s.key} s={s} />
          ))}
        </div>
      </div>
      <Tools c={c} search="icon" className="hd-tools hd-small" />
    </header>
  )
}

/** E: nameplate and buttons; the tally is a rule along the bottom edge, its counts a status line above it. */
function Rule({ c }: { c: Case }) {
  return (
    <header className={`${bar} hd-e`}>
      <Plate c={c} />
      <div className="hd-status" title={spelled(c.tally)}>
        <Figures t={c.tally} />
        <Legend t={c.tally} className="hd-fit justify-end" />
      </div>
      <Tools c={c} search="full" className="hd-tools" />
      <Bar t={c.tally} className="hd-rule" />
    </header>
  )
}

/** F: nameplate and tally only; the buttons hang under the header's right end (Rail). */
function ToolRail({ c }: { c: Case }) {
  return (
    <header className={`${bar} hd-f`}>
      <Plate c={c} />
      <Tally {...c.tally} />
    </header>
  )
}

function Rail({ c }: { c: Case }) {
  return (
    <div className="quest-header hd-rail">
      <Tools c={c} search="icon" className="hd-tools" />
    </div>
  )
}

/** G: the bar and its counts under the title, inside the nameplate. */
function Stacked({ c }: { c: Case }) {
  const t = c.tally
  return (
    <header className={`${bar} hd-g`}>
      <Plate c={c} sub="beside">
        <div className="hd-gbar">
          <span className="font-bold text-[var(--ink-soft)]">{t.title}</span>
          <Bar t={t} className="h-2 flex-1" />
          <span className="font-bold whitespace-nowrap text-[var(--ink)] tabular-nums">
            {t.done}/{t.total} · {pct(t)}%
          </span>
        </div>
        <Legend t={t} className="hd-fit mt-1" />
      </Plate>
      <Tools c={c} search="full" className="hd-tools" />
    </header>
  )
}

/** H: one line: title, the counts as chips, the buttons. */
function Chips({ c }: { c: Case }) {
  const t = c.tally
  return (
    <header className={`${bar} hd-h`}>
      <div className="quest-nameplate">
        <Crumb c={c} />
        <h1 className="quest-display truncate text-lg font-semibold" title={c.crown ? `${c.title}\nCrowned by ${c.crown} · ${c.holds}` : c.title}>
          {c.title}
        </h1>
      </div>
      <div className="hd-chips" title={spelled(t)}>
        <span className="hd-sum tabular-nums">
          {t.title} {t.done}/{t.total} · {pct(t)}%
        </span>
        {all(t)
          .filter((s) => s.n > 0)
          .map((s) => (
            <span key={s.key} data-chip={s.key} className="hd-chip">
              <span data-share={s.key} className="quest-share size-2 shrink-0 rounded-sm" />
              <b className="tabular-nums">{s.n}</b> {s.label}
            </span>
          ))}
      </div>
      <Tools c={c} search="icon" className="hd-tools hd-small" />
    </header>
  )
}

const variants: { letter: string; name: string; idea: string; Header: (p: { c: Case }) => ReactNode; rail?: boolean }[] = [
  {
    letter: 'A',
    name: 'Current',
    idea: 'Today’s header, the baseline. Title, tally and the two-row rack of buttons each take their own height, and a long title or a narrow window pushes the tally and buttons onto new rows.',
    Header: Current,
  },
  {
    letter: 'B',
    name: 'Level',
    idea: 'One row, 72px, everything the same height. The subtitle joins the crumb line and the legend the count line, so the tally is two lines; legend counts that do not fit drop off the end (hover the tally for all).',
    Header: Level,
  },
  {
    letter: 'C',
    name: 'Two decks',
    idea: 'Two rows by design: title and buttons on top, a slim full-width tally below with count, bar and legend on one line. The buttons can never fall under the tally, at the cost of a taller header.',
    Header: TwoDecks,
  },
  {
    letter: 'D',
    name: 'Title owns the row',
    idea: 'The nameplate takes all the free space; the tally shrinks to its count and a short bar, with the legend on hover. Best for long titles; the split by state is one hover away.',
    Header: TitleRow,
  },
  {
    letter: 'E',
    name: 'Progress rule',
    idea: 'Nameplate and buttons only; the tally becomes a thin rule along the header’s bottom edge, its counts a small status line above it. Quiet and short, but the bar is thin and easy to overlook.',
    Header: Rule,
  },
  {
    letter: 'F',
    name: 'Tool rail',
    idea: 'The header holds the nameplate and the full tally, at one height; the buttons leave it for a small rail under its right end. The header has room to spare, but the rail covers a corner of the page.',
    Header: ToolRail,
    rail: true,
  },
  {
    letter: 'G',
    name: 'Stacked nameplate',
    idea: 'The bar sits under the title, the count beside it and the legend under that, so title and progress read as one unit; the buttons keep one row on the right. A taller nameplate, but only one block.',
    Header: Stacked,
  },
  {
    letter: 'H',
    name: 'Compact chips',
    idea: 'One short line: title, then the counts as coloured chips instead of a bar, then the buttons. The shortest header; the subtitle moves into the title’s tooltip, zero counts are left out and chips that do not fit drop off.',
    Header: Chips,
  },
]

// ---- the page -----------------------------------------------------------------

export default function HeaderDemo() {
  const [theme, setTheme] = useTheme()
  const [width, setWidth] = useState<Width>(initialWidth)
  const pickWidth = (w: Width) => {
    setWidth(w)
    keepWidth(w)
  }

  return (
    <PageTheme value={[theme, setTheme]}>
      <div data-theme={theme} className="quest-theme hd-page min-h-screen">
        <div className="hd-top sticky top-0 z-40 flex flex-wrap items-center gap-x-8 gap-y-2 border-b border-[var(--panel-border)] bg-[var(--panel)] px-6 py-3">
          <h1 className="quest-display text-2xl font-semibold">Header alternatives</h1>
          <Choice label="Theme" options={THEMES.map((t) => [t.id, t.name] as const)} value={theme} onChange={setTheme} />
          <Choice label="Frame" options={WIDTHS.map((w) => [w, w === 'full' ? 'Full width' : `${w}px`] as const)} value={width} onChange={pickWidth} />
          <span className="text-[13px] text-[var(--ink-soft)]">Made-up data; tell us the good and bad ones by letter.</span>
        </div>
        <main className={`space-y-14 px-6 py-8 ${width === 'full' ? '' : 'w-max min-w-full'}`}>
          {variants.map((v) => (
            <section key={v.letter} className="space-y-4">
              <div className="max-w-[860px]">
                <h2 className="quest-display text-[22px] font-semibold">
                  {v.letter} · {v.name}
                </h2>
                <p className="text-[14px] text-[var(--ink-soft)]">{v.idea}</p>
              </div>
              {cases.map((c) => (
                <figure key={c.label}>
                  <figcaption className="mb-1.5 text-[12px] font-semibold text-[var(--ink-soft)]">{c.label}</figcaption>
                  <div className="hd-frame relative" style={{ width: width === 'full' ? '100%' : `${width}px` }}>
                    <v.Header c={c} />
                    {/* A strip of the page under the header, so its edge reads. */}
                    <div className={`hd-strip relative isolate ${v.rail ? 'h-[150px]' : 'h-[60px]'}`}>
                      {theme === 'wartable' && <div className="wt-map wt-map-table" aria-hidden />}
                      {v.rail && <Rail c={c} />}
                    </div>
                  </div>
                </figure>
              ))}
            </section>
          ))}
        </main>
      </div>
    </PageTheme>
  )
}
