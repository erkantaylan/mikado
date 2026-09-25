import { EdgeLabelRenderer, type EdgeProps } from '@xyflow/react'
import { questEdgePath, type QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'
import './road.css'

// King's Highway (1.1 — cased roads: two ink edges, a paper core; crossings read as overpasses).
// Every line is a road drawn the way surveyors draw them: an ink casing with a lighter core. Each edge
// draws its casing and then its core, so where two roads cross the later road's core cuts through the
// earlier one's casing, which is how a map draws an overpass. The kind lives in the road's make-up:
//   done       a gilded highway (brass core with a bright crown line)
//   held       the same road under construction (brass paving laid in blocks on a paper bed)
//   spent      an old road, a single faded ink line
//   locked     an unmetalled road: ink edges, empty paper core
//   side       a green bridleway, a dotted track
//   cancelled  a washed-out road: broken casing, a cross where it gave way
//   bridge     a road in a tunnel: dashed casing, solid core
// A one-way arrow painted on the road at its middle gives the direction, and powered roads end in a
// milestone ring where they reach the card.

type Pt = [number, number]

/** The four points of the cubic bezier `questEdgePath` draws ("M x,y C x,y x,y x,y"). */
function bezier(path: string): [Pt, Pt, Pt, Pt] | null {
  const n = path.match(/-?\d+(\.\d+)?(e-?\d+)?/g)?.map(Number)
  if (!n || n.length < 8) return null
  return [
    [n[0], n[1]],
    [n[2], n[3]],
    [n[4], n[5]],
    [n[6], n[7]],
  ]
}

function at([p0, p1, p2, p3]: [Pt, Pt, Pt, Pt], t: number): { x: number; y: number; deg: number } {
  const u = 1 - t
  const x = u * u * u * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t * t * t * p3[0]
  const y = u * u * u * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t * t * t * p3[1]
  const dx = 3 * u * u * (p1[0] - p0[0]) + 6 * u * t * (p2[0] - p1[0]) + 3 * t * t * (p3[0] - p2[0])
  const dy = 3 * u * u * (p1[1] - p0[1]) + 6 * u * t * (p2[1] - p1[1]) + 3 * t * t * (p3[1] - p2[1])
  return { x, y, deg: (Math.atan2(dy, dx) * 180) / Math.PI }
}

export default function Line(props: EdgeProps<QuestEdge>) {
  const { data, targetX, targetY } = props
  const { path, labelX, labelY } = questEdgePath(props, useWarTable())
  const flow = data!.flow
  const curve = bezier(path)
  const mid = curve ? at(curve, 0.5) : { x: labelX, y: labelY, deg: 0 }
  const arrow = `translate(${mid.x} ${mid.y}) rotate(${mid.deg})`
  const powered = flow === 'done' || flow === 'held'

  return (
    <g className="ln-road" data-flow={flow} data-dim={data!.dim || undefined}>
      {/* the paper verge: a faint wash under every road so it lifts off the map's ink */}
      <path d={path} className="ln-road-verge" />
      <path d={path} className="ln-road-casing" />
      <path d={path} className="ln-road-core" />
      {flow === 'held' && <path d={path} className="ln-road-paving" />}
      {flow === 'done' && <path d={path} className="ln-road-crown" />}

      {flow === 'bridge' &&
        curve &&
        [0.14, 0.86].map((t, i) => {
          const p = at(curve, t)
          // a tunnel mouth: a bracket across the road, its wings flared away from the tunnel
          const d = i === 0 ? 'M-4 -8 L-1 -6 L-1 6 L-4 8' : 'M4 -8 L1 -6 L1 6 L4 8'
          return <path key={t} d={d} className="ln-road-portal" transform={`translate(${p.x} ${p.y}) rotate(${p.deg})`} />
        })}

      {flow === 'cancelled' ? (
        <g transform={arrow} className="ln-road-breach">
          <path d="M-4 -4 L4 4 M4 -4 L-4 4" />
        </g>
      ) : (
        <g transform={arrow} className="ln-road-arrow">
          <path d={flow === 'spent' || flow === 'side' ? 'M-3.5 -4.5 L2.5 0 L-3.5 4.5' : 'M-5 -5 L4.5 0 L-5 5 L-2.4 0 Z'} />
        </g>
      )}

      {powered && (
        // The milestone sits in the label layer, above every road, so the roads that meet at the same
        // port cannot bury it.
        <EdgeLabelRenderer>
          <svg
            className="ln-road-milestone"
            data-flow={flow}
            data-dim={data!.dim || undefined}
            width={18}
            height={18}
            viewBox="-9 -9 18 18"
            style={{ transform: `translate(${targetX - 18}px, ${targetY - 9}px)` }}
          >
            <circle r={6.4} className="ln-road-ring" />
            {flow === 'done' ? <circle r={2.6} className="ln-road-pip" /> : <path d="M0 -3.2 A3.2 3.2 0 0 0 0 3.2 Z" className="ln-road-pip" />}
          </svg>
        </EdgeLabelRenderer>
      )}
    </g>
  )
}
