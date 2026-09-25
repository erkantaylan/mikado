import type { EdgeProps } from '@xyflow/react'
import { questEdgePath, QuestEdgeView, type QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'
import './lanterns.css'

// Lantern Road (4.1 — lantern dots, lit amber when powered, dark iron before).
// Every line is a road lined with lanterns. The lanterns are round-capped zero-length dashes, so they
// sit at an even spacing along the curve whatever its shape. Powered roads have their lanterns lit: an
// amber core on an ink rim, inside a warm pool of light that is itself only a wider, faint dash of the
// same rhythm (no blur, no filter). Unpowered roads carry dark iron lanterns. A small waymark at the
// middle of each road points the way the power runs, left to right. Everything is static.
//
// Layers, bottom to top: halo, pool of light (outer, inner), road, lantern rim, lantern core, shutter, waymark.

// The point and heading halfway along the bezier questEdgePath draws: M x0,y0 C x1,y1 x2,y2 x3,y3.
function midway(path: string) {
  const n = path.match(/-?\d+(\.\d+)?(e-?\d+)?/g)?.map(Number) ?? []
  if (n.length < 8) return null
  const [x0, y0, x1, y1, x2, y2, x3, y3] = n
  const x = (x0 + 3 * x1 + 3 * x2 + x3) / 8
  const y = (y0 + 3 * y1 + 3 * y2 + y3) / 8
  const a = (Math.atan2(y3 + y2 - y1 - y0, x3 + x2 - x1 - x0) * 180) / Math.PI
  return { x, y, a }
}

export default function Line(props: EdgeProps<QuestEdge>) {
  const wt = useWarTable()
  if (!wt) return <QuestEdgeView {...props} />
  const { data } = props
  const { path } = questEdgePath(props, wt)
  const flow = data!.flow
  const mid = midway(path)
  const cls = (part: string) => `ln-lanterns-${part}`
  const p = (part: string) => <path d={path} fill="none" className={cls(part)} />
  return (
    <g className="ln-lanterns" data-flow={flow} data-dim={data!.dim || undefined}>
      {p('halo')}
      {p('pool')}
      {p('glow')}
      {p('road')}
      {p('rim')}
      {p('core')}
      {p('shutter')}
      {mid && (
        <g transform={`translate(${mid.x.toFixed(1)} ${mid.y.toFixed(1)}) rotate(${mid.a.toFixed(1)})`}>
          <path d="M-5,-5.5 L2.5,0 L-5,5.5" className={cls('way-halo')} />
          <path d="M-5,-5.5 L2.5,0 L-5,5.5" className={cls('way')} />
        </g>
      )}
      {/* An invisible, wide path so the line stays easy to hover and click. */}
      <path d={path} fill="none" className="react-flow__edge-interaction" strokeOpacity={0} strokeWidth={16} />
    </g>
  )
}
