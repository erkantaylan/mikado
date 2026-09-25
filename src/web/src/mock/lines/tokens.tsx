import { getBezierPath, useStore, type EdgeProps, type ReactFlowState } from '@xyflow/react'
import type { ReactNode } from 'react'
import { useWarTable } from '../../quest/theme'
import type { QuestEdge } from '../../quest/graph'
import './tokens.css'

// Gate Tokens (5.3 — simple lines; a pictogram token where each line meets its card).
// The line body only carries width and colour. The meaning sits in a round brass-rimmed token at the
// card's door, just outside its left edge: key (open), hourglass (held), padlock (locked), gem (side),
// ellipsis (hidden steps), a small tick (spent). A solid port dot marks where the line leaves its source,
// so direction reads dot → token. Several lines entering one card fan out along its left edge, so their
// tokens stack into a readable column instead of piling on one point.

type Flow = NonNullable<QuestEdge['data']>['flow']

const TUCK = 12
const TORN_TUCK = 32
const R = 9 // token radius
const GAP = 5 // between the token and the card edge
const PITCH = 21 // token spacing when lines fan into one card

// ---- the glyphs, drawn in a box of about ±4.5 around the token centre --------------------------------

const KEY = (
  <g fill="none" stroke="currentColor" strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round">
    <circle cx={-2.6} cy={0} r={2.1} />
    <path d="M-0.5 0H4.6M3 0v2.2M4.6 0v1.6" />
  </g>
)
const HOURGLASS = (
  <g fill="currentColor">
    <path d="M-3.6 -4.3h7.2v1.1h-7.2zM-3.6 3.2h7.2v1.1h-7.2z" />
    <path d="M-2.7 -3.2h5.4c0 1.8-1.2 2.6-2.1 3.2c.9.6 2.1 1.4 2.1 3.2h-5.4c0-1.8 1.2-2.6 2.1-3.2c-.9-.6-2.1-1.4-2.1-3.2z" fillOpacity={0.35} />
    <path d="M-1.6 -1.6h3.2L0 -0.2zM-2 3.1c.3-1 1.2-1.6 2-1.8c.8.2 1.7.8 2 1.8z" />
  </g>
)
const PADLOCK = (
  <g>
    <path d="M-2.2 -0.6v-1.6a2.2 2.2 0 0 1 4.4 0v1.6" fill="none" stroke="currentColor" strokeWidth={1.4} />
    <rect x={-3.5} y={-0.8} width={7} height={5.2} rx={0.9} fill="currentColor" />
    <path d="M0 0.8v1.8" stroke="var(--ln-tokens-face)" strokeWidth={1.2} strokeLinecap="round" />
  </g>
)
const GEM = (
  <g strokeLinejoin="round">
    <path d="M0 -4.2L3.9 -0.4L0 4.4L-3.9 -0.4z" fill="currentColor" />
    <path d="M-3.9 -0.4h7.8M-1.5 -0.4L0 -4.2L1.5 -0.4L0 4.4z" fill="none" stroke="var(--ln-tokens-face)" strokeWidth={0.7} strokeOpacity={0.7} />
  </g>
)
const DOTS = (
  <g fill="currentColor">
    <circle cx={-3.1} cy={0} r={1.25} />
    <circle cx={0} cy={0} r={1.25} />
    <circle cx={3.1} cy={0} r={1.25} />
  </g>
)
const TICK = <path d="M-3 0.2L-0.9 2.4L3.2 -2.6" fill="none" stroke="currentColor" strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" />

// A wax seal cracked in two, for a line from (or into) an abandoned deed.
const CRACK = 'L-0.9,-3.6 L1.1,-1 L-0.7,2.2 L0.7'
function Seal({ x, y, big }: { x: number; y: number; big: boolean }) {
  return (
    <g className="ln-tokens-token ln-tokens-seal" transform={`translate(${x} ${y}) scale(${big ? 1.45 : 1}) rotate(-14)`}>
      <circle r={R + 1.4} cy={0.9} className="ln-tokens-shadow" />
      <path transform="translate(-1.1 0.4)" d={`M0.5,-${R} ${CRACK},${R} A${R} ${R} 0 1 1 0.5,-${R}Z`} className="ln-tokens-wax" />
      <path transform="translate(1.1 -0.4) rotate(9)" d={`M0.5,-${R} ${CRACK},${R} A${R} ${R} 0 1 0 0.5,-${R}Z`} className="ln-tokens-wax" />
      <circle r={R - 3} className="ln-tokens-wax-ring" />
    </g>
  )
}

const glyph: Partial<Record<Flow, ReactNode>> = { done: KEY, held: HOURGLASS, locked: PADLOCK, side: GEM, bridge: DOTS, spent: TICK }

