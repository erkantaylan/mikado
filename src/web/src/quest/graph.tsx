import { useEffect, useRef, type CSSProperties } from 'react'
import {
  BaseEdge,
  Handle,
  Position,
  getBezierPath,
  useReactFlow,
  useStore,
  type Edge,
  type EdgeProps,
  type Node,
  type NodeProps,
  type ReactFlowState,
} from '@xyflow/react'
import { Ban, Check, Crown, Gem, Hourglass, Sparkles, Trophy } from 'lucide-react'
import type { Item, JourneyModel, JourneyRef, JourneyState, Status } from './model'
import { Achievements, Gate, Hero, Kinds, Label, Npc, Working, crownMedal, glow, medal, plate, spentText, sideMedal, sidePlate, stateColour, words, type Kind } from './look'
import { useWarTable } from './theme'
import { JourneyCardView } from './JourneyCard'
import { RailwayLine } from './railway'

// ---- nodes -----------------------------------------------------------------

// Widths are fixed; heights follow the content, so a long title is shown in full.
// The layout runs after React Flow has measured the real heights.
const GOAL_WIDTH = 290
const CARD_WIDTH = 310
const SIDE_WIDTH = 290

// Node data carries everything a node draws, so node components read no journey state of their own.
type GoalData = { title: string; state?: JourneyState; done: number; total: number; reached: boolean; bonus: number; bonusTotal: number }
export type CardData = { item: Item; tag: string; status: Status; openBefore: number; days: number; dim: boolean; selected: boolean }
type GoalNode = Node<GoalData, 'goal'>
type CardNode = Node<CardData, 'card'>
export type QuestNode = GoalNode | CardNode

const hidden = '!opacity-0'

function MedalIcon({ item, status, openBefore }: { item: Item; status: Status; openBefore: number }) {
  const size = item.sideOf ? 13 : 16
  if (status === 'cancelled') return <Ban size={size} />
  if (status === 'done') return <Check size={size} strokeWidth={3} />
  if (item.sideOf) return <Gem size={size} />
  if (status === 'awaiting') return <Hourglass size={size} />
  if (status === 'locked') return <span className="text-[14px] leading-none font-bold">{openBefore}</span>
  if (item.final) return <Crown size={size} />
  return <Sparkles size={size} />
}

const journeyWord: Record<JourneyState, string> = { active: 'Journey', complete: 'Journey fulfilled', cancelled: 'Journey abandoned' }

function GoalView({ data }: NodeProps<GoalNode>) {
  const pct = data.total ? Math.round((data.done / data.total) * 100) : 0
  return (
    <div
      style={{ width: GOAL_WIDTH, borderColor: 'var(--gold)', background: 'var(--plate)', boxShadow: glow('--gold', 30, 35) }}
      data-state={data.state ?? 'active'}
      data-reached={data.reached || undefined}
      className="quest-goal flex flex-col items-center justify-center gap-2 rounded-2xl border-[3px] px-4 py-4 text-center"
    >
      <Handle type="target" position={Position.Left} className={hidden} />
      <span
        className="quest-goal-medal grid size-12 place-items-center rounded-full border-2"
        style={data.reached ? crownMedal : data.state === 'cancelled' ? medal.cancelled : { borderColor: 'var(--gold)', color: 'var(--gold)' }}
      >
        {data.state === 'cancelled' ? <Ban size={24} /> : <Trophy size={24} />}
      </span>
      <span
        className="text-[12px] font-bold tracking-[0.25em] uppercase"
        style={{ color: data.state === 'cancelled' ? stateColour.cancelled : stateColour.done }}
      >
        {journeyWord[data.state ?? 'active']}
      </span>
      <span className="quest-display quest-goal-title text-[15px] leading-tight font-semibold">{data.title}</span>
      <div className="w-full">
        <div className="h-2 overflow-hidden rounded-full bg-[var(--chip)] ring-1 ring-[var(--plate-border)]">
          <div className="quest-progress h-full bg-[var(--gold)]" style={{ width: `${pct}%` }} />
        </div>
        <div className="mt-1 text-[13px] text-[var(--ink-soft)]">
          Main quest {data.done}/{data.total}
        </div>
      </div>
      <Achievements done={data.bonus} total={data.bonusTotal} />
    </div>
  )
}

