import SharedBoard from '../quest/Atlas'
import { hasMap, quests } from './data'

/** The mock Quest Board, from data.ts. Only the quests with a mock chart open one. */
export default function QuestBoard() {
  return (
    <SharedBoard
      journeys={quests}
      hrefOf={(q) => (hasMap(q.key) ? `/mock/quest/${q.key}` : undefined)}
      eyebrow="mikado · mock"
      noMap="no chart in the mock"
    />
  )
}
