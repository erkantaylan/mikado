const repo = 'https://github.com/erkantaylan/mikado'

/** A `git describe` string split around the part that links to GitHub. */
export type VersionParts = { before: string; link?: { text: string; href: string }; after: string }

/**
 * Splits the server's version (`git describe --tags --always --dirty`) so its commit can link to
 * GitHub: `v0.1.0-8-g4427d07` and a bare `4427d07` link the hash, an exact tag like `v0.2.0` links
 * its release. A `-dirty` suffix (uncommitted changes) stays as text; `dev` links nowhere.
 */
export function parseVersion(version: string): VersionParts {
  const dirty = version.endsWith('-dirty') ? '-dirty' : ''
  const v = version.slice(0, version.length - dirty.length)
  const hash = v.match(/^(.*-g|)([0-9a-f]{7,40})$/)
  if (hash) return { before: hash[1], link: { text: hash[2], href: `${repo}/commit/${hash[2]}` }, after: dirty }
  if (/^v\d[\w.+-]*$/.test(v)) return { before: '', link: { text: v, href: `${repo}/releases/tag/${v}` }, after: dirty }
  return { before: version, after: '' }
}
