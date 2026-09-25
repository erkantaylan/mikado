import { useMemo, type SVGProps } from 'react'
import type { EdgeProps } from '@xyflow/react'
import { questEdgePath, type QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'

// Couched Thread (3.3 — lines stitched through the map; the stitch tells the kind).
// Every line is sewn through the paper. The thread goes in under the source card, is tied off in a knot
// where it comes out (left), runs across the map held down by stitches, and is finished with a tight
// fly stitch (a V, pointing right) where it goes under the card it feeds. The kind is the stitch:
//   done       gold metal thread, twisted, couched down by close dark-silk tacks
//   held       the same gold, duller and thinner, with only a few loose tacks
//   spent      the thread pulled out: a pressed crease and the pairs of needle holes the tacks left
//   locked     a dark brown backstitch, needle holes in the tiny gaps
//   side       a row of green cross-stitches
//   cancelled  a running stitch cut half way, a loose curled tail, then only needle holes
//   bridge     a red tailor's basting stitch, long runs with holes at each end
// Everything is static geometry: the stitches are sampled along the curve once per layout.

type Pt = { x: number; y: number }
type Frame = Pt & { tx: number; ty: number; nx: number; ny: number }
type Flow = NonNullable<QuestEdge['data']>['flow']

const TUCK = 12
const TORN_TUCK = 32
const TEAR = 16 // how far the torn strip eats into an abandoned card, at the height a line leaves it

/** The curve `questEdgePath` draws, as an arc-length table: frame(s) gives the point, tangent and normal at s. */
function curve(path: string) {
  const n = path.match(/-?\d+(\.\d+)?(e-?\d+)?/g)?.map(Number)
  if (!n || n.length < 8) return null
  const [x0, y0, x1, y1, x2, y2, x3, y3] = n
  const N = 96
  const pts: Pt[] = []
  const cum: number[] = [0]
  for (let i = 0; i <= N; i++) {
    const t = i / N
    const u = 1 - t
    pts.push({
      x: u * u * u * x0 + 3 * u * u * t * x1 + 3 * u * t * t * x2 + t * t * t * x3,
      y: u * u * u * y0 + 3 * u * u * t * y1 + 3 * u * t * t * y2 + t * t * t * y3,
    })
    if (i > 0) cum.push(cum[i - 1] + Math.hypot(pts[i].x - pts[i - 1].x, pts[i].y - pts[i - 1].y))
  }
  const length = cum[N]
  const frame = (s: number): Frame => {
    s = Math.max(0, Math.min(length, s))
    let i = 1
    while (i < N && cum[i] < s) i++
    const a = pts[i - 1]
    const b = pts[i]
    const seg = cum[i] - cum[i - 1] || 1
    const k = (s - cum[i - 1]) / seg
    let tx = b.x - a.x
    let ty = b.y - a.y
    const d = Math.hypot(tx, ty) || 1
    tx /= d
    ty /= d
    // the normal points "down" the page on a left-to-right line
    return { x: a.x + (b.x - a.x) * k, y: a.y + (b.y - a.y) * k, tx, ty, nx: -ty, ny: tx }
  }
  return { length, frame }
}

const f = (v: number) => v.toFixed(1)
/** A dot drawn as a zero-length round-capped segment, so many dots share one path. */
const dot = (p: Pt) => `M${f(p.x)} ${f(p.y)}h0`
const seg = (a: Pt, b: Pt) => `M${f(a.x)} ${f(a.y)}L${f(b.x)} ${f(b.y)}`
const off = (p: Frame, along: number, across: number): Pt => ({
  x: p.x + p.tx * along + p.nx * across,
  y: p.y + p.ty * along + p.ny * across,
})
/** Positions every `step` from a to b (inclusive of a), centred so the run ends as evenly as it starts. */
function every(a: number, b: number, step: number) {
  const out: number[] = []
  if (b <= a) return out
  const n = Math.max(1, Math.floor((b - a) / step))
  const lead = (b - a - n * step) / 2
  for (let i = 0; i <= n; i++) out.push(a + lead + i * step)
  return out
}

type Look = {
  thread: string // the thread's colour
  edge: string // its darker edge (the underside of a round thread)
  w: number // thread width
  twist?: string // light diagonal ply marks (metal thread)
  tack?: { color: string; step: number } // couching stitches across the thread
  knot?: string // knot fill; its outline is `edge`
  fly?: string // the final V stitch's colour
}

const LOOK: Record<Flow, Look> = {
  done: { thread: '#dca62a', edge: '#6e4708', w: 4.6, twist: '#fff1bf', tack: { color: '#24160a', step: 11 }, knot: '#dca62a', fly: '#24160a' },
  held: { thread: '#b88a34', edge: '#5e3f10', w: 3.4, twist: '#f0dca4', tack: { color: '#3a2812', step: 30 }, knot: '#b88a34', fly: '#5e3f10' },
  spent: { thread: 'none', edge: 'none', w: 0 },
  locked: { thread: '#34241a', edge: '#150d06', w: 3.4, knot: '#34241a', fly: '#150d06' },
  side: { thread: '#4f7328', edge: '#26380f', w: 0, knot: '#4f7328', fly: '#26380f' },
  cancelled: { thread: '#4f3d27', edge: '#2a1e10', w: 1.9, knot: '#4f3d27' },
  bridge: { thread: '#a83f22', edge: '#4a180a', w: 3, knot: '#a2442a', fly: '#6e2412' },
}

const HOLE = '#1c1208'
const PAPER = 'rgba(252, 245, 226, 0.5)'
const round: SVGProps<SVGPathElement> = { fill: 'none', strokeLinecap: 'round', strokeLinejoin: 'round' }

export default function Line(props: EdgeProps<QuestEdge>) {
  const wt = useWarTable()
  const { path } = questEdgePath(props, wt)
  const { flow, dim, torn } = props.data!
  const look = LOOK[flow]

  const g = useMemo(() => {
    const c = curve(path)
    if (!c) return null
    const s0 = wt ? (torn ? TORN_TUCK - TEAR : TUCK) : 0 // where the line comes out from under the source card
    const s1 = c.length - (wt ? TUCK : 0) // where it goes under the target card
    const vis = s1 - s0
    const out = {
      tacks: '',
      twist: '',
      holes: '',
      crosses: ['', ''] as [string, string],
      fly: '',
      tail: '',
      fray: '',
      crease: '',
      knot: c.frame(s0 + 4),
      dash: undefined as string | undefined,
      dashOffset: 0,
      cut: s1,
      s0,
    }
    const F = c.frame

    // the fly stitch: a V whose point touches the target card, pointing into it
    const flyAt = (arm: number, spread: number) => {
      const e = F(s1 - 1.5)
      const tip = off(e, 0, 0)
      out.fly = `${seg(off(e, -arm, -spread), tip)}${seg(tip, off(e, -arm, spread))}`
    }

    if (flow === 'done' || flow === 'held') {
      const r = look.w / 2
      if (!dim) for (const s of every(s0 + 3, s1, 2.8)) {
        const p = F(s)
        out.twist += seg(off(p, -1.1, -r + 0.7), off(p, 1.1, r - 0.7))
      }
      for (const s of every(s0 + 13, s1 - 12, look.tack!.step)) {
        const p = F(s)
        const h = r + 2.2
        out.tacks += seg(off(p, 0, -h), off(p, 0, h))
      }
      flyAt(6, flow === 'done' ? 5.5 : 4.5)
    } else if (flow === 'spent') {
      out.crease = path
      for (const s of every(s0 + 10, s1 - 6, 12)) {
        const p = F(s)
        out.holes += dot(off(p, 0, -3.6)) + dot(off(p, 0, 3.6))
      }
      flyAt(5, 4)
    } else if (flow === 'locked') {
      // stitches 8 long with 2px gaps, from where the thread comes out; a hole in each gap
      out.dash = '5.4 4.6' // with round caps each stitch is ~8.8 long and the gaps close to a needle hole
      out.dashOffset = -(s0 + 1.7)
      for (let s = s0 + 9.4; s < s1 - 1; s += 10) out.holes += dot(F(s))
      flyAt(6, 5)
    } else if (flow === 'side') {
      // cross-stitches: each X spans `size`, the second leg laid over the first
      const size = 6.4
      for (const s of every(s0 + 8, s1 - 9, size + 1.4)) {
        const p = F(s)
        const h = size / 2
        out.crosses[0] += seg(off(p, -h, h), off(p, h, -h))
        out.crosses[1] += seg(off(p, -h, -h), off(p, h, h))
      }
      flyAt(5.5, 4.5)
    } else if (flow === 'cancelled') {
      // a running stitch, cut a little past half way; the rest is only needle holes
      const cut = s0 + vis * 0.5
      out.cut = cut
      out.dash = '4.5 3.5'
      out.dashOffset = -s0
      const e = F(cut)
      const tip = off(e, 5, 9)
      out.tail = `M${f(e.x)} ${f(e.y)}Q${f(off(e, 9, 1).x)} ${f(off(e, 9, 1).y)} ${f(tip.x)} ${f(tip.y)}`
      out.fray = seg(tip, off(e, 3.5, 12)) + seg(tip, off(e, 7.5, 11.5))
      for (let s = s0 + Math.ceil((cut + 6 - s0) / 8) * 8; s < s1 - 2; s += 8) out.holes += dot(F(s)) + dot(F(s + 4.5))
    } else if (flow === 'bridge') {
      // basting: long 14px runs, 6px gaps; a hole where each run goes in and comes out
      out.dash = '14 6'
      out.dashOffset = -s0
      for (let s = s0; s < s1 - 1; s += 20) out.holes += dot(F(s + 14.8)) + dot(F(s + 19.2))
      flyAt(6, 5)
    }
    return out
  }, [path, flow, dim, torn, wt, look])

  if (!g) return <></>
  const opacity = dim ? 0.15 : 1
  const shadow = 'translate(0.8 1.3)'
  const k = g.knot

  return (
    <g className="ln-stitch" data-flow={flow} style={{ opacity, pointerEvents: 'none' }}>
      {/* a pale lift of paper under every line, so it parts from the ink of the map */}
      <path
        d={path}
        {...round}
        stroke={PAPER}
        strokeWidth={flow === 'spent' ? 5 : flow === 'cancelled' ? 7 : flow === 'side' ? 12 : look.w + 7}
      />

      {flow === 'spent' && (
        <>
          {/* the crease the pulled thread pressed into the paper: a dark fold with a light lip below */}
          <path d={g.crease} {...round} stroke="rgba(255, 248, 230, 0.55)" strokeWidth={1.2} transform="translate(0 1.1)" />
          <path d={g.crease} {...round} stroke="rgba(70, 48, 24, 0.5)" strokeWidth={1.4} />
          <path d={g.holes} {...round} stroke="rgba(255, 248, 230, 0.9)" strokeWidth={2.8} transform="translate(0.5 0.7)" />
          <path d={g.holes} {...round} stroke={HOLE} strokeOpacity={0.72} strokeWidth={2.4} />
          <path d={g.fly} {...round} stroke="rgba(70, 48, 24, 0.5)" strokeWidth={1.4} />
        </>
      )}

      {(flow === 'done' || flow === 'held' || flow === 'locked') && (
        <>
          <path d={path} {...round} stroke="rgba(28, 16, 4, 0.38)" strokeWidth={look.w + 0.8} transform={shadow} strokeDasharray={g.dash} strokeDashoffset={g.dashOffset} strokeLinecap={flow === 'locked' ? 'round' : g.dash ? 'butt' : 'round'} />
          <path d={path} {...round} stroke={look.edge} strokeWidth={look.w + 1.4} strokeDasharray={g.dash} strokeDashoffset={g.dashOffset} strokeLinecap={flow === 'locked' ? 'round' : g.dash ? 'butt' : 'round'} />
          <path d={path} {...round} stroke={look.thread} strokeWidth={look.w - 0.4} strokeDasharray={g.dash} strokeDashoffset={g.dashOffset} strokeLinecap={flow === 'locked' ? 'round' : g.dash ? 'butt' : 'round'} />
          {g.twist && <path d={g.twist} {...round} stroke={look.twist} strokeOpacity={flow === 'done' ? 0.85 : 0.6} strokeWidth={0.9} />}
          {flow === 'locked' && (
            <>
              {/* a sheen along the top of each backstitch, and the needle holes in the gaps */}
              <path d={path} fill="none" strokeLinecap="round" stroke="#8a6848" strokeWidth={0.9} strokeDasharray="3.4 6.6" strokeDashoffset={g.dashOffset - 1} transform="translate(-0.3 -0.7)" />
              <path d={g.holes} {...round} stroke={HOLE} strokeWidth={1.5} />
            </>
          )}
          {g.tacks && (
            <>
              <path d={g.tacks} {...round} stroke="rgba(28, 16, 4, 0.35)" strokeWidth={1.9} transform={shadow} />
              <path d={g.tacks} {...round} stroke={look.tack!.color} strokeWidth={1.7} />
            </>
          )}
        </>
      )}

      {flow === 'side' && (
        <>
          <path d={g.crosses[0] + g.crosses[1]} {...round} stroke="rgba(20, 30, 6, 0.35)" strokeWidth={2.2} transform={shadow} />
          <path d={g.crosses[0]} {...round} stroke={look.edge} strokeWidth={2.1} />
          <path d={g.crosses[1]} {...round} stroke={look.thread} strokeWidth={2.1} />
        </>
      )}

      {(flow === 'cancelled' || flow === 'bridge') && (
        <>
          <path d={path} {...round} strokeLinecap="butt" stroke="rgba(28, 16, 4, 0.35)" strokeWidth={look.w + 0.6} strokeDasharray={flow === 'cancelled' ? cutDash(g.dash!, g.cut - g.s0) : g.dash} strokeDashoffset={g.dashOffset} transform={shadow} />
          <path
            d={path}
            {...round}
            strokeLinecap="butt"
            stroke={look.thread}
            strokeWidth={look.w}
            strokeDasharray={flow === 'cancelled' ? cutDash(g.dash!, g.cut - g.s0) : g.dash}
            strokeDashoffset={g.dashOffset}
          />
          {flow === 'bridge' && <path d={path} fill="none" stroke="#e08a62" strokeOpacity={0.55} strokeWidth={0.8} strokeDasharray={g.dash} strokeDashoffset={g.dashOffset - 1} transform="translate(-0.3 -0.6)" />}
          <path d={g.holes} {...round} stroke="rgba(255, 248, 230, 0.9)" strokeWidth={2.4} transform="translate(0.4 0.6)" />
          <path d={g.holes} {...round} stroke={HOLE} strokeOpacity={flow === 'cancelled' ? 0.6 : 0.9} strokeWidth={2} />
          {g.tail && (
            <>
              <path d={g.tail} {...round} stroke={look.thread} strokeWidth={look.w} />
              <path d={g.fray} {...round} stroke={look.thread} strokeWidth={0.8} />
            </>
          )}
        </>
      )}

      {/* the final stitch, pulled tight into the card */}
      {look.fly && g.fly && flow !== 'spent' && (
        <>
          <path d={g.fly} {...round} stroke={PAPER} strokeWidth={4.4} />
          <path d={g.fly} {...round} stroke={look.fly} strokeWidth={2} />
        </>
      )}

      {/* the knot where the needle first came up, beside the source card */}
      {look.knot && (
        <>
          <circle cx={k.x + 0.8} cy={k.y + 1.2} r={knotR(flow)} fill="rgba(28, 16, 4, 0.35)" />
          <circle cx={k.x} cy={k.y} r={knotR(flow)} fill={look.knot} stroke={look.edge} strokeWidth={1.3} />
          <path d={`M${f(k.x - knotR(flow) * 0.55)} ${f(k.y - 0.4)}q${f(knotR(flow) * 0.5)} ${f(-knotR(flow) * 0.7)} ${f(knotR(flow) * 1.05)} ${f(knotR(flow) * 0.1)}`} {...round} stroke="rgba(255, 244, 210, 0.75)" strokeWidth={0.9} />
        </>
      )}
    </g>
  )
}

function knotR(flow: Flow) {
  return flow === 'done' ? 4.4 : flow === 'cancelled' ? 3 : 3.6
}

/** A running-stitch dasharray that stops at arc length `cut` (dashes are measured from the path start). */
function cutDash(dash: string, cut: number) {
  const [on, gap] = dash.split(' ').map(Number)
  const n = Math.max(1, Math.floor(cut / (on + gap)))
  return `${Array.from({ length: n }, () => `${on} ${gap}`).join(' ')} 0 100000`
}
