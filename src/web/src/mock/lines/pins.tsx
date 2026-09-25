import type { EdgeProps } from '@xyflow/react'
import type { JSX } from 'react'
import type { QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'
import './pins.css'

// Thread and Pins (2.1 — thread pulled taut from a brass map pin).
// Every line is a length of thread, held by a pin just outside the card it leaves and running in
// under the paper of the card it opens. The pin shows the direction; what the thread is made of
// shows the state. Static: no animation, no filters.

type Flow = NonNullable<QuestEdge['data']>['flow']
type PinKind = 'brass' | 'ring' | 'pewter' | 'iron' | 'knot'

type Thread = {
  colour: string
  width: number
  strand: string // the lighter strand wound round it, drawn broken so it reads as a twist
  strandWidth: number
  dash?: string // a broken thread (stitches, or the dashed bridge)
  pin: PinKind
  halo: number // extra width of the pale halo each side
}

const threads: Record<Flow, Thread> = {
  done: { colour: '#b3202a', width: 3.8, strand: '#f07a7f', strandWidth: 1.5, pin: 'brass', halo: 2 },
  held: { colour: '#8c2630', width: 2.8, strand: '#c9555d', strandWidth: 1.1, pin: 'ring', halo: 1.8 },
  spent: { colour: '#b9736e', width: 2.4, strand: '#ecc3be', strandWidth: 0.9, pin: 'pewter', halo: 1.4 },
  locked: { colour: '#3d2914', width: 3.4, strand: '#a3804d', strandWidth: 1.2, pin: 'iron', halo: 1.4 },
  side: { colour: '#3a6a1c', width: 2.8, strand: '#86b35a', strandWidth: 0, dash: '2 4', pin: 'knot', halo: 1.8 },
  cancelled: { colour: '#8e2a2a', width: 2.2, strand: '#c86a66', strandWidth: 0.9, pin: 'pewter', halo: 1.6 },
  bridge: { colour: '#26345e', width: 2.8, strand: '#7f90c4', strandWidth: 1, dash: '8 5', pin: 'iron', halo: 1.8 },
}

const TUCK = 12 // how far the thread runs in under a card
const TORN_TUCK = 32 // an abandoned card is torn on its right side: start past the tear
const PIN = 9 // the pin sits this far outside the card's right edge, clear of the paper

type Pt = [number, number]

// A taut thread: from the pin, nearly straight to the target card, easing in only enough to go
// under the card's left edge level rather than skimming its corner.
function curve(a: Pt, b: Pt) {
  const dx = b[0] - a[0]
  const k = Math.min(Math.abs(dx) * 0.4, 30)
  const c1: Pt = [a[0] + k, a[1]]
  const c2: Pt = [b[0] - k, b[1]]
  const at = (t: number): Pt => {
    const u = 1 - t
    const w = [u * u * u, 3 * u * u * t, 3 * u * t * t, t * t * t]
    return [
      w[0] * a[0] + w[1] * c1[0] + w[2] * c2[0] + w[3] * b[0],
      w[0] * a[1] + w[1] * c1[1] + w[2] * c2[1] + w[3] * b[1],
    ]
  }
  return { d: `C ${c1[0]} ${c1[1]} ${c2[0]} ${c2[1]} ${b[0]} ${b[1]}`, at }
}

function Pin({ kind, x, y }: { kind: PinKind; x: number; y: number }) {
  switch (kind) {
    case 'brass':
      return (
        <g className="ln-pins-pin">
          <ellipse cx={x + 1.8} cy={y + 2.4} rx={6.2} ry={5.6} fill="rgba(35,20,6,0.38)" />
          <circle cx={x} cy={y} r={6} fill="#d9a441" stroke="#6b4a14" strokeWidth={1.4} />
          <circle cx={x + 0.9} cy={y + 0.9} r={3.2} fill="#b98526" />
          <circle cx={x - 2} cy={y - 2.1} r={1.6} fill="#fff6d8" />
        </g>
      )
    case 'ring':
      // not pushed home: a hollow ring standing proud, with a longer shadow
      return (
        <g className="ln-pins-pin">
          <circle cx={x + 3} cy={y + 3.8} r={5.4} fill="none" stroke="rgba(35,20,6,0.32)" strokeWidth={2.4} />
          <circle cx={x} cy={y} r={5} fill="none" stroke="#6b4a14" strokeWidth={3.8} />
          <circle cx={x} cy={y} r={5} fill="none" stroke="#d9a441" strokeWidth={2.1} />
          <circle cx={x - 2.8} cy={y - 2.8} r={1} fill="#fff6d8" />
        </g>
      )
    case 'pewter':
      return (
        <g className="ln-pins-pin">
          <ellipse cx={x + 1.4} cy={y + 1.9} rx={4.2} ry={3.8} fill="rgba(35,20,6,0.28)" />
          <circle cx={x} cy={y} r={4} fill="#9a9284" stroke="#544c40" strokeWidth={1.1} />
          <circle cx={x - 1.2} cy={y - 1.3} r={1} fill="#ece7dc" />
        </g>
      )
    case 'iron':
      return (
        <g className="ln-pins-pin">
          <ellipse cx={x + 1.2} cy={y + 1.6} rx={3.8} ry={3.4} fill="rgba(35,20,6,0.3)" />
          <circle cx={x} cy={y} r={4} fill="#3a2c1e" stroke="#1c140b" strokeWidth={1} />
          <circle cx={x - 1} cy={y - 1.1} r={0.9} fill="#a8927a" />
        </g>
      )
    case 'knot':
      // tied on, not pinned: a small knot with two loose ends
      return (
        <g className="ln-pins-pin" stroke="#3f6b22" strokeLinecap="round" fill="none">
          <path d={`M ${x - 1} ${y + 1} l -5 6 M ${x - 1} ${y + 1} l -7 2.5`} strokeWidth={1.4} />
          <circle cx={x} cy={y} r={3.2} fill="#3a6a1c" stroke="#24410f" strokeWidth={1} />
          <circle cx={x - 0.8} cy={y - 0.8} r={0.7} fill="#b5d98e" stroke="none" />
        </g>
      )
  }
}

// Three short splayed strands where a thread was cut. dir is +1 when the loose end points right.
function Fray({ x, y, dir, colour }: { x: number; y: number; dir: number; colour: string }) {
  const s = 8 * dir
  return (
    <path
      d={`M ${x} ${y} l ${s} -4.5 M ${x} ${y} l ${s * 1.15} 0.5 M ${x} ${y} l ${s} 4.8`}
      stroke={colour}
      strokeWidth={1.5}
      strokeLinecap="round"
      fill="none"
    />
  )
}

// One run of thread: its shadow on the board, a pale halo parting it from the map, the thread and its strand.
function Strand({ d, t, dash }: { d: string; t: Thread; dash?: string }) {
  const dasharray = dash ?? t.dash
  return (
    <>
      <path d={d} className="ln-pins-shadow" strokeWidth={t.width} strokeDasharray={dasharray} />
      <path d={d} className="ln-pins-halo" strokeWidth={t.width + t.halo * 2} strokeDasharray={dasharray} />
      <path d={d} fill="none" stroke={t.colour} strokeWidth={t.width} strokeDasharray={dasharray} strokeLinecap="round" />
      {t.strandWidth > 0 && !dasharray && (
        <path d={d} className="ln-pins-twist" stroke={t.strand} strokeWidth={t.strandWidth} />
      )}
    </>
  )
}

export default function Line(props: EdgeProps<QuestEdge>): JSX.Element {
  const { sourceX, sourceY, targetX, targetY, data } = props
  const wt = useWarTable()
  const flow = data!.flow
  const t = threads[flow]
  const tuck = wt ? TUCK : 0
  const start = sourceX - (wt && data!.torn ? TORN_TUCK : tuck)
  const pin: Pt = [sourceX + PIN, sourceY]
  const end: Pt = [targetX, targetY]
  const cls = `ln-pins${data!.dim ? ' ln-pins-dim' : ''}`

  if (flow === 'cancelled') {
    // The thread is cut: a frayed stub hangs from the pin, another from the card it once opened,
    // and only a faint pencilled trace remembers where it ran.
    const stubEnd: Pt = [pin[0] + 22, sourceY]
    const tailStart: Pt = [targetX - 20, targetY]
    const trace = curve([stubEnd[0] + 6, stubEnd[1]], [tailStart[0] - 6, tailStart[1]])
    return (
      <g className={cls} data-flow={flow}>
        <path d={`M ${stubEnd[0] + 6} ${stubEnd[1]} ${trace.d}`} className="ln-pins-trace" />
        <Strand d={`M ${start} ${sourceY} L ${stubEnd[0]} ${stubEnd[1]}`} t={t} />
        <Strand d={`M ${tailStart[0]} ${tailStart[1]} L ${targetX + tuck} ${targetY}`} t={t} />
        <Fray x={stubEnd[0]} y={stubEnd[1]} dir={1} colour={t.colour} />
        <Fray x={tailStart[0]} y={tailStart[1]} dir={-1} colour={t.colour} />
        <Pin kind={t.pin} x={pin[0]} y={pin[1]} />
      </g>
    )
  }

  const c = curve(pin, end)
  const d = `M ${start} ${sourceY} L ${pin[0]} ${pin[1]} ${c.d} L ${targetX + tuck} ${targetY}`
  const mid = c.at(0.5)
  return (
    <g className={cls} data-flow={flow}>
      <Strand d={d} t={t} />
      {flow === 'bridge' && (
        // a brass eyelet where the thread passes under the hidden cards
        <g className="ln-pins-pin">
          <circle cx={mid[0] + 1.2} cy={mid[1] + 1.6} r={5} fill="rgba(35,20,6,0.3)" />
          <circle cx={mid[0]} cy={mid[1]} r={4.6} fill="#2a1d10" stroke="#d9a441" strokeWidth={2.2} />
          <circle cx={mid[0]} cy={mid[1]} r={5.7} fill="none" stroke="#6b4a14" strokeWidth={0.8} />
        </g>
      )}
      <Pin kind={t.pin} x={pin[0]} y={pin[1]} />
    </g>
  )
}
