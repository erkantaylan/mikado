import ELK from 'elkjs/lib/elk.bundled.js'
import type { Edge, Node } from '@xyflow/react'

import { NODE_HEIGHT, NODE_WIDTH } from './size'

const elk = new ELK()

type Size = { width: number; height: number }
const defaultSize = (): Size => ({ width: NODE_WIDTH, height: NODE_HEIGHT })

/** The gaps ELK leaves: between columns (layers) and between cards in the same column. */
export type Spacing = { layers: number; nodes: number }
const defaultSpacing: Spacing = { layers: 60, nodes: 40 }

/** Positions nodes with ELK's layered algorithm; edges flow in `direction`. */
export async function layout<N extends Node>(
  nodes: N[],
  edges: Edge[],
  sizeOf: (n: N) => Size = defaultSize,
  direction: 'DOWN' | 'UP' | 'RIGHT' = 'DOWN',
  spacing: Spacing = defaultSpacing,
): Promise<N[]> {
  const graph = await elk.layout({
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': direction,
      'elk.spacing.nodeNode': String(spacing.nodes),
      'elk.layered.spacing.nodeNodeBetweenLayers': String(spacing.layers),
    },
    children: nodes.map((n) => ({ id: n.id, ...sizeOf(n) })),
    edges: edges.map((e) => ({ id: e.id, sources: [e.source], targets: [e.target] })),
  })

  const placed = new Map(graph.children?.map((c) => [c.id, c]))
  return nodes.map((n) => {
    const p = placed.get(n.id)
    return { ...n, position: { x: p?.x ?? 0, y: p?.y ?? 0 } }
  })
}
