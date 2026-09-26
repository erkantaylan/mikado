import { useMemo } from 'react'
import { fetchJourneys } from '../api'
import Atlas from '../quest/Atlas'
import { Banner, Cli, Notice } from '../quest/Notice'
import { toAtlasJourney } from './adapt'
import { Failed, Loading, StaleBanner } from './states'
import { usePoll } from './usePoll'
import { useVersion } from './useVersion'

/** `/`: the Atlas, every journey the server knows, refreshed every 15 s. */
export default function AtlasPage() {
  const { data, error } = usePoll(fetchJourneys)
  const journeys = useMemo(() => data?.journeys.map(toAtlasJourney), [data])
  const version = useVersion()
  if (!data || !journeys) return error ? <Failed error={error} what="the Atlas" /> : <Loading what="the Atlas" />
  return (
    <Atlas
      journeys={journeys}
      hrefOf={(q) => `/journey/${encodeURIComponent(q.key)}`}
      eyebrow="mikado"
      search
      version={version}
      banner={
        <>
          {error && <StaleBanner error={error} />}
          {data.github && <Banner>GitHub: {data.github}</Banner>}
        </>
      }
      empty={
        <Notice title="No journeys yet">
          <p>A journey is a small goal, crowned by one quest. Start one from the command line:</p>
          <Cli>mikado journey new "…"</Cli>
          <p>It shows up here within 15 seconds.</p>
        </Notice>
      }
    />
  )
}
