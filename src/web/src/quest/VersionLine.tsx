import { parseVersion } from './version'

/** The running server's version as one faint line, e.g. `mikado v0.1.0-8-g4427d07`. */
export function VersionLine({ version, className = '' }: { version: string; className?: string }) {
  const { before, link, after } = parseVersion(version)
  return (
    <div className={`truncate font-mono text-[11px] leading-4 text-[var(--ink-faint)] ${className}`}>
      mikado {before}
      {link && (
        <a href={link.href} target="_blank" rel="noopener noreferrer" className="underline decoration-dotted underline-offset-2 hover:text-[var(--ink-soft)]">
          {link.text}
        </a>
      )}
      {after}
    </div>
  )
}
