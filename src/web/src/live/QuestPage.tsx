import { useCallback, useMemo } from 'react'
import { fetchQuest, retitleQuest, setNpc } from '../api'
import QuestMap from '../quest/QuestMap'
import { questModel } from '../quest/model'
import { Banner, NoticePage } from '../quest/Notice'
import { toQuestData } from './adapt'
import { Failed, Loading, StaleBanner } from './states'
import { usePoll } from './usePoll'

/** `/quest/:slug`: one quest's chart, refreshed every 15 s without moving the view. */
export default function QuestPage({ slug }: { slug: string }) {
  const load = useCallback(() => fetchQuest(slug), [slug])
  const { data, error, refresh } = usePoll(load)
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
      archived={!!data.quest.archivedAt}
      slug={data.quest.slug}
      onRetitle={async (title) => {
        await retitleQuest(data.quest.slug, title)
        await refresh()
      }}
      onSetNpc={async (item, npc) => {
        await setNpc(Number(item.id.slice(1)), npc) // node ids are c<card id> (adapt.ts)
        await refresh()
      }}
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
