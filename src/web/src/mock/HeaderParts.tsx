import { useContext, useId, useLayoutEffect, useRef, useState, type ComponentType, type ReactNode } from 'react'
import { Compass, PanelRightClose, PanelRightOpen, Search as SearchIcon, Sparkles } from 'lucide-react'
import { medal, stateColour, type Share } from '../quest/look'
import { ThemeMenu } from '../quest/theme'
import { PageTheme, all, pct, type Case, type Count } from './headerKit'

// Pieces the header comparison pages (HeaderDemo, HeaderCDemo, HeaderMDemo) share.

/** One count with its swatch, as in the real tally's legend. */
export function Item({ s }: { s: Share }) {
  return (
    <span data-zero={s.n === 0 || undefined} className="quest-tally-item flex items-center gap-1.5 whitespace-nowrap">
      <span data-share={s.key} className="quest-share quest-swatch size-2.5 shrink-0 rounded-sm" />
      <b className="tabular-nums">{s.n}</b> {s.label}
    </span>
  )
}

export function Legend({ t, className = '' }: { t: Count; className?: string }) {
  return (
    <div className={`quest-tally-legend flex gap-x-3.5 text-[12px] font-semibold text-[var(--ink-soft)] ${className}`}>
      {all(t).map((s) => (
        <Item key={s.key} s={s} />
      ))}
    </div>
  )
}

/** The tally's title, count and percentage, on one line. */
export function Figures({ t }: { t: Count }) {
  return (
    <div className="flex shrink-0 items-baseline gap-2 text-[12px] font-bold whitespace-nowrap">
      <span className="quest-tally-title text-[var(--ink-soft)]">{t.title}</span>
      <span className="quest-tally-count text-[15px] text-[var(--ink)] tabular-nums">
        {t.done}/{t.total}
      </span>
      <span className="quest-tally-pct text-[var(--ink-faint)] tabular-nums">{pct(t)}%</span>
    </div>
  )
}

export function Bar({ t, className = 'h-2.5' }: { t: Count; className?: string }) {
  const sealed = t.shares.find((s) => s.key === 'sealed')?.n ?? 0
  return (
    <div className={`quest-bar flex gap-[2px] overflow-hidden rounded-full bg-[var(--chip)] ${className}`} role="img" aria-label={`${t.title}: ${t.done} of ${t.total}`}>
      {t.shares
        .filter((s) => s.n > 0 && s.key !== 'sealed')
        .map((s) => (
          <span key={s.key} data-share={s.key} className="quest-share h-full" style={{ flexGrow: s.n, flexBasis: 0 }} title={`${s.n} ${s.label}`} />
        ))}
      {!!sealed && <span className="h-full" style={{ flexGrow: sealed, flexBasis: 0 }} />}
    </div>
  )
}

// The real search button binds Ctrl K for the whole page, so two dozen of them would each open a
// popup; this one only looks the part.
export function SearchTool({ look }: { look: 'full' | 'chip' | 'icon' }) {
  return (
    <button
      aria-label="Search journeys and quests"
      title="Search journeys and quests (Ctrl K)"
      className={`quest-tool hd-search flex h-10 shrink-0 items-center gap-2 rounded-md border border-[var(--panel-border)] text-[var(--ink-soft)] hover:text-[var(--ink)] ${
        look === 'icon' ? 'w-10 justify-center' : 'px-3'
      }`}
    >
      <SearchIcon size={17} />
      {look === 'full' && <span className="text-[14px]">Search</span>}
      {look !== 'icon' && <kbd className="rounded border border-[var(--panel-border)] px-1 font-mono text-[11px]">Ctrl K</kbd>}
    </button>
  )
}

function PanelTool() {
  const [open, setOpen] = useState(true)
  return (
    <button
      onClick={() => setOpen((o) => !o)}
      className="quest-tool grid size-10 place-items-center rounded-md border border-[var(--panel-border)] text-[var(--ink-soft)] hover:text-[var(--ink)]"
      aria-label={open ? 'Hide side panel' : 'Show side panel'}
      title={open ? 'Hide side panel' : 'Show side panel'}
    >
      {open ? <PanelRightClose size={18} /> : <PanelRightOpen size={18} />}
    </button>
  )
}

/** Search, the theme and (on a chart) the side panel, in that order. */
export function Tools({ c, search, className }: { c: Case; search: 'full' | 'chip' | 'icon'; className: string }) {
  const [theme, setTheme] = useContext(PageTheme)
  return (
    <div className={className}>
      <SearchTool look={search} />
      <ThemeMenu theme={theme} onChange={setTheme} />
      {c.chart && <PanelTool />}
    </div>
  )
}