// ---- geometry ---------------------------------------------------------------------------------------

/** Where this edge sits among the lines entering its target, sorted by the height of their sources,
 *  and how tall the target card is. A primitive string, so the edge re-renders only when that changes. */
function fanKey(id: string, target: string) {
  return (s: ReactFlowState) => {
    const inc = s.edges.filter((e) => e.target === target && !e.hidden)
    const y = (n: string) => {
      const node = s.nodeLookup.get(n)
      return node ? node.internals.positionAbsolute.y + (node.measured.height ?? 0) / 2 : 0
    }
    inc.sort((a, b) => y(a.source) - y(b.source) || (a.id < b.id ? -1 : 1))
    const h = s.nodeLookup.get(target)?.measured.height ?? 80
    return `${inc.findIndex((e) => e.id === id)}/${inc.length}/${Math.round(h)}`
  }
}

const safe = (id: string) => id.replace(/[^a-zA-Z0-9_-]/g, '_')

function Token({ flow, x, y, big }: { flow: Flow; x: number; y: number; big: boolean }) {
  const s = big ? 1.45 : 1
  if (flow === 'spent') {
    return (
      <g className="ln-tokens-token" transform={`translate(${x} ${y}) scale(${s})`}>
        <circle r={5.4} className="ln-tokens-plate" />
        <g className="ln-tokens-glyph" transform="scale(0.8)">{TICK}</g>
      </g>
    )
  }
  return (
    <g className="ln-tokens-token" transform={`translate(${x} ${y}) scale(${s})`}>
      <circle r={R + 1.6} cy={0.9} className="ln-tokens-shadow" />
      <circle r={R} className="ln-tokens-rim" />
      <circle r={R - 1.9} className="ln-tokens-face" />
      <path d={`M${-(R - 2.6)} -1.4a${R - 2.6} ${R - 2.6} 0 0 1 ${2 * (R - 2.6)} 0`} className="ln-tokens-shine" />
      <g className="ln-tokens-glyph">{glyph[flow]}</g>
    </g>
  )
}

export default function Line(props: EdgeProps<QuestEdge>) {
  const { id, target, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data } = props
  const flow = data!.flow
  const wt = useWarTable()
  const far = useStore((s) => s.transform[2] < 0.72)
  const [idx, count, cardH] = useStore(fanKey(id, target)).split('/').map(Number)

  // Fan the lines entering one card apart along its left edge.
  const pitch = far ? PITCH * 1.3 : PITCH
  const span = Math.max(0, cardH - 26)
  const step = count > 1 ? Math.min(pitch, span / (count - 1)) : 0
  const off = idx >= 0 ? (idx - (count - 1) / 2) * step : 0
  const ty = targetY + off

  // The curve runs between two short straight stubs: one out of the source, one into the target. The
  // target stub is where the token sits, so it always rests level at the card's door, however steep the
  // curve; the source stub carries the port dot.
  const tuck = wt ? TUCK : 0
  const scale = far ? 1.45 : 1
  const inStub = (R + GAP) * scale + R * scale + 6
  const outStub = flow === 'bridge' ? inStub : 10
  const sx = sourceX - (wt && data!.torn ? TORN_TUCK : tuck)
  const [curve] = getBezierPath({
    sourceX: sourceX + outStub,
    sourceY,
    targetX: targetX - inStub,
    targetY: ty,
    sourcePosition,
    targetPosition,
  })
  const path = `M${sx},${sourceY} L${curve.slice(1)} L${targetX + tuck},${ty}`
  const [kx, ky] = [targetX - (R + GAP) * scale, ty]
  const [px, py] = [sourceX + (flow === 'bridge' ? (R + GAP) * scale : 5), sourceY]
  const dim = data!.dim
  const uid = safe(id)

  return (
    <g className="ln-tokens" data-flow={flow} data-dim={dim || undefined} data-far={far || undefined} id={`ln-tokens-${uid}`}>
      <path d={path} fill="none" className="ln-tokens-halo" />
      <path d={path} fill="none" stroke="currentColor" className="ln-tokens-body" />
      {/* the port: where the line leaves its source */}
      {flow === 'bridge' ? (
        <Token flow="bridge" x={px} y={py} big={far} />
      ) : data!.torn ? (
        <Seal x={sourceX + 6 * (far ? 1.45 : 1)} y={sourceY} big={far} />
      ) : (
        <circle cx={px} cy={py} r={far ? 4.6 : 3.4} className="ln-tokens-port" />
      )}
      {/* the gate: what the line means, at the door of the card it enters */}
      {glyph[flow] && <Token flow={flow} x={kx} y={ky} big={far} />}
      {flow === 'cancelled' && !data!.torn && <Seal x={kx} y={ky} big={far} />}
    </g>
  )
}
