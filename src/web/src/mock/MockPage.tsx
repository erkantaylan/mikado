import { useMemo } from 'react'
import QuestMap from '../quest/QuestMap'
import { questModel, type Item, type QuestData } from '../quest/model'
import { maps } from './data'

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
  return <QuestMap model={model} boardHref="/mock" />
}