/** A row of buttons, one of them pressed. */
export function Choice<T extends string>({ label, options, value, onChange }: { label: string; options: readonly (readonly [T, string])[]; value: T; onChange: (v: T) => void }) {
  return (
    <div className="flex items-center gap-2 text-[13px]">
      <span className="text-[var(--ink-soft)]">{label}</span>
      <div className="flex overflow-hidden rounded-md border border-[var(--panel-border)]">
        {options.map(([v, name]) => (
          <button
            key={v}
            onClick={() => onChange(v)}
            aria-pressed={value === v}
            className={`border-l border-[var(--panel-border)] px-3 py-1 first:border-l-0 ${
              value === v ? 'bg-[var(--chip)] font-semibold text-[var(--ink)]' : 'text-[var(--ink-soft)] hover:bg-[var(--chip)]'
            }`}
          >
            {name}
          </button>
        ))}
      </div>
    </div>
  )
}

// ---- wood (styles in headers-c.css, under .hc-page) -------------------------------------------------------------------------

type Shape = 'plank' | 'arrow' | 'tag'

/**
 * The outline of a piece of wood w by h, inset by half its stroke. A plank has rounded ends; an arrow
 * a head pointing left, wider than its shaft; a tag clipped corners on the left (by its nail) and a
 * slightly ragged right end.
 */
function outline(shape: Shape, w: number, h: number, sw: number) {
  const i = sw / 2
  const [l, t, r, b] = [i, i, w - i, h - i]
  if (shape === 'arrow') {
    const head = Math.round(h * 0.62)
    const [st, sb] = [h * 0.15, h * 0.85] // the shaft
    const rr = Math.min(5, (sb - st) / 2)
    return `M${l},${h / 2} L${head},${t} L${head},${st} L${r - rr},${st} Q${r},${st} ${r},${st + rr} L${r},${sb - rr} Q${r},${sb} ${r - rr},${sb} L${head},${sb} L${head},${b} Z`
  }
  if (shape === 'tag') {
    const c = 3
    return `M${l + c},${t} L${r - 1},${t + 0.5} L${r},${h * 0.38} L${r - 1.5},${h * 0.6} L${r - 0.5},${b} L${l + c},${b} L${l},${b - c} L${l},${t + c} Z`
  }
  const rr = Math.min(5, h / 4)
  return `M${l + rr},${t} L${r - rr},${t} Q${r},${t} ${r},${t + rr} L${r},${b - rr} Q${r},${b} ${r - rr},${b} L${l + rr},${b} Q${l},${b} ${l},${b - rr} L${l},${t + rr} Q${l},${t} ${l + rr},${t} Z`
}

/**
 * Flat, cartoon wood in three tones (a lit top, the body, a shaded bottom) with a dark outline, drawn
 * to fit whatever it holds: an inline SVG sized to the box after layout. tone: red (the way back) or
 * pale (an id). A sign carries a little grain; a tag a nail at its left end.
 */
