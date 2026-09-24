import { useEffect, useRef, useState } from 'react'
import { ApiError } from '../api'

export type Poll<T> = { data?: T; error?: ApiError; loading: boolean }

const EVERY_MS = 15_000

/**
 * Loads once, then again every 15 s and whenever the window regains focus.
 * An answer equal to the last one keeps the same object, so pages only
 * re-render for real changes. A failure keeps the last good data beside the error.
 */
export function usePoll<T>(load: () => Promise<T>): Poll<T> {
  const [state, setState] = useState<Poll<T>>({ loading: true })
  const last = useRef<string | null>(null)
  useEffect(() => {
    let alive = true
    let busy = false
    last.current = null
    const run = async () => {
      if (busy) return
      busy = true
      try {
        const data = await load()
        const text = JSON.stringify(data)
        if (!alive) return
        if (text === last.current) {
          setState((s) => (s.error || s.loading ? { data: s.data, loading: false } : s))
        } else {
          last.current = text
          setState({ data, loading: false })
        }
      } catch (e) {
        if (!alive) return
        const error = e instanceof ApiError ? e : new ApiError(0, e instanceof Error ? e.message : String(e), true)
        setState((s) => ({ data: s.data, error, loading: false }))
      } finally {
        busy = false
      }
    }
    run()
    const timer = setInterval(run, EVERY_MS)
    window.addEventListener('focus', run)
    return () => {
      alive = false
      clearInterval(timer)
      window.removeEventListener('focus', run)
    }
  }, [load])
  return state
}