/** The other journeys a quest also belongs to, each a link to its chart. */
export function AlsoIn({ journeys, label = true }: { journeys: JourneyRef[]; label?: boolean }) {
  return (
    <span className="flex min-w-0 flex-wrap items-center gap-1 text-[11px] font-semibold text-[var(--ink-faint)]">
      {label && <span className="tracking-wider uppercase">also in:</span>}
      {journeys.map((q) => (
        <a
          key={q.key}
          href={`/journey/${encodeURIComponent(q.key)}`}
          onClick={(e) => e.stopPropagation()}
          className="max-w-full truncate rounded border border-[var(--panel-border)] bg-[var(--chip)] px-1 text-[var(--ink-soft)] hover:text-[var(--ink)] hover:underline"
          title={q.title}
        >
          {q.title}
        </a>
      ))}
    </span>
  )
}

/**
 * An NPC quest's plate: red frame and tint over whatever its status says, so NPCs stand out at a
 * glance while the medal and label still tell the status. A fulfilled or abandoned NPC is a quieter
 * red, without the glow, like any finished quest.
 */
function npcPlate(status: Status): CSSProperties {
  if (status === 'done' || status === 'cancelled')
    return {
      background: 'color-mix(in srgb, var(--npc-plate) 55%, var(--panel))',
      borderColor: 'color-mix(in srgb, var(--npc) 55%, var(--panel))',
      opacity: plate[status].opacity,
    }
  return { background: 'var(--npc-plate)', borderColor: 'var(--npc)', boxShadow: glow('--npc', 16, 40) }
}

