import { useCallback, useMemo } from 'react'
import { fetchJourney, retitleJourney, setNpc } from '../api'
import JourneyMap from '../quest/JourneyMap'
import { journeyModel } from '../quest/model'
import { Banner, NoticePage } from '../quest/Notice'
import { toJourneyData } from './adapt'
import { Failed, Loading, StaleBanner } from './states'
import { usePoll } from './usePoll'
import { useVersion } from './useVersion'

/** `/journey/:key`: one journey's chart, refreshed every 15 s without moving the view. */
export default function JourneyPage({ journeyKey }: { journeyKey: string }) {
  const load = useCallback(() => fetchJourney(journeyKey), [journeyKey])
  const { data, error, refresh } = usePoll(load)
  const model = useMemo(() => (data ? journeyModel(toJourneyData(data)) : undefined), [data])
  const version = useVersion()

  if (error?.status === 404 || error?.status === 400)
    return (
      <NoticePage title="No such journey" back={{ href: '/', label: 'Atlas' }}>
        <p>
          There is no journey <b className="font-mono">{journeyKey}</b>.
        </p>
      </NoticePage>
    )
  if (!data || !model) return error ? <Failed error={error} what="this journey" /> : <Loading what="the journey" />
  return (
    <JourneyMap
      model={model}
      state={data.journey.state}
      archived={!!data.journey.archivedAt}
      journeyKey={data.journey.key}
      onRetitle={async (title) => {
        await retitleJourney(data.journey.key, title)
        await refresh()
      }}
      onSetNpc={async (item, npc) => {
        await setNpc(Number(item.id.slice(1)), npc) // node ids are c<card id> (adapt.ts)
        await refresh()
      }}
      atlasHref="/"
      version={version}
      banner={
        <>
          {error && <StaleBanner error={error} />}
          {data.github && <Banner>GitHub: {data.github}</Banner>}
        </>
      }
    />
  )
}
