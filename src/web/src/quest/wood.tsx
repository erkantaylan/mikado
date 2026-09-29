import { useId, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react'

// Flat cartoon wood (quest.css, "wood"): the way back is a red sign with cream letters, and every id
// (J7, Q142, R2) a small nailed tag of light wood with red letters.

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

type WoodProps = { shape: Shape; tone: 'red' | 'pale'; className?: string; children: ReactNode; title?: string; href?: string }

/**
 * A piece of wood in three tones (a lit top, the body, a shaded foot) with a dark outline, drawn to fit
 * whatever it holds: an inline SVG sized to the box after layout. A red sign carries a little grain; a
 * tag a nail at its left end. With href it is a link.
 */
function Wood({ shape, tone, className = '', children, title, href }: WoodProps) {
  const box = useRef<HTMLElement>(null)
  const [[w, h], setSize] = useState<[number, number]>([0, 0])
  const clip = useId()
  useLayoutEffect(() => {
    const el = box.current
    if (!el) return
    const ro = new ResizeObserver(() => setSize([el.offsetWidth, el.offsetHeight]))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  const sw = tone === 'red' ? 2 : 1.5
  const d = w > 0 ? outline(shape, w, h, sw) : ''
  const grain = tone === 'red' && w > 50
  const top = shape === 'arrow' ? h * 0.15 : 0 // where the body's top edge is
  const art = w > 0 && (
    <svg className="wood-art" width={w} height={h} viewBox={`0 0 ${w} ${h}`} aria-hidden>
      <clipPath id={clip}>
        <path d={d} />
      </clipPath>
      <g clipPath={`url(#${clip})`}>
        <rect className="wood-body" width={w} height={h} />
        <rect className="wood-lit" width={w} height={h * 0.4} />
        <rect className="wood-shade" y={h * 0.72} width={w} height={h * 0.28} />
        {grain && (
          // Two streaks of grain in the lit band, above the letters' caps.
          <path className="wood-grain" d={`M${w * 0.22},${top + 2.5} q${w * 0.1},-1.2 ${w * 0.22},0 M${w * 0.58},${top + 3} q${w * 0.08},1 ${w * 0.26},-0.4`} />
        )}
      </g>
      <path className="wood-edge" d={d} strokeWidth={sw} />
      {shape === 'tag' && (
        <>
          <circle className="wood-nail" cx={6.5} cy={h / 2} r={2} />
          <circle className="wood-nail-lit" cx={6} cy={h / 2 - 0.6} r={0.7} />
        </>
      )}
    </svg>
  )
  const cls = `wood wood-${shape} wood-${tone} ${className}`
  const text = <span className="wood-text">{children}</span>
  return href ? (
    <a ref={box as RefObject<HTMLAnchorElement>} href={href} className={cls} title={title}>
      {art}
      {text}
    </a>
  ) : (
    <span ref={box} className={cls} title={title}>
      {art}
      {text}
    </span>
  )
}

/**
 * The way back, top left of every page: a red arrow sign naming the place it leads to. Where there is
 * no way back (the Atlas itself) it is a plain red plank with the app's name.
 */
export function Sign({ label, href }: { label: string; href?: string }) {
  return href ? (
    <Wood shape="arrow" tone="red" className="quest-sign" href={href} title={`Back to ${label}`}>
      {label}
    </Wood>
  ) : (
    <Wood shape="plank" tone="red" className="quest-sign">
      {label}
    </Wood>
  )
}

/** An id (J7, Q142, R2): a nailed tag of light wood with red letters. struck: crossed out, as a removed quest. */
export function IdTag({ id, title, struck }: { id: string; title?: string; struck?: boolean }) {
  return (
    <Wood shape="tag" tone="pale" className={`quest-id${struck ? ' quest-id-struck' : ''}`} title={title}>
      {id}
    </Wood>
  )
}

const ID = /\b([JQR]\d+)\b/

/** Text with every id in it (J7, Q142, R2) set on its tag: a chronicle line, a glossary entry, a quest named by key. */
export function IdText({ text, struck }: { text: string; struck?: boolean }) {
  const parts = text.split(ID)
  if (parts.length === 1) return text
  // split with a capture group puts the ids at the odd places.
  return parts.map((p, k) => (k % 2 ? <IdTag key={k} id={p} struck={struck} /> : p))
}
