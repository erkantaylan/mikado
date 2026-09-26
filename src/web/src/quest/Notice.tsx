import type { ReactNode } from 'react'
import { TriangleAlert } from 'lucide-react'
import './quest.css'
import { useTheme } from './theme'

/** A whole page with one thing to say: nothing here, the API is down, no such journey. */
export function NoticePage({ title, children, back }: { title: string; children: ReactNode; back?: { href: string; label: string } }) {
  const [theme] = useTheme()
  return (
    <div data-theme={theme} className="quest-theme grid min-h-screen place-items-center p-6">
      <Notice title={title} back={back}>
        {children}
      </Notice>
    </div>
  )
}

/** The same message as a card, for inside a page that is otherwise empty. */
export function Notice({ title, children, back }: { title: string; children: ReactNode; back?: { href: string; label: string } }) {
  return (
    <div className="mx-auto max-w-xl space-y-3 rounded-xl border-2 border-[var(--plate-border)] bg-[var(--plate)] p-6 text-center">
      <h1 className="quest-display text-xl font-semibold">{title}</h1>
      <div className="space-y-2 text-[15px] text-[var(--ink-soft)]">{children}</div>
      {back && (
        <a href={back.href} className="inline-block text-[12px] font-bold tracking-[0.2em] text-[var(--gold)] uppercase hover:underline">
          ← {back.label}
        </a>
      )}
    </div>
  )
}

/** A small strip under a page header for something worth knowing that blocks nothing. */
export function Banner({ children }: { children: ReactNode }) {
  return (
    <div
      role="status"
      className="flex items-center gap-2 border-b border-[var(--panel-border)] bg-[var(--await-plate)] px-5 py-1.5 text-[13px] text-[var(--ink)]"
    >
      <TriangleAlert size={14} className="shrink-0 text-[var(--await)]" />
      <span className="min-w-0">{children}</span>
    </div>
  )
}

/** Shell commands, shown as something to copy. */
export function Cli({ children }: { children: ReactNode }) {
  return (
    <code className="block rounded-md bg-[var(--chip)] px-3 py-2 text-left font-mono text-[13px] break-all text-[var(--ink)] select-all">
      {children}
    </code>
  )
}