export function Wood({ shape, tone, className = '', children, title }: { shape: Shape; tone: 'red' | 'pale'; className?: string; children: ReactNode; title?: string }) {
  const box = useRef<HTMLSpanElement>(null)
  const [[w, h], setSize] = useState<[number, number]>([0, 0])
  const clip = useId()
  useLayoutEffect(() => {
    const el = box.current
    if (!el) return
    const measure = () => setSize([el.offsetWidth, el.offsetHeight])
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  const sw = tone === 'red' ? 2 : 1.5
  const d = w > 0 ? outline(shape, w, h, sw) : ''
  const grain = tone === 'red' && w > 50
  const top = shape === 'arrow' ? h * 0.15 : 0 // where the body's top edge is
  return (
    <span ref={box} className={`hc-wood hc-wood-${shape} hc-${tone} ${className}`} title={title}>
      {w > 0 && (
        <svg className="hc-wood-art" width={w} height={h} viewBox={`0 0 ${w} ${h}`} aria-hidden>
          <clipPath id={clip}>
            <path d={d} />
          </clipPath>
          <g clipPath={`url(#${clip})`}>
            <rect className="hc-w-body" width={w} height={h} />
            <rect className="hc-w-lit" width={w} height={h * 0.4} />
            <rect className="hc-w-shade" y={h * 0.72} width={w} height={h * 0.28} />
            {grain && (
              // Two streaks of grain in the lit band, above the letters' caps; a knot would run into them.
              <path className="hc-w-grain" d={`M${w * 0.22},${top + 2.5} q${w * 0.1},-1.2 ${w * 0.22},0 M${w * 0.58},${top + 3} q${w * 0.08},1 ${w * 0.26},-0.4`} />
            )}
          </g>
          <path className="hc-w-edge" d={d} strokeWidth={sw} />
          {shape === 'tag' && (
            <>
              <circle className="hc-w-nail" cx={6.5} cy={h / 2} r={2} />
              <circle className="hc-w-nail-lit" cx={6} cy={h / 2 - 0.6} r={0.7} />
            </>
          )}
        </svg>
      )}
      <span className="hc-wood-text">{children}</span>
    </span>
  )
}

// ---- ids elsewhere: look-alikes of a chart card, side-panel rows and an Atlas row ----------------
// (their styles are in headers-c.css, under .hc-page)

/** How a page draws an id (J3, Q9, R2); plain is the classes of today's look in that place. */
export type IdView = ComponentType<{ k: string; plain?: string; link?: boolean }>

function Card({ Id }: { Id: IdView }) {
  return (
    <div className="hc-card relative w-[300px] shrink-0 rounded-lg border-2 py-2 pr-3 pl-5" style={{ background: 'var(--avail-plate)', borderColor: 'var(--avail)' }}>
      <span className="absolute -top-3 -left-3 z-10 grid size-9 place-items-center rounded-full border-2" style={medal.available}>
        <Sparkles size={16} />
      </span>
      <div className="relative flex min-w-0 flex-col gap-1.5">
        <div className="flex items-center gap-1.5 pl-2.5 text-[12px]">
          <Id k="Q16" plain="quest-key font-mono font-semibold text-[var(--ink-faint)]" />
        </div>
        <div className="hc-card-title text-[15px] leading-snug font-semibold">Buy a box of eggs from the corner shop</div>
        <div className="text-[12px] font-bold" style={{ color: stateColour.available }}>
          Open
        </div>
      </div>
    </div>
  )
}

function Row({ Id, k, petition, title }: { Id: IdView; k: string; petition?: boolean; title: string }) {
  return (
    <li className="hc-row flex items-center gap-1.5 overflow-hidden rounded-md border border-[var(--panel-border)] bg-[var(--plate)] px-2.5 py-2 text-[14px] whitespace-nowrap">
      <Id k={k} plain="quest-key font-mono text-[12px] font-semibold text-[var(--ink-faint)]" />
      {petition && <span className="quest-kind shrink-0 text-[12px] font-bold text-[var(--await)]">Petition</span>}
      <span className="min-w-0 truncate">{title}</span>
    </li>
  )
}

function Panel({ Id }: { Id: IdView }) {
  return (
    <div className="hc-panel w-[400px] shrink-0 rounded-md border border-[var(--panel-border)] bg-[var(--panel)] p-3">
      <div className="hc-panel-head mb-2 text-[13px] font-semibold">Open now</div>
      <ul className="space-y-1.5">
        <Row Id={Id} k="Q11" title="Heat a knob of butter in the pan until it foams but doesn’t brown" />
        <Row Id={Id} k="Q18" petition title="Ask your flatmate whether the cheddar is still good" />
      </ul>
    </div>
  )
}

function AtlasRow({ Id }: { Id: IdView }) {
  return (
    <div className="hc-atlasrow flex w-[380px] shrink-0 items-center gap-4 rounded-lg border-2 border-[var(--plate-border)] bg-[var(--plate)] px-4 py-2.5">
      <span className="grid size-9 shrink-0 place-items-center rounded-full border-2" style={{ borderColor: 'var(--gold)', color: 'var(--gold)' }}>
        <Compass size={17} />
      </span>
      <div className="min-w-0 flex-1">
        <div className="quest-display truncate text-[16px] leading-snug font-semibold">Kitchen</div>
        <div className="flex items-center gap-1 truncate text-[13px] text-[var(--ink-soft)]">
          <Id k="R2" /> · 3 underway
        </div>
      </div>
      <div className="flex w-[130px] items-center gap-2">
        <div className="h-2 flex-1 overflow-hidden rounded-full bg-[var(--chip)] ring-1 ring-[var(--plate-border)]">
          <div className="h-full bg-[var(--gold)]" style={{ width: '28%' }} />
        </div>
        <span className="text-[14px] font-semibold tabular-nums">5/18</span>
      </div>
    </div>
  )
}

/** A chart card, side-panel rows and an Atlas row, each with its ids drawn by Id. */
export function Elsewhere({ Id, wartable }: { Id: IdView; wartable: boolean }) {
  return (
    <div className="hd-strip hc-else relative isolate flex items-start gap-8 px-8 pt-7 pb-6">
      {wartable && <div className="wt-map wt-map-table" aria-hidden />}
      <Card Id={Id} />
      <Panel Id={Id} />
      <AtlasRow Id={Id} />
    </div>
  )
}
