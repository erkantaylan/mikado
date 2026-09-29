import { createContext, useContext, useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import '../quest/quest.css'
import './headers.css'
import './headers-c.css'
import { stateColour } from '../quest/look'
import { THEMES, useTheme } from '../quest/theme'
import { PageTheme, WIDTHS, cases, initialWidth, keepWidth, type Case, type Width } from './headerKit'
import { Bar, Choice, Elsewhere, Figures, Legend, Tools, Wood } from './HeaderParts'

// A throwaway page to refine header C (two decks): no emblem, the way back as a red wood sign and
// every id (J3, Q9, R2) on a chip of wood with red letters. Six takes, each shown on the two charts,
// the Atlas and a strip of the other places ids show. All data is made up.

type VariantId = 'c0' | 'c1' | 'c2' | 'c3' | 'c4' | 'c5'
const Variant = createContext<VariantId>('c0')

// ---- ids and the way back, per variant ----------------------------------------------

/** An id (J3, Q9, R2). plain: its classes in C0, which keeps today's look for each place. */
function Id({ k, plain = 'font-mono', link }: { k: string; plain?: string; link?: boolean }) {
  const v = useContext(Variant)
  const tip = link ? `Go to ${k}` : undefined
  if (v === 'c0') return <span className={`hc-id-plain ${plain}`}>{k}</span>
  if (v === 'c3')
    return (
      <span className="hc-oak hc-oak-chip" title={tip}>
        {k}
      </span>
    )
  if (v === 'c5')
    return (
      <Wood shape="tag" tone="pale" className="hc-idw" title={tip}>
        {k}
      </Wood>
    )
  return (
    <span className="hc-chip" title={tip}>
      {k}
    </span>
  )
}

/** The way back: to the region on a chart, the app's name on the Atlas. */
function Back({ c }: { c: Case }) {
  const v = useContext(Variant)
  const to = c.chart ? `Back to ${c.crumb}` : 'mikado'
  // The arrow character is tiny in both faces; a drawn one reads at this size.
  const arrow = c.chart && <ArrowLeft size={13} strokeWidth={3} className="hc-arrow" aria-hidden />
  if (v === 'c0')
    return c.chart ? (
      <span className="quest-crumb shrink-0 cursor-pointer text-[12px] font-bold hover:underline" style={{ color: stateColour.done }}>
        ← {c.crumb}
        <Id k={c.jkey!} plain="ml-2 font-mono text-[var(--ink-soft)]" />
      </span>
    ) : (
      <span className="quest-crumb shrink-0 text-[12px] font-bold text-[var(--ink-soft)]">{c.crumb}</span>
    )
  const id = c.jkey && <Id k={c.jkey} />
  if (v === 'c3')
    return (
      <span className="hc-backline">
        <span className="hc-oak hc-oak-sign" title={to}>
          {arrow}
          {c.crumb}
        </span>
        {id}
      </span>
    )
  if (v === 'c2')
    return (
      <span className="hc-backline">
        <Wood shape="arrow" tone="red" className="hc-sign" title={to}>
          {c.crumb}
        </Wood>
        {id}
      </span>
    )
  if (v === 'c4')
    return (
      <span className="hc-backline">
        <Wood shape="arrow" tone="red" className="hc-sign" title={to}>
          {c.crumb}
          {id && (
            <>
              <span className="hc-seam" aria-hidden />
              {id}
            </>
          )}
        </Wood>
      </span>
    )
  // c1, c5: a rectangular plank
  return (
    <span className="hc-backline">
      <Wood shape="plank" tone="red" className="hc-sign" title={to}>
        {arrow}
        {c.crumb}
      </Wood>
      {id}
    </span>
  )
}

// ---- the header (C, without the emblem) -----------------------------------------------

const bar = 'quest-header border-b border-[var(--panel-border)] bg-[var(--panel)]'

function Header({ c }: { c: Case }) {
  return (
    <header className={`${bar} hd-c hc-c`}>
      <div className="hd-deck">
        <div className="quest-nameplate flex min-w-0 items-center gap-3">
          <div className="min-w-0 flex-1">
            <div className="hc-crumbline flex min-w-0 items-center gap-2 whitespace-nowrap">
              <Back c={c} />
              {c.crown && (
                <span className="quest-subtitle hc-aside min-w-0 text-[13px] text-[var(--ink-soft)]">
                  <span className="shrink-0">Crowned by</span>
                  <Id k={c.crown} link plain="font-mono font-semibold text-[var(--ink)] underline decoration-dotted underline-offset-2" />
                  <span className="min-w-0 truncate">· {c.holds}</span>
                </span>
              )}
            </div>
            <h1 className="quest-display truncate text-xl font-semibold" title={c.title}>
              {c.title}
            </h1>
          </div>
        </div>
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

// ---- the variants ---------------------------------------------------------------

const variants: { id: VariantId; name: string; idea: string }[] = [
  {
    id: 'c0',
    name: 'Plain',
    idea: 'C as it stands, without the round emblem: the way back and the ids as today. The baseline.',
  },
  {
    id: 'c1',
    name: 'Red plank',
    idea: 'The way back is a flat red-painted plank with a dark outline and cream lettering (the look of the reference sign, as a rectangle). Every id is a small pale wood chip in the same style, with red letters.',
  },
  {
    id: 'c2',
    name: 'Signpost',
    idea: 'The way back is the reference sign itself: a red wood arrow pointing left, so the “←” is the shape, not a character. Ids are the same pale wood chips with red letters.',
  },
  {
    id: 'c3',
    name: 'Dark oak',
    idea: 'The sign and the id tags are cut from the dark oak of the war table, with bright vermilion letters. A quieter, more grown-up take on the red; it depends on the letters staying bright enough on the dark grain.',
  },
  {
    id: 'c4',
    name: 'Sign carries the id',
    idea: 'One red arrow sign holds the way back and the journey’s id: “Kitchen”, a seam, then the id on a pale chip set into the sign. The other ids are pale chips with red letters.',
  },
  {
    id: 'c5',
    name: 'Nailed tags',
    idea: 'The red plank of C1, and ids as small wood tags: clipped corners and a nail at the left end, a slightly ragged right end. The most decorative take (optional ornament, per the clarity-first rule).',
  },
]

// ---- the page -----------------------------------------------------------------

export default function HeaderCDemo() {
  const [theme, setTheme] = useTheme()
  const [width, setWidth] = useState<Width>(initialWidth)
  const pickWidth = (w: Width) => {
    setWidth(w)
    keepWidth(w)
  }
  const frame = { width: width === 'full' ? '100%' : `${width}px` }

  return (
    <PageTheme value={[theme, setTheme]}>
      <div data-theme={theme} className="quest-theme hd-page hc-page min-h-screen">
        <div className="hd-top sticky top-0 z-40 flex flex-wrap items-center gap-x-8 gap-y-2 border-b border-[var(--panel-border)] bg-[var(--panel)] px-6 py-3">
          <h1 className="quest-display text-2xl font-semibold">Header C, refined</h1>
          <Choice label="Theme" options={THEMES.map((t) => [t.id, t.name] as const)} value={theme} onChange={setTheme} />
          <Choice label="Frame" options={WIDTHS.map((w) => [w, w === 'full' ? 'Full width' : `${w}px`] as const)} value={width} onChange={pickWidth} />
          <span className="text-[13px] text-[var(--ink-soft)]">Made-up data; tell us the good and bad ones by number.</span>
        </div>
        <main className={`space-y-14 px-6 py-8 ${width === 'full' ? '' : 'w-max min-w-full'}`}>
          {variants.map((v) => (
            <Variant key={v.id} value={v.id}>
              <section id={v.id} className={`hc-v hc-${v.id} space-y-4`}>
                <div className="max-w-[860px]">
                  <h2 className="quest-display text-[22px] font-semibold">
                    C{v.id.slice(1)} · {v.name}
                  </h2>
                  <p className="text-[14px] text-[var(--ink-soft)]">{v.idea}</p>
                </div>
                {cases.map((c) => (
                  <figure key={c.label}>
                    <figcaption className="mb-1.5 text-[12px] font-semibold text-[var(--ink-soft)]">{c.label}</figcaption>
                    <div className="hd-frame relative" style={frame}>
                      <Header c={c} />
                      {/* A strip of the page under the header, so its edge reads. */}
                      <div className="hd-strip relative isolate h-[60px]">{theme === 'wartable' && <div className="wt-map wt-map-table" aria-hidden />}</div>
                    </div>
                  </figure>
                ))}
                <figure>
                  <figcaption className="mb-1.5 text-[12px] font-semibold text-[var(--ink-soft)]">Ids elsewhere</figcaption>
                  <div className="hd-frame relative" style={frame}>
                    <Elsewhere Id={Id} wartable={theme === 'wartable'} />
                  </div>
                </figure>
              </section>
            </Variant>
          ))}
        </main>
      </div>
    </PageTheme>
  )
}