function CardView({ data }: NodeProps<CardNode>) {
  const { item, tag, status, openBefore, days, dim, selected } = data
  const wt = useWarTable()
  // A quest that crowns another journey stands for that whole journey here.
  if (item.crowns) return <JourneyCardView item={item} q={item.crowns} status={status} dim={dim} selected={selected} />
  const side = !!item.sideOf
  const label = side && status !== 'done' ? 'Optional' : status === 'awaiting' ? `${words.awaiting} · ${days} days` : words[status]
  const underway = item.working && !item.done
  // War table: what sort of card it is, told by small marks on the top-right instead of words.
  const kinds: Kind[] = []
  if (side) kinds.push('side')
  if (item.foundWhile) kinds.push('found')
  if (item.npc) kinds.push('npc')
  return (
    <div
      style={{ width: side ? SIDE_WIDTH : CARD_WIDTH }}
      // The data-* attributes are only for the war-table theme's CSS; the other themes ignore them.
      data-status={status}
      data-side={side || undefined}
      data-npc={item.npc || undefined}
      data-dim={dim || undefined}
      className={`quest-item relative cursor-pointer transition-opacity ${dim ? 'opacity-25' : ''}`}
    >
      <Handle type="target" position={Position.Left} className={hidden} />
      {wt ? (
        // The war table's gate: can it be started?
        <Gate status={status} count={openBefore} small={side} />
      ) : (
        /* The state badge sits on the quest's corner. */
        <span
          style={side && status !== 'done' ? sideMedal : medal[status]}
          data-mark={side && status !== 'done' ? 'side' : status}
          className={`quest-medal absolute -top-3 -left-3 z-10 grid place-items-center rounded-full border-2 ${side ? 'size-7' : 'size-9'} ${
            status === 'available' && !side ? 'quest-available' : ''
          }`}
        >
          <MedalIcon item={item} status={status} openBefore={openBefore} />
        </span>
      )}
      {underway && !wt && (
        <span className="absolute -top-3 right-3 z-10">
          <Working by={item.workingBy} />
        </span>
      )}
      <div
        style={item.npc ? npcPlate(status) : side ? sidePlate : plate[status]}
        className={`quest-plate relative flex flex-col gap-1.5 rounded-lg py-2 pr-3 pl-5 ${side ? 'border' : 'border-2'} ${
          item.foundWhile && !wt ? 'quest-discovered !border-dashed' : ''
        } ${selected ? 'quest-lit' : ''}`}
      >
        {/* The key would sit under the corner medal, so the row starts clear of it. */}
        <div className={`flex min-w-0 items-center gap-1.5 ${item.key ? (side ? 'pl-1.5' : 'pl-2.5') : ''}`}>
          <Label item={item} tag={tag} />
          {item.final && (
            <span className="shrink-0 text-[12px] font-bold tracking-wider whitespace-nowrap uppercase" style={{ color: stateColour.done }}>
              · Crowning quest
            </span>
          )}
          {wt ? (
            <Kinds kinds={kinds} reason={item.reason}>
              {underway && <Working by={item.workingBy} />}
            </Kinds>
          ) : (
            <>
              {side && (
                <span className="shrink-0 text-[12px] font-semibold tracking-wider whitespace-nowrap text-[var(--side)] uppercase">· Side quest</span>
              )}
              {item.npc && <Npc />}
              {item.foundWhile && (
                <span className="ml-auto shrink-0 text-[11px] font-semibold tracking-wide text-[var(--ink-faint)] uppercase" title={item.reason}>
                  found
                </span>
              )}
            </>
          )}
        </div>
        <span
          className={`quest-title leading-snug ${
            side || status === 'done' ? 'text-[var(--ink-soft)]' : status === 'cancelled' ? 'text-[var(--ink-faint)] line-through' : ''
          } ${side ? 'text-[13.5px]' : status === 'done' ? 'text-[15px] font-medium' : 'text-[15px] font-semibold'}`}
        >
          {item.title.replace(/^Polish: /, '')}
        </span>
        {!!item.alsoIn?.length && <AlsoIn journeys={item.alsoIn} />}
        <div className="flex items-center justify-between">
          <span
            className="quest-status text-[12px] font-bold tracking-wider uppercase"
            style={{ color: side && status !== 'done' ? 'var(--side)' : status === 'done' ? spentText : stateColour[status] }}
          >
            {label}
            {status === 'locked' && ` · ${openBefore} to go`}
          </span>
          <Hero name={item.assignee} />
        </div>
      </div>
      <Handle type="source" position={Position.Right} className={hidden} />
    </div>
  )
}

export const nodeTypes = { goal: GoalView, card: CardView }

// ---- edges -----------------------------------------------------------------

// From a fulfilled quest: done opens a quest you can do now (bright, and it flows); held feeds one that still
// waits on others; spent joins two fulfilled quests, so it steps back.
// A bridge stands in for a chain that runs through hidden quests.
export type Flow = 'done' | 'held' | 'spent' | 'locked' | 'side' | 'cancelled' | 'bridge'
// torn: the line comes from an abandoned quest, whose paper the war table draws with a strip torn off its right side.
// powered: the quest the line comes from is fulfilled (for a side quest's line: the side quest is).
export type QuestEdge = Edge<{ flow: Flow; live: boolean; dim: boolean; torn: boolean; powered: boolean }, 'quest'>

// How each kind of line is drawn in the parchment and midnight themes, from the theme's own palette.
export const stroke: Record<Flow, CSSProperties> = {
  done: { stroke: 'var(--gold)', strokeWidth: '3.5px', filter: 'drop-shadow(0 0 3px color-mix(in srgb, var(--gold) 70%, transparent))' },
  held: { stroke: 'var(--gold)', strokeWidth: '2.5px', opacity: 0.75 },
  spent: { stroke: 'color-mix(in srgb, var(--gold) 40%, var(--edge-off))', strokeWidth: '2px' },
  locked: { stroke: 'var(--edge-off)', strokeWidth: '2.5px' },
  side: { stroke: 'var(--side)', strokeWidth: '1.5px', strokeDasharray: '5 6' },
  cancelled: { stroke: 'var(--edge-off)', strokeWidth: '1.5px', strokeDasharray: '2 5', opacity: 0.6 },
  bridge: { stroke: 'var(--ink-faint)', strokeWidth: '2px', strokeDasharray: '10 4 2 4', opacity: 0.8 },
}

