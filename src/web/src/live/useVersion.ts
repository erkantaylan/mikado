import { useEffect, useState } from 'react'
import { fetchHealth } from '../api'

// Asked once per page load; a failure just leaves the version out.
let asked: Promise<string | undefined> | undefined

/** The running server's version (GET /api/health), once it has answered. */
export function useVersion(): string | undefined {
  const [version, setVersion] = useState<string>()
  useEffect(() => {
    let alive = true
    asked ??= fetchHealth().then(
      (h) => h.version,
      () => undefined,
    )
    void asked.then((v) => alive && setVersion(v))
    return () => {
      alive = false
    }
  }, [])
  return version
}
