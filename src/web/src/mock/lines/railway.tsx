import { useEffect, useMemo, useRef } from 'react'
import type { EdgeProps } from '@xyflow/react'
import { questEdgePath, type QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'
import './railway.css'

// Supply Railway (2.3 / 1.4 — track with cross-ties; planned, laid and torn-up track).
//
// Every line is a length of narrow-gauge track laid along the edge's curve:
// A line from a finished deed is laid track (sleepers and two rails), the same build whatever the kind;
// only its colours tell the kind:
//   done      red lacquered sleepers, brass rails
//   held      bare timber sleepers, iron rails; built, waiting on the junction
//   spent     weathered grey sleepers, pewter rails; old track between two finished deeds
//   side      green sleepers, green-brass rails, from a finished side quest
// A line from an unfinished deed is planned track: sleepers along a surveyor's line, no rails yet:
//   locked    dark ink sleepers
//   side      green sleepers, from a side quest not yet done
//   cancelled torn-up track: a few scattered sleepers and a rail stub where the work stopped
//   bridge    track in a tunnel: two dashed rails in a dark casing, through the hidden deeds
// The sleepers are one wide stroke with a sparse butt-capped dasharray, so they stay square to the curve.
// The two rails are true offsets of the curve (sampled to a polyline), so the sleepers show between them.
// No arrows. The track itself is static SVG strokes. Small carts roll along two kinds of track: a cart loaded
// with gold ore where power is arriving (done), an empty one on old track between two finished deeds (spent).
// A cart moves by transform alone, which the compositor runs without repainting.

type Pt = [number, number]

const GAUGE = 3.2 // half the distance between the rails

function bezier(p: number[], t: number): Pt {
  const u = 1 - t
  const a = u * u * u, b = 3 * u * u * t, c = 3 * u * t * t, d = t * t * t
  return [a * p[0] + b * p[2] + c * p[4] + d * p[6], a * p[1] + b * p[3] + c * p[5] + d * p[7]]
}
function tangent(p: number[], t: number): Pt {
  const u = 1 - t
  const x = 3 * u * u * (p[2] - p[0]) + 6 * u * t * (p[4] - p[2]) + 3 * t * t * (p[6] - p[4])
  const y = 3 * u * u * (p[3] - p[1]) + 6 * u * t * (p[5] - p[3]) + 3 * t * t * (p[7] - p[5])
  const l = Math.hypot(x, y) || 1
  return [x / l, y / l]
}

/** The two rails: the curve offset by ±GAUGE, as polylines. */
function rails(p: number[]) {
  const len = Math.hypot(p[6] - p[0], p[7] - p[1])
  const n = Math.max(12, Math.min(80, Math.round(len / 6)))
  const l: string[] = []
  const r: string[] = []
  for (let i = 0; i <= n; i++) {
    const t = i / n
    const [x, y] = bezier(p, t)
    const [tx, ty] = tangent(p, t)
    const nx = -ty * GAUGE, ny = tx * GAUGE
    l.push(`${(x + nx).toFixed(1)},${(y + ny).toFixed(1)}`)
    r.push(`${(x - nx).toFixed(1)},${(y - ny).toFixed(1)}`)
  }
  return { left: 'M' + l.join('L'), right: 'M' + r.join('L') }
}

/** Where a cart is along the curve, as transform keyframes at even distances, so it rolls at a steady speed. */
function cartFrames(p: number[]) {
  const n = 64
  const pts: Pt[] = []
  const lens = [0]
  for (let i = 0; i <= n; i++) {
    pts.push(bezier(p, i / n))
    if (i) lens.push(lens[i - 1] + Math.hypot(pts[i][0] - pts[i - 1][0], pts[i][1] - pts[i - 1][1]))
  }
  const total = lens[n]
  const frames: Keyframe[] = []
  const steps = 32
  let j = 0
  for (let k = 0; k <= steps; k++) {
    const want = (total * k) / steps
    while (j < n - 1 && lens[j + 1] < want) j++
    const t = (j + (want - lens[j]) / (lens[j + 1] - lens[j] || 1)) / n
    const [x, y] = bezier(p, t)
    const [tx, ty] = tangent(p, t)
    frames.push({ transform: `translate(${x.toFixed(1)}px, ${y.toFixed(1)}px) rotate(${((Math.atan2(ty, tx) * 180) / Math.PI).toFixed(1)}deg)` })
  }
  return { frames, total }
}

const CART_SPEED = 40 // px per second

/** A small ore cart rolling along the track, under the cards at both ends; `loaded` heaps gold ore on it. */
function Cart({ p, seed, loaded }: { p: number[]; seed: number; loaded: boolean }) {
  const ref = useRef<SVGGElement>(null)
  useEffect(() => {
    const el = ref.current
    if (!el || matchMedia('(prefers-reduced-motion: reduce)').matches) return
    const { frames, total } = cartFrames(p)
    const duration = (total / CART_SPEED) * 1000
    // Carts on different lines start at different points, so they don't move in step.
    const anim = el.animate(frames, { duration, iterations: Infinity, easing: 'linear', delay: -((seed % 997) / 997) * duration })
    return () => anim.cancel()
  }, [p, seed])
  return (
    <g ref={ref} className="ln-railway-cart">
      <path d="M-6,-4.5 H6 L5,4.5 H-5 Z" className="ln-railway-cart-body" />
      <path d="M-3.5,-4.5 V4.5 M0,-4.5 V4.5 M3.5,-4.5 V4.5" className="ln-railway-cart-slat" />
      {loaded && (
        // Seen from above like the rest of the cart: a heap of gold lumps.
        <g className="ln-railway-cart-ore">
          <circle cx={-2.3} cy={-1.2} r={2} />
          <circle cx={2} cy={-1.4} r={1.8} />
          <circle cx={0} cy={1.6} r={2.1} />
        </g>
      )}
    </g>
  )
}

const hash = (s: string) => [...s].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7)

