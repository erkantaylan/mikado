import SharedBoard from '../quest/QuestBoard'
import { hasMap, quests } from './data'

/** The mock Quest Board, from data.ts. Only the quests with a mock chart open one. */
export default function QuestBoard() {
  return (
    <SharedBoard
      quests={quests}
      hrefOf={(q) => (hasMap(q.slug) ? `/mock/quest/${q.slug}` : undefined)}
      eyebrow="mikado · mock"
      noMap="no chart in the mock"
    />
  )
}
