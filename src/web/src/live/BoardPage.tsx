import { useMemo } from 'react'
import { fetchQuests } from '../api'
import QuestBoard from '../quest/QuestBoard'
import { Banner, Cli, Notice } from '../quest/Notice'
import { toBoardQuest } from './adapt'
import { Failed, Loading, StaleBanner } from './states'
import { usePoll } from './usePoll'

/** `/`: every quest the server knows, refreshed every 15 s. */
export default function BoardPage() {
  const { data, error } = usePoll(fetchQuests)
  const quests = useMemo(() => data?.quests.map(toBoardQuest), [data])
  if (!data || !quests) return error ? <Failed error={error} what="the Quest Board" /> : <Loading what="the Quest Board" />
  return (
    <QuestBoard
      quests={quests}
      hrefOf={(q) => `/quest/${encodeURIComponent(q.slug)}`}
      eyebrow="mikado"
      banner={
        <>
          {error && <StaleBanner error={error} />}
          {data.github && <Banner>GitHub: {data.github}</Banner>}
        </>
      }
      empty={
        <Notice title="No quests yet">
          <p>A quest is a small goal, crowned by one deed. Start one from the command line:</p>
          <Cli>mikado quest new "…"</Cli>
          <p>It shows up here within 15 seconds.</p>
        </Notice>
      }
    />
  )
}
