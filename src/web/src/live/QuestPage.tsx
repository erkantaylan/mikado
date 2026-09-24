import { useCallback, useMemo } from 'react'
import { fetchQuest } from '../api'
import QuestMap from '../quest/QuestMap'
import { questModel } from '../quest/model'
import { Banner, NoticePage } from '../quest/Notice'
import { toQuestData } from './adapt'
import { Failed, Loading, StaleBanner } from './states'
import { usePoll } from './usePoll'

/** `/quest/:slug`: one quest's chart, refreshed every 15 s without moving the view. */
export default function QuestPage({ slug }: { slug: string }) {
  const load = useCallback(() => fetchQuest(slug), [slug])
  const { data, error } = usePoll(load)
  const model = useMemo(() => (data ? questModel(toQuestData(data)) : undefined), [data])

  if (error?.status === 404)
    return (
      <NoticePage title="No such quest" back={{ href: '/', label: 'Quest Board' }}>
        <p>
          There is no quest called <b className="font-mono">{slug}</b>.
        </p>
      </NoticePage>
    )
  if (!data || !model) return error ? <Failed error={error} what="this quest" /> : <Loading what="the quest" />
  return (
    <QuestMap
      model={model}
      state={data.quest.state}
      boardHref="/"
      banner={
        <>
          {error && <StaleBanner error={error} />}
          {data.github && <Banner>GitHub: {data.github}</Banner>}
        </>
      }
    />
  )
}