// On the war table a line starts and ends a little under the cards, so it comes out from under the paper
// rather than stopping short of the card's edge; the cards are opaque there, so the extra length is hidden.
// Lines always leave a card's right side and enter the next card's left side. From an abandoned card the
// line tucks deeper, past the torn strip, so it does not stop in the tear.
const TUCK = 12
const TORN_TUCK = 32

/** The curve a line follows, with the war table's tuck under the cards. Shared with the mock's trial line designs. */
export function questEdgePath(
  { sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data }: EdgeProps<QuestEdge>,
  wt: boolean,
) {
  const tuck = wt ? TUCK : 0
  const [path, labelX, labelY] = getBezierPath({
    sourceX: sourceX - (wt && data!.torn ? TORN_TUCK : tuck),
    sourceY,
    targetX: targetX + tuck,
    targetY,
    sourcePosition,
    targetPosition,
  })
  return { path, labelX, labelY }
}

/** A line between quests in the parchment and midnight themes. */
export function QuestEdgeView(props: EdgeProps<QuestEdge>) {
  const { data } = props
  const { path } = questEdgePath(props, false)
  const style = stroke[data!.flow]
  const opacity = data!.dim ? 0.15 : ((style.opacity as number | undefined) ?? 1)
  return (
    <>
      <BaseEdge path={path} style={{ ...style, opacity }} />
      {data!.live && <path d={path} fill="none" stroke="var(--gold-ink)" strokeWidth={2} className="quest-flow" style={{ opacity }} />}
    </>
  )
}

// The war table lays narrow-gauge track instead (railway.tsx). The theme is only known inside the flow, so one
// edge type picks per line.
function QuestLine(props: EdgeProps<QuestEdge>) {
  return useWarTable() ? <RailwayLine {...props} /> : <QuestEdgeView {...props} />
}

export const edgeTypes = { quest: QuestLine }

// ---- graph -----------------------------------------------------------------

/** Which quests can be left off the chart: fulfilled ones, abandoned ones, or both. */
export type Hide = { done: boolean; cancelled: boolean }

/**
 * The quests `hide` leaves off the chart. The crowning quest always stays; a side quest goes with
 * the quest it hangs on, as it would float on its own.
 */
export function hiddenQuests(m: JourneyModel, hide: Hide): Set<string> {
  const off = (i: Item) => {
    if (i.final || i.id === m.goal.doneWhen) return false
    const s = m.statusOf(i)
    return (hide.done && s === 'done') || (hide.cancelled && s === 'cancelled')
  }
  const out = new Set(m.items.filter(off).map((i) => i.id))
  for (const q of m.sideQuests) if (off(q) || (q.sideOf && out.has(q.sideOf))) out.add(q.id)
  return out
}

/**
 * Where a shown quest requires a shown one only through hidden quests, a bridge joins the two, so
 * the chain still reads left to right. None where shown edges or other bridges already join them.
 * Each is [required, requiring], like a need's [to, from].
 */
