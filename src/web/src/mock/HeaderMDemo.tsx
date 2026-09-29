import { useContext, useLayoutEffect, useRef, useState } from 'react'
import { Background, ControlButton, Controls, ReactFlow } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { ChevronDown, ChevronUp, Eye, Map as MapIcon, PanelRightClose, PanelRightOpen } from 'lucide-react'
import '../quest/quest.css'
import './headers.css'
import './headers-c.css'
import './headers-m.css'
import { THEMES, ThemeMenu, useTheme } from '../quest/theme'
import { PageTheme, WIDTHS, cases as baseCases, initialWidth, keepWidth, spelled, type Case, type Width } from './headerKit'
import { Bar, Choice, Elsewhere, Figures, Legend, SearchTool, Wood } from './HeaderParts'

// A throwaway page for the merged header: C4's red arrow sign (just the place's name) and the title on
// one line, C5's nailed tags for every id, the journey's id at the left end of the tally, no subtitle.
// The header keeps Search and the theme; the side-panel button moves into the chart's control stack
// (zoom, fit, overview, finished quests). Two header layouts and three places in the stack. All data
// is made up.

type VariantId = 'm1' | 'm2' | 'm3' | 'm4'
/** The header's layout, and where the panel button sits in the control stack. */
type Look = { header: 'decks' | 'row'; stack: 'gap' | 'rule' | 'top' }

/** The previews: the two charts, the first again with its side panel hidden, and the Atlas. */
type Preview = Case & { panel: boolean; collapsed?: boolean }
const [short, long, atlas] = baseCases
const previews: Preview[] = [
  { ...short, panel: true },
  { ...long, panel: true },
  { ...short, label: 'Chart, side panel hidden', panel: false },
  { ...short, label: 'Chart, header collapsed', panel: true, collapsed: true },
  { ...atlas, panel: false },
]
/** A single-row header has no first row to fold away, so no collapsed preview. */
const previewsFor = (look: Look) => previews.filter((c) => look.header === 'decks' || !c.collapsed)

// ---- ids and the way back -------------------------------------------------------------

/** Every id is a small nailed tag of pale wood with red letters (C5). */
function Tag({ k, link, tip }: { k: string; plain?: string; link?: boolean; tip?: string }) {
  return (
    <Wood shape="tag" tone="pale" className="hc-idw" title={tip ?? (link ? `Go to ${k}` : k)}>
      {k}
    </Wood>
  )
}

/** The way back: a red wood arrow with the place's name (C4, without the id). */
function Sign({ c }: { c: Case }) {
  return (
    <Wood shape="arrow" tone="red" className="hc-sign" title={c.chart ? `Back to ${c.crumb}` : 'mikado'}>
      {c.crumb}
    </Wood>
  )
}

// ---- the header ---------------------------------------------------------------

const bar = 'quest-header border-b border-[var(--panel-border)] bg-[var(--panel)]'

function Header({ c, look, folded, onFold }: { c: Preview; look: Look; folded: boolean; onFold: () => void }) {
  const [theme, setTheme] = useContext(PageTheme)
  // The sign and the title share one line; the subtitle is gone.
  const nameplate = (
    <div className="quest-nameplate hm-oneline flex min-w-0 items-center gap-3">
      <Sign c={c} />
      <h1 className="quest-display min-w-0 truncate text-xl font-semibold" title={c.title}>
        {c.title}
      </h1>
    </div>
  )
  const tools = (
    <div className="hd-tools">
      <SearchTool look="full" />
      <ThemeMenu theme={theme} onChange={setTheme} />
    </div>
  )
  // Its tooltip names the journey too, for when the title is folded away.
  const id = c.jkey && <Tag k={c.jkey} tip={`${c.jkey} · ${c.title}`} />
  if (look.header === 'row')
    // One row: the nameplate, the tally filling the middle (its legend dropping whole counts off its
    // end when short of room; the tooltip spells them all), the buttons.
    return (
      <header className={`${bar} hd-m4 hm-head hm-row`}>
        {nameplate}
        <div className="quest-tally rounded-md border border-[var(--panel-border)] bg-[var(--plate)]" title={spelled(c.tally)}>
          {id}
          <Figures t={c.tally} />
          <Bar t={c.tally} className="hm-bar h-2" />
          <Legend t={c.tally} className="hd-fit hm-fit justify-end" />
        </div>
        {tools}
      </header>
    )
  return (
    <header className={`${bar} hd-c hc-c hm-head`} data-folded={folded || undefined}>
      {!folded && (
        <div className="hd-deck">
          {nameplate}
          {tools}
        </div>
      )}
      <div className="quest-tally rounded-md border border-[var(--panel-border)] bg-[var(--plate)]">
        {id}
        <Figures t={c.tally} />
        <Bar t={c.tally} className="h-2 min-w-[120px] flex-1" />
        <Legend t={c.tally} className="shrink-0" />
        <FoldButton folded={folded} onFold={onFold} />
      </div>
    </header>
  )
}

