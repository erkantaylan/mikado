import { EdgeLabelRenderer, type EdgeProps } from '@xyflow/react'
import { questEdgePath, type QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'
import seal from './ribbon/seal.webp'
import sealBroken from './ribbon/seal-broken.webp'
import './ribbon.css'

// Seal and Ribbon (3.4 — silk ribbons ending in a wax seal on the deed they open).
// Each line is a flat ribbon drawn as stacked strokes: a pale halo that lifts it off the map, a dark
// selvedge, a woven border, the body and a thread down the middle. The cloth tells the kind, and so does
// what is pressed onto it: a red wax seal just before the deed it opens (done), a blob of wax not yet
// sealed (held), a green wafer (side), a broken seal where an abandoned deed's ribbon was cut (cancelled).
// Everything is static: no animation, no filters.

// The war table tucks a line 12px under the target card and 12px (32px from a torn card) under the source.
const TUCK = 12
const TORN_TUCK = 32
const SEAL = 22 // px across; the seals on the cards are 36
const SEAL_BACK = TUCK + 14 // the seal's centre, along the ribbon from its tucked end: just clear of the card
const WAFER_BACK = TUCK + 34 // the side wafer sits further out, so it never lands on a seal at the same card
const MARK = 32 // the box each mark is drawn in
const CUT = 0.42 // where a cancelled ribbon is cut, along the part of it that shows

// Which layers a kind's ribbon has, bottom to top. The colours and widths live in ribbon.css.
const layers: Record<string, string[]> = {
  done: ['halo', 'edge', 'border', 'body', 'sheen'],
  held: ['halo', 'edge', 'border', 'body', 'stripe'],
  spent: ['halo', 'edge', 'body', 'weave'],
  locked: ['halo', 'edge', 'body', 'sheen'],
  side: ['halo', 'edge', 'body', 'weave'],
  cancelled: ['halo', 'edge', 'body', 'sheen'],
  bridge: ['under', 'halo', 'edge', 'body', 'sheen', 'slit'],
}

type Pt = { x: number; y: number }

/** The cubic from getBezierPath ("M x,y C x,y x,y x,y"), sampled into a polyline with running lengths. */
function sample(path: string) {
  const n = path.match(/-?\d*\.?\d+(?:e-?\d+)?/g)!.map(Number)
  const [x0, y0, x1, y1, x2, y2, x3, y3] = n
  const pts: Pt[] = []
  const len: number[] = []
  for (let i = 0; i <= 48; i++) {
    const t = i / 48
    const u = 1 - t
    const p = {
      x: u * u * u * x0 + 3 * u * u * t * x1 + 3 * u * t * t * x2 + t * t * t * x3,
      y: u * u * u * y0 + 3 * u * u * t * y1 + 3 * u * t * t * y2 + t * t * t * y3,
    }
    len.push(i ? len[i - 1] + Math.hypot(p.x - pts[i - 1].x, p.y - pts[i - 1].y) : 0)
    pts.push(p)
  }
  const total = len[len.length - 1]
  /** The point `d` px along the curve from its start. */
  const at = (d: number): Pt => {
    if (d >= total) return pts[pts.length - 1]
    const i = Math.max(1, len.findIndex((l) => l >= d))
    const f = (d - len[i - 1]) / (len[i] - len[i - 1] || 1)
    return { x: pts[i - 1].x + f * (pts[i].x - pts[i - 1].x), y: pts[i - 1].y + f * (pts[i].y - pts[i - 1].y) }
  }
  return { total, at }
}

export default function Line(props: EdgeProps<QuestEdge>) {
  const { id, data } = props
  const wt = useWarTable()
  const { path } = questEdgePath(props, wt)
  const flow = data!.flow
  const uid = `ln-ribbon-${id.replace(/[^a-zA-Z0-9_-]/g, '_')}`
  const curve = sample(path)
  const back = (d: number) => curve.at(Math.max(0, curve.total - d))

  // A cancelled ribbon is cut partway along the part of it that shows between the cards.
  const start = wt ? (data!.torn ? TORN_TUCK : TUCK) : 0
  const cutAt = start + CUT * Math.max(0, curve.total - start - (wt ? TUCK : 0))
  const cut = flow === 'cancelled' ? { strokeDasharray: `${cutAt} ${curve.total + 1}` } : undefined

  // The mark on the ribbon (seal, wax, wafer) is drawn in React Flow's edge-label layer, above every line but
  // under the cards, so a line into the same card can never paint over another line's seal.
  let at: Pt | null = null
  let mark = null
  if (flow === 'done') {
    at = back(SEAL_BACK)
    mark = (
      <>
        <ellipse cx={0.6} cy={1.8} rx={SEAL * 0.47} ry={SEAL * 0.43} className="ln-ribbon-shadow" />
        <image href={seal} x={-SEAL / 2} y={-SEAL / 2} width={SEAL} height={SEAL} />
        {/* the impression pressed into the wax: a small crown */}
        <path d="M-4.4 2.8h8.8l0.9-5.8-2.8 2.4-1.5-3.8-1.5 3.8-2.8-2.4z" className="ln-ribbon-crest" />
      </>
    )
  } else if (flow === 'held') {
    at = back(SEAL_BACK - 2)
    mark = (
      <>
        <defs>
          <radialGradient id={`${uid}-wax`} cx="0.36" cy="0.32" r="0.8">
            <stop offset="0" stopColor="#cf5d42" />
            <stop offset="0.5" stopColor="#932c1c" />
            <stop offset="1" stopColor="#5a140a" />
          </radialGradient>
        </defs>
        <ellipse cx={0.6} cy={1.5} rx={6.8} ry={6} className="ln-ribbon-shadow" />
        {/* an unpressed drop of wax: lumpy, glossy, no emblem */}
        <path
          d="M-6.4 -0.4c0.3-4 4-6.4 7.6-5.4 3 0.7 5.6 3 5.2 6.2-0.3 2.6-1.7 3.8-3.1 4.9-2.1 1.6-5.6 1.4-7.5-0.2-1.6-1.4-2.5-3.2-2.2-5.5z"
          fill={`url(#${uid}-wax)`}
          className="ln-ribbon-wax"
        />
        <ellipse cx={-2.2} cy={-2.6} rx={2} ry={1.1} className="ln-ribbon-gloss" />
      </>
    )
  } else if (flow === 'side') {
    at = back(WAFER_BACK)
    mark = (
      <>
        <circle cx={0.5} cy={1.2} r={5.4} className="ln-ribbon-shadow" />
        <circle r={5.2} className="ln-ribbon-wafer" />
        <circle r={2.6} className="ln-ribbon-wafer-ring" />
      </>
    )
  } else if (flow === 'cancelled') {
    at = curve.at(cutAt)
    const h = (SEAL * 38) / 48
    mark = (
      <>
        <ellipse cx={0.6} cy={1.6} rx={SEAL * 0.46} ry={h * 0.44} className="ln-ribbon-shadow" />
        <image href={sealBroken} x={-SEAL / 2} y={-h / 2} width={SEAL} height={h} />
      </>
    )
  }

  return (
    <g className="ln-ribbon" data-flow={flow} data-dim={data!.dim || undefined}>
      <g className="ln-ribbon-band">
        {/* past the cut, only a faint trace of where the ribbon ran */}
        {flow === 'cancelled' && <path d={path} fill="none" className="ln-ribbon-ghost" />}
        {layers[flow].map((l) => (
          <path key={l} d={path} fill="none" className={`ln-ribbon-${l}`} style={cut} />
        ))}
      </g>
      {at && (
        <EdgeLabelRenderer>
          <svg
            className="ln-ribbon-end"
            data-flow={flow}
            data-dim={data!.dim || undefined}
            width={MARK}
            height={MARK}
            viewBox={`${-MARK / 2} ${-MARK / 2} ${MARK} ${MARK}`}
            style={{ transform: `translate(${at.x - MARK / 2}px, ${at.y - MARK / 2}px)` }}
          >
            {mark}
          </svg>
        </EdgeLabelRenderer>
      )}
    </g>
  )
}