function bridges(m: JourneyModel, hidden: Set<string>): [string, string][] {
  const requires = new Map<string, string[]>()
  for (const n of m.needs) requires.set(n.from, [...(requires.get(n.from) ?? []), n.to])
  const shown = (id: string) => m.byId.has(id) && !hidden.has(id)

  const found: [string, string][] = []
  for (const a of m.items) {
    if (!shown(a.id)) continue
    const beyond = new Set<string>()
    const seen = new Set<string>()
    const stack = (requires.get(a.id) ?? []).filter((id) => hidden.has(id))
    while (stack.length) {
      const cur = stack.pop()!
      if (seen.has(cur)) continue
      seen.add(cur)
      for (const t of requires.get(cur) ?? []) {
        if (hidden.has(t)) stack.push(t)
        else if (shown(t)) beyond.add(t)
      }
    }
    for (const c of beyond) found.push([c, a.id])
  }

  // Drop each bridge that the shown edges and the bridges still kept already imply.
  const opens = new Map<string, string[]>()
  const link = (to: string, from: string) => opens.set(to, [...(opens.get(to) ?? []), from])
  for (const n of m.needs) if (shown(n.to) && shown(n.from)) link(n.to, n.from)
  let kept = found
  for (const b of found) {
    const others = kept.filter((k) => k !== b)
    const next = new Map(opens)
    for (const [to, from] of others) next.set(to, [...(next.get(to) ?? []), from])
    const seen = new Set<string>()
    const stack = [b[0]]
    let joined = false
    while (stack.length && !joined) {
      const cur = stack.pop()!
      for (const t of next.get(cur) ?? []) {
        if (t === b[1]) joined = true
        else if (!seen.has(t)) {
          seen.add(t)
          stack.push(t)
        }
      }
    }
    if (joined) kept = others
  }
  return kept
}

/** The chart's nodes and edges; quests in `hidden` are left off, bridged where they joined others. */
export function buildGraph(
  m: JourneyModel,
  selected: string | null,
  state?: JourneyState,
  hidden: Set<string> = new Set(),
): { nodes: QuestNode[]; edges: QuestEdge[] } {
  const { goal, items, sideQuests, needs, byId } = m
  const onPath = selected ? m.pathToGoal(selected) : null
  const done = items.filter((i) => i.done && m.counted(i)).length
  const reached = !!byId.get(goal.doneWhen)?.done

  const nodes: QuestNode[] = [
    {
      id: 'goal',
      type: 'goal',
      position: { x: 0, y: 0 },
      data: {
        title: goal.title,
        state,
        done,
        total: items.filter(m.counted).length,
        reached,
        bonus: sideQuests.filter((q) => q.done && m.counted(q)).length,
        bonusTotal: sideQuests.filter(m.counted).length,
      },
    },
    ...[...items, ...sideQuests]
      .filter((item) => !hidden.has(item.id))
      .map(
      (item): CardNode => ({
        id: item.id,
        type: 'card',
        position: { x: 0, y: 0 },
        data: {
          item,
          tag: m.short(item.id),
          status: m.statusOf(item),
          openBefore: m.openBefore(item),
          days: m.daysSince(item),
          dim: !!onPath && !onPath.has(item.id),
          selected: item.id === selected,
        },
      }),
    ),
  ]

  const dim = (a: string, b: string) => !!onPath && !(onPath.has(a) && onPath.has(b))
  const torn = (id: string) => {
    const i = byId.get(id)
    return !!i && m.statusOf(i) === 'cancelled'
  }
  // An edge carries power once the quest it comes from is fulfilled; it flows while it feeds an unfulfilled quest.
  const edge = (source: string, target: string, side = false): QuestEdge => {
    const powered = !!byId.get(source)?.done
    const to = byId.get(target)
    return {
      id: `${source}->${target}`,
      source,
      target,
      type: 'quest',
      data: {
        flow: side
          ? 'side'
          : byId.get(source)?.cancelled
            ? 'cancelled'
            : !powered
              ? 'locked'
              : !to || m.statusOf(to) === 'available'
                ? 'done' // into an open quest, or the crowning quest into the journey's goal
                : to.done
                  ? 'spent'
                  : 'held',
        live: !side && powered && !!to && m.statusOf(to) === 'available',
        powered,
        dim: dim(source, target),
        torn: torn(source),
      },
    }
  }

  // Only edges between quests that are on the chart: a link to anything else would be drawn to nowhere.
  const known = (id: string) => id === 'goal' || (byId.has(id) && !hidden.has(id))
  const bridge = ([source, target]: [string, string]): QuestEdge => ({
    id: `bridge:${source}->${target}`,
    source,
    target,
    type: 'quest',
    data: { flow: 'bridge', live: false, dim: dim(source, target), torn: torn(source), powered: false },
  })
  return {
    nodes,
    edges: [
      ...(goal.doneWhen ? [edge(goal.doneWhen, 'goal')] : []),
      ...needs.map((n) => edge(n.to, n.from)),
      ...sideQuests.filter((q) => q.sideOf).map((q) => edge(q.id, q.sideOf!, true)),
      ...(hidden.size ? bridges(m, hidden).map(bridge) : []),
    ].filter((e) => known(e.source) && known(e.target)),
  }
}