/** Folds the header's first row away (the tally alone stays) and back. Ctrl K still opens search. */
function FoldButton({ folded, onFold }: { folded: boolean; onFold: () => void }) {
  const say = folded ? 'Show the title and buttons' : 'Hide the title and buttons'
  return (
    <button onClick={onFold} aria-label={say} title={say} aria-expanded={!folded} className="quest-tool hm-fold grid place-items-center rounded-md border border-[var(--panel-border)] text-[var(--ink-soft)] hover:text-[var(--ink)]">
      {folded ? <ChevronDown size={16} /> : <ChevronUp size={16} />}
    </button>
  )
}

// ---- a slice of the page under the header -----------------------------------------------

const TABS = ['Journey', 'Chronicle · 21', 'Legend', 'Glossary']

/**
 * The chart's control stack, bottom left, as the real chart draws it: React Flow's own zoom and fit
 * buttons, the overview and the finished quests, and the side-panel button in one of three places.
 * An empty React Flow underneath, so the buttons are the real ones (they zoom nothing).
 */
function Stack({ look, open, onToggle, theme }: { look: Look; open: boolean; onToggle: () => void; theme: string }) {
  const say = open ? 'Hide side panel' : 'Show side panel'
  const panel = (
    <ControlButton onClick={onToggle} title={say} aria-label={say} className={`hm-panelbtn hm-at-${look.stack}`}>
      {open ? <PanelRightClose size={14} /> : <PanelRightOpen size={14} />}
    </ControlButton>
  )
  return (
    <div className={`hm-flow hm-stack-${look.stack} absolute inset-0`}>
      <ReactFlow
        nodes={[]}
        edges={[]}
        colorMode={theme === 'midnight' ? 'dark' : 'light'}
        panOnDrag={false}
        zoomOnScroll={false}
        preventScrolling={false}
        proOptions={{ hideAttribution: true }}
        style={{ background: 'transparent' }}
      >
        {theme !== 'wartable' && <Background gap={24} size={1.5} color="var(--dots)" bgColor="transparent" />}
        <Controls showInteractive={false}>
          <ControlButton title="Hide the overview" aria-label="Hide the overview" aria-pressed>
            <MapIcon size={14} />
          </ControlButton>
          <ControlButton title="Show or hide finished quests" aria-label="Show or hide finished quests">
            <Eye size={14} />
          </ControlButton>
          {look.stack !== 'top' && <span className="hm-stack-break" aria-hidden />}
          {panel}
        </Controls>
      </ReactFlow>
    </div>
  )
}

/** The chart area under the header: the map and its control stack, and the top of the side panel. */
function Slice({ look, open, onToggle }: { look: Look; open: boolean; onToggle: () => void }) {
  const [theme] = useContext(PageTheme)
  return (
    <div className="hm-slice flex h-[220px] overflow-hidden">
      <div className="hd-strip hm-chart relative isolate min-w-0 flex-1">
        {theme === 'wartable' && <div className="wt-map wt-map-table" aria-hidden />}
        <Stack look={look} open={open} onToggle={onToggle} theme={theme} />
      </div>
      {open && (
        <aside className="quest-ledger hm-ledger relative z-10 flex shrink-0 flex-col border-l border-[var(--panel-border)] bg-[var(--panel)] shadow-[-8px_0_16px_-10px_rgba(0,0,0,0.35)]">
          <div role="tablist" className="quest-tabs flex shrink-0 gap-1 border-b border-[var(--panel-border)] px-3 pt-3">
            {TABS.map((t, i) => (
              <button
                key={t}
                role="tab"
                aria-selected={i === 0}
                className={`quest-tab -mb-px rounded-t-md border px-3 py-1.5 text-[13px] font-semibold whitespace-nowrap ${
                  i === 0 ? 'border-[var(--panel-border)] border-b-[var(--panel)] bg-[var(--panel)] text-[var(--ink)]' : 'border-transparent text-[var(--ink-soft)]'
                }`}
              >
                {t}
              </button>
            ))}
          </div>
          <div className="quest-section p-4">
            <h3 className="quest-section-title mb-2 text-[13px] font-semibold">Open now</h3>
            <ul className="quest-rows space-y-1.5">
              <li className="quest-row hm-row-look flex items-center gap-1.5 overflow-hidden rounded-md border border-[var(--panel-border)] bg-[var(--plate)] px-2.5 py-2 text-[14px] whitespace-nowrap">
                <Tag k="Q11" />
                <span className="min-w-0 truncate">Heat a knob of butter in the pan until it foams</span>
              </li>
              <li className="quest-row hm-row-look flex items-center gap-1.5 overflow-hidden rounded-md border border-[var(--panel-border)] bg-[var(--plate)] px-2.5 py-2 text-[14px] whitespace-nowrap">
                <Tag k="Q16" />
                <span className="min-w-0 truncate">Buy a box of eggs from the corner shop</span>
              </li>
            </ul>
          </div>
        </aside>
      )}
    </div>
  )
}

// ---- one preview: the header, its measured height, and the slice under it ------------------

