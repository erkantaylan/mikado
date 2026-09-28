import { useMemo } from 'react'
import { fetchJourneys, fetchRegions } from '../api'
import Atlas, { type AtlasRegion } from '../quest/Atlas'
import { Banner, Cli, Notice, NoticePage } from '../quest/Notice'
import { toAtlasJourney } from './adapt'
import { Failed, Loading, StaleBanner } from './states'
import { usePoll } from './usePoll'
import { useVersion } from './useVersion'

const load = async () => {
  const [atlas, regions] = await Promise.all([fetchJourneys(), fetchRegions()])
  return { ...atlas, regions }
}

const regionHref = (key: string) => `/region/${encodeURIComponent(key)}`

/**
 * `/`: the Atlas, every region the server knows as a card; `/region/:key` (regionKey set): that
 * region's journeys on their shelves. Both refresh every 15 s.
 */
export default function AtlasPage({ regionKey }: { regionKey?: string }) {
  const { data, error } = usePoll(load)
  const journeys = useMemo(() => data?.journeys.map(toAtlasJourney), [data])
  const regions = useMemo<AtlasRegion[] | undefined>(
    () => data?.regions.map((r) => ({ key: r.key, name: r.name, journeys: journeys?.filter((q) => q.region?.key === r.key) ?? [] })),
    [data, journeys],
  )
  const version = useVersion()
  if (!data || !journeys || !regions) return error ? <Failed error={error} what="the Atlas" /> : <Loading what="the Atlas" />
  const region = regionKey ? regions.find((r) => r.key.toLowerCase() === regionKey.toLowerCase()) : undefined
  if (regionKey && !region)
    return (
      <NoticePage title="No such region" back={{ href: '/', label: 'Atlas' }}>
        <p>
          There is no region <b className="font-mono">{regionKey}</b>.
        </p>
      </NoticePage>
    )
  return (
    <Atlas
      journeys={region ? region.journeys : journeys}
      regions={region ? undefined : regions}
      regionHref={(r) => regionHref(r.key)}
      hrefOf={(q) => `/journey/${encodeURIComponent(q.key)}`}
      eyebrow={
        region ? (
          <a href="/" className="hover:underline">
            ← Atlas<span className="ml-2 font-mono tracking-normal normal-case">{region.key}</span>
          </a>
        ) : (
          'mikado'
        )
      }
      title={region?.name}
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
          <Cli>{region ? `mikado journey new "…" --region ${region.key}` : 'mikado journey new "…"'}</Cli>
          <p>It shows up here within 15 seconds.</p>
        </Notice>
      }
    />
  )
}