export const miniClass = (n: Node) => {
  if (n.type === 'goal') return 'mm-goal'
  const d = n.data as CardData
  if (d.item.npc) return d.status === 'done' || d.status === 'cancelled' ? 'mm-npc-done' : 'mm-npc'
  return d.item.sideOf && !d.item.done ? 'mm-side' : `mm-${d.status}`
}

// ---- layout ----------------------------------------------------------------

export type Placed = Map<string, { x: number; y: number }>

// What the layout depends on: which nodes and edges there are and how big each
// node measured. Empty while any node is still unmeasured. Status and style
// changes leave it alone, so they never move a card.
function shapeOf(s: ReactFlowState): string {
  if (s.nodes.length === 0) return ''
  const parts: string[] = []
  for (const n of s.nodes) {
    const m = s.nodeLookup.get(n.id)?.measured
    if (!m?.width || !m.height) return ''
    parts.push(`${n.id}:${Math.round(m.width)}x${Math.round(m.height)}`)
  }
  return `${parts.join(',')}|${s.edges.map((e) => e.id).join(',')}`
}

/** Whenever React Flow has measured a new set of cards, run ELK with the real sizes. */
export function LayoutWhenMeasured({ onPlaced }: { onPlaced: (p: Placed) => void }) {
  const shape = useStore(shapeOf)
  // The war table's lines are drawn objects (roads, tracks) that need room to read, so its columns sit further apart.
  const wt = useWarTable()
  const { getNodes, getEdges, getInternalNode } = useReactFlow()
  const ran = useRef('')
  useEffect(() => {
    // The spacing is part of what was laid out, so switching into or out of the war table lays out again.
    const key = shape && `${shape}|${wt}`
    if (!key || key === ran.current) return
    ran.current = key
    const size = (n: Node) => {
      const m = getInternalNode(n.id)?.measured ?? n.measured
      return { width: m?.width ?? 0, height: m?.height ?? 0 }
    }
    const spacing = wt ? { layers: 160, nodes: 48 } : undefined
    import('../graph/layout')
      .then(({ layout }) => layout(getNodes(), getEdges(), size, 'RIGHT', spacing))
      .then((placed) => {
        // A newer shape arrived while ELK was busy: its own run will place the nodes.
        if (ran.current === key) onPlaced(new Map(placed.map((n) => [n.id, n.position])))
      })
  }, [shape, getNodes, getEdges, getInternalNode, onPlaced, wt])
  return null
}

/** Once placed, open at a readable zoom on the frontier: what can be worked on now. Only once per mount. */
export function FocusWhenPlaced({ placed, frontier }: { placed: boolean; frontier: string[] }) {
  const { fitView } = useReactFlow()
  const done = useRef(false)
  const latest = useRef(frontier)
  useEffect(() => {
    latest.current = frontier
  }, [frontier])
  useEffect(() => {
    if (!placed || done.current) return
    done.current = true
    const nodes = (latest.current.length ? latest.current : ['goal']).map((id) => ({ id }))
    requestAnimationFrame(() => fitView({ nodes, padding: 0.35, minZoom: 0.8, maxZoom: 1 }))
  }, [placed, fitView])
  return null
}