function Figure({ c, look, frame, wartable }: { c: Preview; look: Look; frame: { width: string }; wartable: boolean }) {
  const [open, setOpen] = useState(c.panel)
  const toggle = () => setOpen((o) => !o)
  const [folded, setFolded] = useState(!!c.collapsed)
  const box = useRef<HTMLDivElement>(null)
  const [height, setHeight] = useState(0)
  useLayoutEffect(() => {
    const el = box.current?.querySelector('header')
    if (!el) return
    const ro = new ResizeObserver(() => setHeight(Math.round(el.getBoundingClientRect().height)))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return (
    <figure>
      <figcaption className="mb-1.5 flex gap-3 text-[12px] font-semibold text-[var(--ink-soft)]">
        <span>{c.label}</span>
        {height > 0 && <span className="font-normal text-[var(--ink-faint)]">header {height}px tall</span>}
      </figcaption>
      <div ref={box} className="hd-frame relative" style={frame}>
        <Header c={c} look={look} folded={folded} onFold={() => setFolded((f) => !f)} />
        {c.chart ? <Slice look={look} open={open} onToggle={toggle} /> : <div className="hd-strip relative isolate h-[60px]">{wartable && <div className="wt-map wt-map-table" aria-hidden />}</div>}
      </div>
    </figure>
  )
}

// ---- the variants ---------------------------------------------------------------

const variants: { id: VariantId; name: string; look: Look; idea: string }[] = [
  {
    id: 'm1',
    name: 'Two decks',
    look: { header: 'decks', stack: 'gap' },
    idea: 'The sign and the title on one line with Search and the theme on the right; the tally below, the journey’s id first. A chevron at the tally’s right end folds the first row away, leaving only the tally (the journey’s id included); a folded header has no Search button, but Ctrl K still opens search. The side-panel button sits at the foot of the chart’s control stack, a small gap above it.',
  },
  {
    id: 'm2',
    name: 'Single row',
    look: { header: 'row', stack: 'gap' },
    idea: 'Everything on one row: the sign and title, the tally filling the middle, Search and the theme. The shortest header, but the title and the tally share one width. At 1366px the long title is cut after about 35 characters (“mikado can hook in any task tool, no…”) and the legend keeps only fulfilled, underway and open; with the short title it drops only “abandoned”. At 1600px the short title shows everything and the long one still loses sealed and abandoned. Hover the tally for every count. With no first row to fold away it has no chevron. The stack as in M1.',
  },
  {
    id: 'm3',
    name: 'Stack end',
    look: { header: 'decks', stack: 'rule' },
    idea: 'M1’s header, chevron and all. The panel button is the last in the control stack and joined to it, set apart only by a heavier rule, so the stack stays one piece. Compare with M1’s gap.',
  },
  {
    id: 'm4',
    name: 'Stack top',
    look: { header: 'decks', stack: 'top' },
    idea: 'M1’s header, chevron and all. The panel button heads the control stack, above zoom in: the first thing the eye meets there, but it sits among the view buttons as if it were one of them.',
  },
]

// ---- the page -----------------------------------------------------------------

export default function HeaderMDemo() {
  const [theme, setTheme] = useTheme()
  const [width, setWidth] = useState<Width>(initialWidth)
  const pickWidth = (w: Width) => {
    setWidth(w)
    keepWidth(w)
  }
  const frame = { width: width === 'full' ? '100%' : `${width}px` }
  const wartable = theme === 'wartable'

  return (
    <PageTheme value={[theme, setTheme]}>
      <div data-theme={theme} className="quest-theme hd-page hc-page hm-page min-h-screen">
        <div className="hd-top sticky top-0 z-40 flex flex-wrap items-center gap-x-8 gap-y-2 border-b border-[var(--panel-border)] bg-[var(--panel)] px-6 py-3">
          <h1 className="quest-display text-2xl font-semibold">Header, merged</h1>
          <Choice label="Theme" options={THEMES.map((t) => [t.id, t.name] as const)} value={theme} onChange={setTheme} />
          <Choice label="Frame" options={WIDTHS.map((w) => [w, w === 'full' ? 'Full width' : `${w}px`] as const)} value={width} onChange={pickWidth} />
          <span className="text-[13px] text-[var(--ink-soft)]">Made-up data. The side-panel buttons work; tell us the good and bad ones by number.</span>
        </div>
        <main className={`space-y-14 px-6 py-8 ${width === 'full' ? '' : 'w-max min-w-full'}`}>
          {variants.map((v) => (
            <section key={v.id} id={v.id} className={`hm-v hm-${v.id} space-y-4`}>
              <div className="max-w-[860px]">
                <h2 className="quest-display text-[22px] font-semibold">
                  M{v.id.slice(1)} · {v.name}
                </h2>
                <p className="text-[14px] text-[var(--ink-soft)]">{v.idea}</p>
              </div>
              {previewsFor(v.look).map((c) => (
                <Figure key={c.label} c={c} look={v.look} frame={frame} wartable={wartable} />
              ))}
              <figure>
                <figcaption className="mb-1.5 text-[12px] font-semibold text-[var(--ink-soft)]">Ids elsewhere</figcaption>
                <div className="hd-frame relative" style={frame}>
                  <Elsewhere Id={Tag} wartable={wartable} />
                </div>
              </figure>
            </section>
          ))}
        </main>
      </div>
    </PageTheme>
  )
}
