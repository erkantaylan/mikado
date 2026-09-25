import type { EdgeProps } from '@xyflow/react'
import type { JSX } from 'react'
import type { QuestEdge } from '../../quest/graph'
import road from './road'
import molten from './molten'
import railway from './railway'
import stitch from './stitch'
import tokens from './tokens'
import ribbon from './ribbon'
import pins from './pins'
import lanterns from './lanterns'
import chain from './chain'

// Trial designs for the lines between deeds on the war table, one file each.
// /mock/quest/all-lines?lines=<id> draws the chart with one of them.
// `shortlist`: still in the running. The rest are kept as ideas: good looking, not usable as they are.
export type LineDesign = { id: string; name: string; idea: string; shortlist?: boolean; Line: (props: EdgeProps<QuestEdge>) => JSX.Element }

export const lineDesigns: LineDesign[] = [
  { id: 'road', name: "King's Highway", shortlist: true, idea: "1.1 — cased roads: two ink edges, a paper core; crossings read as overpasses", Line: road },
  { id: 'railway', name: "Supply Railway", shortlist: true, idea: "2.3 / 1.4 — track with cross-ties; planned, laid and torn-up track", Line: railway },
  { id: 'molten', name: "Molten Gold", idea: "4.4 — engraved grooves, empty when locked, filled with gold when powered", Line: molten },
  { id: 'stitch', name: "Couched Thread", idea: "3.3 — lines stitched through the map; the stitch tells the kind", Line: stitch },
  { id: 'tokens', name: "Gate Tokens", idea: "5.3 — simple lines; a pictogram token where each line meets its card", Line: tokens },
  { id: 'ribbon', name: "Seal and Ribbon", idea: "3.4 — silk ribbons ending in a wax seal on the deed they open", Line: ribbon },
  { id: 'pins', name: "Thread and Pins", idea: "2.1 — thread pulled taut from a brass map pin", Line: pins },
  { id: 'chain', name: "Rope and Chain", idea: "2.4 — hemp rope for what is planned, brass chain once powered; segments laid along the curve", Line: chain },
  { id: 'lanterns', name: "Lantern Road", idea: "4.1 — lantern dots, lit amber when powered, dark iron before", Line: lanterns },
]
