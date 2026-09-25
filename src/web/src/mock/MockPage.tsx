import { useMemo } from 'react'
import QuestMap from '../quest/QuestMap'
import { questModel, type Item, type QuestData } from '../quest/model'
import { maps } from './data'
import { lineDesigns } from './lines'

const TODAY = 24 // Sep 24, fixed for the mock

// The mock has no keys or day counts of its own: keys are numbered in order
// (main cards, then side quests) and waits count from the fixed mock "today".
function prepare(q: QuestData): QuestData {
  let n = 0
  const fill = (i: Item): Item => ({
    ...i,
    key: `M${++n}`,
    sinceDays: i.since ? TODAY - Number(i.since.split(' ')[1]) : undefined,
  })
  return { ...q, items: q.items.map(fill), sideQuests: q.sideQuests.map(fill) }
}

/** The mock chart: the quest named in the URL, from data.ts. */
export default function MockPage() {
  const slug = location.pathname.split('/').pop() ?? ''
  const model = useMemo(() => questModel(prepare(maps[slug] ?? maps['winter-update'])), [slug])
  // ?lines=<id> tries one of the trial line designs (mock/lines) instead of today's lines.
  const lines = new URLSearchParams(location.search).get('lines')
  const design = lineDesigns.find((d) => d.id === lines)
  const lineTypes = useMemo(() => (design ? { quest: design.Line } : undefined), [design])
  return (
    <>
      <QuestMap model={model} boardHref="/mock" lineTypes={lineTypes} />
      <LinePicker current={design?.id} />
    </>
  )
}

/** A strip at the bottom to switch between today's lines and the trial designs. */
function LinePicker({ current }: { current?: string }) {
  const href = (id?: string) => (id ? `?lines=${id}` : location.pathname)
  const link = (id: string | undefined, name: string, title: string) => (
    <a
      key={id ?? 'today'}
      href={href(id)}
      title={title}
      className={`rounded px-2 py-0.5 whitespace-nowrap ${id === current ? 'bg-[#6b4812] text-[#f3e9cf]' : 'hover:bg-[#e6d7b2]'}`}
    >
      {name}
    </a>
  )
  return (
    <nav className="fixed bottom-3 left-1/2 z-50 flex -translate-x-1/2 gap-1 rounded-lg border border-[#8f6a26] bg-[#f3e9cf] px-2 py-1 text-[13px] text-[#3a2812] shadow-lg">
      {link(undefined, 'Today', 'the lines as they are now')}
      {lineDesigns.filter((d) => d.shortlist).map((d) => link(d.id, d.name, d.idea))}
      <span className="ml-2 self-center border-l border-[#8f6a26] pl-2 text-[11px] tracking-wider text-[#8f6a26] uppercase">ideas</span>
      {lineDesigns.filter((d) => !d.shortlist).map((d) => link(d.id, d.name, d.idea))}
    </nav>
  )
}
