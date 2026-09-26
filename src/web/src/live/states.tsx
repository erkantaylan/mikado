import type { ApiError } from '../api'
import { Banner, Cli, NoticePage } from '../quest/Notice'

export function Loading({ what }: { what: string }) {
  return (
    <NoticePage title="mikado">
      <p>Loading {what}…</p>
    </NoticePage>
  )
}

/** Nothing to show: the API is down, or it refused. */
export function Failed({ error, what }: { error: ApiError; what: string }) {
  if (error.down)
    return (
      <NoticePage title="The mikado API is down">
        <p>Could not load {what}: the server did not answer. Start it, and this page picks it up within 15 seconds:</p>
        <Cli>mikado serve</Cli>
      </NoticePage>
    )
  return (
    <NoticePage title={`Could not load ${what}`} back={{ href: '/', label: 'Atlas' }}>
      <p>{error.message}</p>
    </NoticePage>
  )
}

/** Something to show, but the last refresh failed. */
export function StaleBanner({ error }: { error: ApiError }) {
  return (
    <Banner>
      {error.down ? 'The mikado API is not answering' : `The last refresh failed: ${error.message}`} — showing what it said last.
    </Banner>
  )
}
