import type { EdgeProps } from '@xyflow/react'
import { questEdgePath, QuestEdgeView, type QuestEdge } from '../../quest/graph'
import { useWarTable } from '../../quest/theme'
import './molten.css'

// Molten Gold (4.4 — engraved grooves, empty when locked, filled with gold when powered).
// Every line is a channel cut into the table: a pale lip where the light catches the cut, a dark rim,
// and inside it either bare parchment (nothing has reached it yet) or metal poured in.
//   done       molten gold, a white-hot seam down the middle, a drop swelling into the card it opens
//   held       gold poured most of the way; the last stretch and the drop are still empty
//   spent      a thin groove of dull, cold gold
//   locked     an empty groove
//   side       a narrow groove inlaid with verdigris segments
//   cancelled  a groove that was started and abandoned: dashed rim, nothing inside
//   bridge     gold running underground: covered stretches of rim, gold showing between them
// Direction: every line starts at a round pouring well beside the card it leaves and ends in a teardrop
// pointing into the card it feeds. All static: no animation, no filters. Colours live in molten.css.

// The teardrop, tip at (0,0) pointing right, body behind it.
const DROP = 'M0.5,0 C-2.5,-1.2 -4.5,-4.6 -8.5,-4.6 A4.6,4.6 0 0 0 -8.5,4.6 C-4.5,4.6 -2.5,1.2 0.5,0 Z'

export default function Line(props: EdgeProps<QuestEdge>) {
  const wt = useWarTable()
  if (!wt) return <QuestEdgeView {...props} />
  const { data, sourceX, sourceY, targetX, targetY } = props
  const { path } = questEdgePath(props, wt)
  const flow = data!.flow
  const dim = data!.dim
  // Held: the fill stops short of the target; measured in pathLength units, so it scales with the line.
  const fillLen = flow === 'held' ? '68 100' : undefined
  const ends = flow !== 'cancelled'
  return (
    <g className="ln-molten" data-flow={flow} data-dim={dim || undefined}>
      <path d={path} className="ln-molten-lip" />
      <path d={path} className="ln-molten-rim" />
      {!dim && (
        <>
          <path d={path} className="ln-molten-wall" />
          <path d={path} className="ln-molten-bed" transform="translate(0 0.7)" />
          <path d={path} className="ln-molten-fill" pathLength={fillLen ? 100 : undefined} strokeDasharray={fillLen} />
          <path d={path} className="ln-molten-sheen" pathLength={fillLen ? 100 : undefined} strokeDasharray={fillLen} transform="translate(0 0.9)" />
          <path d={path} className="ln-molten-core" transform="translate(0 -0.6)" />
        </>
      )}
      {ends && (
        <>
          <circle cx={sourceX + 5} cy={sourceY} r={4.6} className="ln-molten-well" />
          <path d={DROP} transform={`translate(${targetX + 1} ${targetY})`} className="ln-molten-drop" />
          {!dim && <path d="M-9.6,-2.2 C-8.6,-3.2 -7,-3.2 -5.8,-2.6" transform={`translate(${targetX + 1} ${targetY})`} className="ln-molten-glint" />}
        </>
      )}
    </g>
  )
}