export default function Line(props: EdgeProps<QuestEdge>) {
  const { data } = props
  const flow = data!.flow
  const { path } = questEdgePath(props, useWarTable())
  const geo = useMemo(() => {
    const p = (path.match(/-?\d*\.?\d+(?:e[-+]?\d+)?/gi) ?? []).map(Number)
    if (p.length < 8) return null
    return { p, ...rails(p) }
  }, [path])
  if (!geo) return <path d={path} />

  // Laid: from a finished deed, main or side. Planned: from one still to do.
  const laid = data!.powered && flow !== 'cancelled' && flow !== 'bridge'
  const planned = !data!.powered && (flow === 'locked' || flow === 'side')
  return (
    <g
      className="ln-railway"
      data-flow={flow}
      data-track={laid ? 'laid' : planned ? 'planned' : undefined}
      data-dim={data!.dim || undefined}
      data-torn={data!.torn || undefined}
    >
      <path d={path} className="ln-railway-halo" />
      {flow !== 'bridge' && <path d={path} className="ln-railway-ties" />}
      {planned && <path d={path} className="ln-railway-survey" />}
      {(laid || flow === 'bridge') && (
        <>
          <path d={geo.left} className="ln-railway-rail-under" />
          <path d={geo.right} className="ln-railway-rail-under" />
          <path d={geo.left} className="ln-railway-rail" />
          <path d={geo.right} className="ln-railway-rail" />
        </>
      )}
      {flow === 'cancelled' && (
        <>
          <path d={geo.left} className="ln-railway-stub" />
          <path d={geo.right} className="ln-railway-stub" />
        </>
      )}
      {(flow === 'done' || flow === 'spent') && !data!.dim && <Cart p={geo.p} seed={hash(props.id)} loaded={flow === 'done'} />}
    </g>
  )
}
