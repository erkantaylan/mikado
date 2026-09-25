import type { EdgeProps } from '@xyflow/react'
import { useMemo, type JSX } from 'react'
import { questEdgePath, type QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'
import './chain.css'

// Hemp & Brass Chain (2.4 — rope for what is planned, brass chain once powered; segments laid along the curve).
// A stroke cannot bend a texture round a curve, so the line is laid like a PIXI rope: the curve is walked
// at equal arc-length steps and one small shape (a lay of rope, a chain link) is placed at each step,
// turned to the curve's tangent. Rope sits on one continuous dark core, so no seam shows on a bend.
// Static: no animation, no filters.

type Flow = NonNullable<QuestEdge['data']>['flow']
type P = [number, number]
type Bez = [P, P, P, P]

// ---- the curve -------------------------------------------------------------

const lerp = (a: P, b: P, t: number): P => [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t]

function at([p0, p1, p2, p3]: Bez, t: number): P {
  const u = 1 - t
  const w0 = u * u * u, w1 = 3 * u * u * t, w2 = 3 * u * t * t, w3 = t * t * t
  return [w0 * p0[0] + w1 * p1[0] + w2 * p2[0] + w3 * p3[0], w0 * p0[1] + w1 * p1[1] + w2 * p2[1] + w3 * p3[1]]
}

function angleAt([p0, p1, p2, p3]: Bez, t: number): number {
  const u = 1 - t
  const a = 3 * u * u, b = 6 * u * t, c = 3 * t * t
  const dx = a * (p1[0] - p0[0]) + b * (p2[0] - p1[0]) + c * (p3[0] - p2[0])
  const dy = a * (p1[1] - p0[1]) + b * (p2[1] - p1[1]) + c * (p3[1] - p2[1])
  return (Math.atan2(dy, dx) * 180) / Math.PI
}

// de Casteljau: the part of the curve between t0 and t1, as its own cubic
function sub(b: Bez, t0: number, t1: number): Bez {
  const left = (q: Bez, t: number): Bez => {
    const a = lerp(q[0], q[1], t), m = lerp(q[1], q[2], t), c = lerp(q[2], q[3], t)
    const d = lerp(a, m, t), e = lerp(m, c, t)
    return [q[0], a, d, lerp(d, e, t)]
  }
  const right = (q: Bez, t: number): Bez => {
    const a = lerp(q[0], q[1], t), m = lerp(q[1], q[2], t), c = lerp(q[2], q[3], t)
    const d = lerp(a, m, t), e = lerp(m, c, t)
    return [lerp(d, e, t), e, c, q[3]]
  }
  const l = left(b, t1)
  return t1 > 0 ? right(l, t0 / t1) : l
}

const f = (n: number) => Math.round(n * 100) / 100
const dOf = ([p0, p1, p2, p3]: Bez) => `M${f(p0[0])},${f(p0[1])} C${f(p1[0])},${f(p1[1])} ${f(p2[0])},${f(p2[1])} ${f(p3[0])},${f(p3[1])}`

type Place = { x: number; y: number; a: number }

// The curve measured by arc length: a table of 64 samples, walked by linear interpolation.
function measure(b: Bez) {
  const N = 64
  const ts: number[] = [0]
  const ss: number[] = [0]
  let prev = b[0]
  for (let i = 1; i <= N; i++) {
    const p = at(b, i / N)
    ss.push(ss[i - 1] + Math.hypot(p[0] - prev[0], p[1] - prev[1]))
    ts.push(i / N)
    prev = p
  }
  const len = ss[N]
  const tAt = (s: number) => {
    if (s <= 0) return 0
    if (s >= len) return 1
    let i = 1
    while (ss[i] < s) i++
    return ts[i - 1] + ((s - ss[i - 1]) / (ss[i] - ss[i - 1])) * (ts[i] - ts[i - 1])
  }
  const place = (s: number): Place => {
    const t = tAt(s)
    const [x, y] = at(b, t)
    return { x: f(x), y: f(y), a: f(angleAt(b, t)) }
  }
  // one shape every ~pitch px from s0 to s1, the pitch nudged so the run fills exactly
  const walk = (s0: number, s1: number, pitch: number): Place[] => {
    const n = Math.max(1, Math.round((s1 - s0) / pitch))
    const step = (s1 - s0) / n
    return Array.from({ length: n }, (_, i) => place(s0 + step * (i + 0.5)))
  }
  const d = (s0: number, s1: number) => dOf(sub(b, tAt(s0), tAt(s1)))
  return { len, place, walk, d }
}

// ---- materials ---------------------------------------------------------------

type Rope = {
  kind: 'rope'
  h: number // half width
  pitch: number // one lay (two strands) per pitch
  core: string // dark edge and grooves
  a: string // the two strands
  b: string
  glint: string
}
type Chain = {
  kind: 'chain'
  len: number // half length of a link
  ht: number // half height of a face-on link
  bar: number // thickness of the metal
  pitch: number
  metal: string
  edge: string // dark outline
  glint: string
  shade: string
  gap?: number // drop every gap-th link: a loose chain
}

const looks: Record<Flow, Rope | Chain> = {
  locked: { kind: 'rope', h: 3.3, pitch: 9, core: '#2e1f10', a: '#c29a62', b: '#94703f', glint: '#ecd5a3' },
  side: { kind: 'rope', h: 2, pitch: 6, core: '#1f3a0c', a: '#86ad55', b: '#557f2d', glint: '#c9e3a3' },
  cancelled: { kind: 'rope', h: 2.6, pitch: 8, core: '#3a2e22', a: '#a8977c', b: '#7f6e56', glint: '#d8ccb6' },
  done: { kind: 'chain', len: 6.8, ht: 4.4, bar: 2.4, pitch: 9.6, metal: '#e0a82e', edge: '#3f2604', glint: '#fff3c2', shade: '#9b6710' },
  held: { kind: 'chain', len: 6.4, ht: 4.1, bar: 2.2, pitch: 9, metal: '#a47a2c', edge: '#33200a', glint: '#d9bd7d', shade: '#6e4f17' },
  spent: { kind: 'chain', len: 4.8, ht: 3, bar: 1.5, pitch: 6.8, metal: '#8a8066', edge: '#3a3325', glint: '#c4bca4', shade: '#62593f' },
  bridge: { kind: 'chain', len: 5.8, ht: 3.8, bar: 1.9, pitch: 8.4, metal: '#6d6a66', edge: '#1e1c1a', glint: '#b9b4ac', shade: '#46433f', gap: 4 },
}

// One lay of rope, centred on 0 along the rope: two strands slanting across it, each with a glint.
function RopeLay({ id, r }: { id: string; r: Rope }) {
  const rx = r.h * 1.12
  const ry = r.pitch * 0.22
  const strand = (cx: number, fill: string) => (
    <g transform={`translate(${f(cx)} 0) rotate(-58)`}>
      <ellipse rx={f(rx)} ry={f(ry)} fill={fill} stroke={r.core} strokeWidth={0.55} />
      <path d={`M${f(-rx * 0.55)},${f(-ry * 0.38)} L${f(rx * 0.5)},${f(-ry * 0.38)}`} stroke={r.glint} strokeWidth={0.7} strokeLinecap="round" opacity={0.85} />
    </g>
  )
  return (
    <g id={id}>
      {strand(-r.pitch / 4, r.a)}
      {strand(r.pitch / 4, r.b)}
    </g>
  )
}

// A face-on link: an oval ring, dark rim, brass, a glint on its upper left and a shade on its lower right.
function FaceLink({ id, c }: { id: string; c: Chain }) {
  const rx = c.len - c.bar / 2
  const ry = c.ht - c.bar / 2
  const arc = (a0: number, a1: number) => {
    const p = (a: number) => `${f(Math.cos(a) * rx)},${f(Math.sin(a) * ry)}`
    return `M${p(a0)} A${f(rx)},${f(ry)} 0 0 1 ${p(a1)}`
  }
  return (
    <g id={id}>
      <ellipse rx={f(rx)} ry={f(ry)} fill="rgba(40,24,6,0.22)" stroke={c.edge} strokeWidth={f(c.bar + 1.3)} />
      <ellipse rx={f(rx)} ry={f(ry)} fill="none" stroke={c.metal} strokeWidth={c.bar} />
      <path d={arc(Math.PI * 1.08, Math.PI * 1.62)} fill="none" stroke={c.glint} strokeWidth={f(c.bar * 0.38)} strokeLinecap="round" />
      <path d={arc(Math.PI * 0.12, Math.PI * 0.62)} fill="none" stroke={c.shade} strokeWidth={f(c.bar * 0.4)} strokeLinecap="round" />
    </g>
  )
}

// An edge-on link: a narrow bar lying across the rings either side of it.
function EdgeLink({ id, c }: { id: string; c: Chain }) {
  const t = c.bar * 1.25
  return (
    <g id={id}>
      <rect x={f(-c.len)} y={f(-t / 2)} width={f(c.len * 2)} height={f(t)} rx={f(t / 2)} fill={c.metal} stroke={c.edge} strokeWidth={0.9} />
      <path d={`M${f(-c.len + t * 0.6)},${f(-t * 0.18)} L${f(c.len - t * 0.6)},${f(-t * 0.18)}`} stroke={c.glint} strokeWidth={f(t * 0.28)} strokeLinecap="round" />
    </g>
  )
}

const tf = (p: Place) => `translate(${p.x} ${p.y}) rotate(${p.a})`

// ---- end pieces (drawn in the line's own frame: +x runs towards the target) ----

// Where a rope leaves its card: a fat stopper knot, bound round twice.
function Knot({ r }: { r: Rope }) {
  const k = r.h * 1.55
  return (
    <g>
      <ellipse cx={0.6} cy={1.4} rx={k + 0.6} ry={k} fill="rgba(35,20,6,0.3)" />
      <ellipse rx={k + 0.6} ry={k} fill={r.a} stroke={r.core} strokeWidth={1.1} />
      <path d={`M${f(-k * 0.35)},${f(-k * 0.92)} Q${f(-k * 0.05)},0 ${f(-k * 0.35)},${f(k * 0.92)} M${f(k * 0.3)},${f(-k * 0.92)} Q${f(k * 0.6)},0 ${f(k * 0.3)},${f(k * 0.92)}`} fill="none" stroke={r.core} strokeWidth={0.8} />
      <circle cx={f(-k * 0.45)} cy={f(-k * 0.45)} r={f(k * 0.22)} fill={r.glint} />
    </g>
  )
}

// Where a rope enters its card: its end whipped with dark twine, slim beside the fat knot it leaves by.
function Whip({ r }: { r: Rope }) {
  const h = r.h + 0.9
  return (
    <g>
      <rect x={-3.2} y={f(-h)} width={6.4} height={f(2 * h)} rx={1.2} fill={r.core} />
      {[-1.9, -0.3, 1.3].map((x) => (
        <path key={x} d={`M${x},${f(-h + 0.5)} L${x + 0.8},${f(h - 0.5)}`} stroke={r.glint} strokeWidth={0.55} opacity={0.7} />
      ))}
    </g>
  )
}

// Where a chain leaves its card: a heavy round ring, bigger than a link.
function Ring({ c }: { c: Chain }) {
  const r = c.ht + 1.2
  const w = c.bar * 1.15
  return (
    <g>
      <circle cx={0.8} cy={1.8} r={r} fill="none" stroke="rgba(35,20,6,0.32)" strokeWidth={w + 1} />
      <circle r={r} fill="rgba(40,24,6,0.2)" stroke={c.edge} strokeWidth={f(w + 1.4)} />
      <circle r={r} fill="none" stroke={c.metal} strokeWidth={f(w)} />
      <path d={`M${f(-r * 0.87)},${f(-r * 0.5)} A${f(r)},${f(r)} 0 0 1 ${f(r * 0.1)},${f(-r)}`} fill="none" stroke={c.glint} strokeWidth={f(w * 0.4)} strokeLinecap="round" />
    </g>
  )
}

// Where a powered chain enters its card: a shackle, its bow taking the last link and its pin at the card.
function Shackle({ c }: { c: Chain }) {
  const h = c.ht + 1.4
  const w = h * 1.3
  const bow = `M${f(w)},${f(-h)} L0,${f(-h)} A${f(h)},${f(h)} 0 0 0 0,${f(h)} L${f(w)},${f(h)}`
  return (
    <g>
      <path d={bow} transform="translate(0.8 1.8)" fill="none" stroke="rgba(35,20,6,0.32)" strokeWidth={c.bar + 1.2} />
      <path d={bow} fill="none" stroke={c.edge} strokeWidth={f(c.bar * 1.2 + 1.4)} strokeLinecap="round" />
      <path d={bow} fill="none" stroke={c.metal} strokeWidth={f(c.bar * 1.2)} strokeLinecap="round" />
      <path d={`M${f(w * 0.5)},${f(-h)} L0,${f(-h)} A${f(h)},${f(h)} 0 0 0 ${f(-h * 0.7)},${f(-h * 0.7)}`} fill="none" stroke={c.glint} strokeWidth={0.8} strokeLinecap="round" />
      {/* the pin through both ends, with its two heads */}
      <rect x={f(w - 1.3)} y={f(-h - 2.4)} width={2.6} height={f(2 * h + 4.8)} rx={1.3} fill={c.metal} stroke={c.edge} strokeWidth={0.9} />
      <circle cx={f(w)} cy={f(-h - 2.2)} r={1.8} fill={c.metal} stroke={c.edge} strokeWidth={0.9} />
    </g>
  )
}

// Where a powered chain waits on other deeds: a small padlock hung on it, kept upright.
function Padlock({ c }: { c: Chain }) {
  return (
    <g>
      <rect x={-3.8} y={-1.2} width={9} height={8} rx={1.6} fill="rgba(35,20,6,0.3)" />
      <path d="M-2.4,-1.6 V-4 A2.4,2.4 0 0 1 2.4,-4 V-1.6" fill="none" stroke={c.edge} strokeWidth={2.6} />
      <path d="M-2.4,-1.6 V-4 A2.4,2.4 0 0 1 2.4,-4 V-1.6" fill="none" stroke="#8d8577" strokeWidth={1.3} />
      <rect x={-4.5} y={-2} width={9} height={7.6} rx={1.6} fill={c.metal} stroke={c.edge} strokeWidth={1} />
      <path d="M-3.2,-0.6 H3" stroke={c.glint} strokeWidth={0.8} strokeLinecap="round" />
      <circle cx={0} cy={1.5} r={1.2} fill={c.edge} />
      <path d="M0,1.8 V3.8" stroke={c.edge} strokeWidth={1} strokeLinecap="round" />
    </g>
  )
}

// A cut: the strands splay out in a short brush. +x is the way the loose end points.
function Fray({ r }: { r: Rope }) {
  const h = r.h
  const fibre = (y0: number, x1: number, y1: number, c: string, w = 1.1) => (
    <path d={`M-1.5,${f(y0)} Q${f(x1 * 0.55)},${f(y0 * 1.1)} ${f(x1)},${f(y1)}`} fill="none" stroke={c} strokeWidth={w} strokeLinecap="round" />
  )
  return (
    <g>
      {fibre(-h * 0.8, 4, -h * 2.2, r.core, 1.2)}
      {fibre(-h * 0.45, 5.5, -h * 1.4, r.a)}
      {fibre(-h * 0.1, 6.5, -h * 0.3, r.glint, 0.9)}
      {fibre(h * 0.2, 6, h * 0.8, r.b)}
      {fibre(h * 0.5, 5, h * 1.6, r.a)}
      {fibre(h * 0.85, 3.8, h * 2.3, r.core, 1.2)}
    </g>
  )
}

// ---- the line ----------------------------------------------------------------

const TUCK = 12 // matches graph.tsx: how far a line runs in under a card
const TORN_TUCK = 32

export default function Line(props: EdgeProps<QuestEdge>): JSX.Element {
  const { id, sourceX, sourceY, targetX, targetY, data } = props
  const wt = useWarTable()
  const flow = data!.flow
  const torn = data!.torn
  const look = looks[flow]
  const uid = `ln-chain-${id.replace(/[^a-zA-Z0-9_-]/g, '_')}`

  const g = useMemo(() => {
    const { path } = questEdgePath(props, wt)
    const n = (path.match(/-?\d*\.?\d+(?:e[-+]?\d+)?/gi) ?? []).map(Number)
    const b: Bez = [[n[0], n[1]], [n[2], n[3]], [n[4], n[5]], [n[6], n[7]]]
    // a gentle sag, as rope and chain hang between two pins; the ends stay put, so both still tuck under
    const span = Math.hypot(b[3][0] - b[0][0], b[3][1] - b[0][1])
    const sag = Math.min(7, span * 0.035)
    b[1] = [b[1][0], b[1][1] + sag]
    b[2] = [b[2][0], b[2][1] + sag]
    const m = measure(b)
    const s0 = wt ? (torn ? TORN_TUCK : TUCK) : 0 // where the line comes out from under the source card
    const s1 = m.len - (wt ? TUCK : 0) // where it goes in under the target card
    const pitch = look.pitch
    // shapes run a little way in under the cards at both ends, so nothing stops short of the paper
    if (flow === 'cancelled') {
      // cut a short way out of the torn card: a stub, a gap, then the rest of the rope
      const cut = s0 + Math.min(26, (s1 - s0) * 0.28)
      const gap = Math.min(24, (s1 - s0) * 0.3)
      return {
        m,
        s0,
        s1,
        runs: [
          { d: m.d(Math.max(0, s0 - 6), cut), segs: m.walk(s0 - 4, cut, pitch) },
          { d: m.d(cut + gap, m.len), segs: m.walk(cut + gap, s1 + 4, pitch) },
        ],
        frays: [{ ...m.place(cut), flip: false }, { ...m.place(cut + gap), flip: true }],
      }
    }
    return { m, s0, s1, runs: [{ d: m.d(0, m.len), segs: m.walk(s0 - 4, s1 + 4, pitch) }], frays: [] }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceX, sourceY, targetX, targetY, flow, torn, wt])

  const cls = `ln-chain ln-chain-${look.kind}${data!.dim ? ' ln-chain-dim' : ''}`
  const all = g.runs[g.runs.length - 1].d

  if (look.kind === 'rope') {
    const w = look.h * 2 + 1.4
    return (
      <g className={cls} data-flow={flow}>
        <defs>
          <RopeLay id={`${uid}-lay`} r={look} />
        </defs>
        <path d={all} className="react-flow__edge-interaction" fill="none" strokeOpacity={0} strokeWidth={18} />
        {g.runs.map((run, i) => (
          <g key={i}>
            <path d={run.d} className="ln-chain-shadow" strokeWidth={w} />
            <path d={run.d} className="ln-chain-halo" strokeWidth={w + 3} />
            <path d={run.d} fill="none" stroke={look.core} strokeWidth={w} strokeLinecap="round" />
            {run.segs.map((p, j) => (
              <use key={j} href={`#${uid}-lay`} transform={tf(p)} />
            ))}
          </g>
        ))}
        {g.frays.map((p, i) => (
          <g key={i} transform={`${tf(p)}${p.flip ? ' scale(-1 1)' : ''}`}>
            <Fray r={look} />
          </g>
        ))}
        {flow !== 'cancelled' && (
          <g className="ln-chain-end" transform={tf(g.m.place(g.s0 + look.h + 2))}>
            <Knot r={look} />
          </g>
        )}
        {flow !== 'cancelled' && (
          <g className="ln-chain-end" transform={tf(g.m.place(g.s1 - 4.5))}>
            <Whip r={look} />
          </g>
        )}
      </g>
    )
  }

  // chain: face-on and edge-on links in turn; a loose chain drops every gap-th link
  const faces: Place[] = []
  const edges: Place[] = []
  g.runs[0].segs.forEach((p, i) => {
    if (look.gap && i % look.gap === look.gap - 1) return
    ;(i % 2 === 0 ? faces : edges).push(p)
  })
  const w = look.ht * 1.3
  const ringAt = g.s0 + look.ht + 1.5
  return (
    <g className={cls} data-flow={flow}>
      <defs>
        <FaceLink id={`${uid}-f`} c={look} />
        <EdgeLink id={`${uid}-e`} c={look} />
      </defs>
      <path d={all} className="react-flow__edge-interaction" fill="none" strokeOpacity={0} strokeWidth={18} />
      {!look.gap && <path d={g.runs[0].d} className="ln-chain-shadow" strokeWidth={w} />}
      <path d={g.runs[0].d} className="ln-chain-halo" strokeWidth={w * 2} />
      {faces.map((p, j) => (
        <use key={`f${j}`} href={`#${uid}-f`} transform={tf(p)} />
      ))}
      {edges.map((p, j) => (
        <use key={`e${j}`} href={`#${uid}-e`} transform={tf(p)} />
      ))}
      <g className="ln-chain-end" transform={tf(g.m.place(ringAt))}>
        <Ring c={look} />
      </g>
      {flow === 'held' ? (
        <g className="ln-chain-end" transform={(() => { const p = g.m.place(g.s1 - 9); return `translate(${p.x} ${p.y})` })()}>
          <Padlock c={look} />
        </g>
      ) : (
        <g className="ln-chain-end" transform={tf(g.m.place(g.s1 - (look.ht + 1.4) * 1.3 - 2.5))}>
          <Shackle c={look} />
        </g>
      )}
    </g>
  )
}
