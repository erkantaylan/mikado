import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react'
import { Crown, Search as SearchIcon, Trophy } from 'lucide-react'
import { fetchSearch, type Card, type QuestInfo, type SearchResult } from '../api'
import { ArchivedChip, QuestStateChip, stateColour, words } from './look'

// The search popup: quests and deeds by title, or straight to a deed by its id (M142) or
// issue (owner/repo#n). Ctrl+K / ⌘K or "/" opens it from anywhere on the page.

type Hit = { kind: 'quest'; quest: QuestInfo } | { kind: 'deed'; deed: Card; exact: boolean }

const questHref = (slug: string) => `/quest/${encodeURIComponent(slug)}`
const mac = typeof navigator !== 'undefined' && /Mac|iP(hone|ad)/.test(navigator.platform)

/** The chart a deed opens on: the one being viewed if the deed is on it, else its first quest. */
function chartOf(d: Card, here?: string): string | undefined {
  const slugs = (d.alsoIn ?? []).map((q) => q.slug)
  return here && slugs.includes(here) ? here : slugs[0]
}

export type SearchProps = {
  here?: string // the slug of the chart being viewed, if any
  select?: (key: string) => boolean // select a deed on that chart; false if it is not there
}

export function Search({ here, select }: SearchProps) {
  const [open, setOpen] = useState(false)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = e.target instanceof HTMLElement && (e.target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName))
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setOpen((o) => !o)
      } else if (e.key === '/' && !typing && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault()
        setOpen(true)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])
  return (
    <>
      <button
        onClick={() => setOpen(true)}
        aria-label="Search quests and deeds"
        title={`Search quests and deeds (${mac ? '⌘' : 'Ctrl'} K)`}
        className="flex h-10 items-center gap-2 rounded-md border border-[var(--panel-border)] px-3 text-[var(--ink-soft)] hover:text-[var(--ink)]"
      >
        <SearchIcon size={17} />
        <span className="hidden text-[14px] sm:inline">Search</span>
        <kbd className="hidden rounded border border-[var(--panel-border)] px-1 font-mono text-[11px] sm:inline">{mac ? '⌘K' : 'Ctrl K'}</kbd>
      </button>
      {open && <Popup here={here} select={select} close={() => setOpen(false)} />}
    </>
  )
}

function Popup({ here, select, close }: SearchProps & { close: () => void }) {
  const [q, setQ] = useState('')
  const [res, setRes] = useState<SearchResult>()
  const [error, setError] = useState<string>()
  const [active, setActive] = useState(0)
  const list = useRef<HTMLDivElement>(null)

  // Every keystroke asks again, a moment after typing stops; a newer query cancels an older one.
  useEffect(() => {
    const ctl = new AbortController()
    const t = setTimeout(
      () =>
        fetchSearch(q.trim(), ctl.signal).then(
          (r) => {
            setRes(r)
            setError(undefined)
            setActive(0)
          },
          (e) => !ctl.signal.aborted && setError(e instanceof Error ? e.message : String(e)),
        ),
      q ? 120 : 0,
    )
    return () => {
      clearTimeout(t)
      ctl.abort()
    }
  }, [q])

  const hits: Hit[] = res
    ? [
        ...(res.exact ? [{ kind: 'deed' as const, deed: res.exact, exact: true }] : []),
        ...res.quests.map((quest) => ({ kind: 'quest' as const, quest })),
        ...res.deeds.filter((d) => d.id !== res.exact?.id).map((deed) => ({ kind: 'deed' as const, deed, exact: false })),
      ]
    : []

  const go = (h: Hit | undefined) => {
    if (!h) return
    if (h.kind === 'quest') {
      location.assign(questHref(h.quest.slug))
      return
    }
    const slug = chartOf(h.deed, here)
    if (!slug) return // in no quest: no chart to show it on
    if (slug === here && select?.(h.deed.key ?? `M${h.deed.id}`)) close()
    else location.assign(`${questHref(slug)}?deed=${encodeURIComponent(h.deed.key ?? `M${h.deed.id}`)}`)
  }

  useEffect(() => {
    list.current?.querySelector(`[data-hit="${active}"]`)?.scrollIntoView({ block: 'nearest' })
  }, [active])

  const onKey = (e: ReactKeyboardEvent) => {
    if (e.key === 'Escape') close()
    else if (e.key === 'ArrowDown') setActive((a) => Math.min(a + 1, hits.length - 1))
    else if (e.key === 'ArrowUp') setActive((a) => Math.max(a - 1, 0))
    else if (e.key === 'Enter') go(hits[active])
    else return
    e.preventDefault()
  }

  const row = (h: Hit) => {
    const i = hits.indexOf(h)
    return (
      <div
        key={h.kind === 'quest' ? `q:${h.quest.slug}` : `d:${h.deed.id}`}
        data-hit={i}
        role="option"
        aria-selected={i === active}
        onMouseMove={() => setActive(i)}
        onClick={() => go(h)}
        className={`flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 ${i === active ? 'bg-[var(--chip)]' : ''}`}
      >
        {h.kind === 'quest' ? <QuestHit q={h.quest} /> : <DeedHit d={h.deed} exact={h.exact} here={here} />}
      </div>
    )
  }
  const exact = hits.filter((h) => h.kind === 'deed' && h.exact)
  const quests = hits.filter((h) => h.kind === 'quest')
  const deeds = hits.filter((h) => h.kind === 'deed' && !h.exact)

  return (
    <div
      className="fixed inset-0 z-50 bg-black/40 px-4 pt-[12vh]"
      onMouseDown={(e) => e.target === e.currentTarget && close()}
      role="dialog"
      aria-label="Search"
    >
      <div className="mx-auto flex max-h-[70vh] max-w-2xl flex-col overflow-hidden rounded-xl border border-[var(--panel-border)] bg-[var(--panel)] shadow-2xl">
        <div className="flex items-center gap-3 border-b border-[var(--panel-border)] px-4">
          <SearchIcon size={18} className="shrink-0 text-[var(--ink-soft)]" />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={onKey}
            placeholder="Search quests and deeds, or jump to M142 / owner/repo#n"
            aria-label="Search"
            className="h-14 min-w-0 flex-1 bg-transparent text-[16px] text-[var(--ink)] outline-none placeholder:text-[var(--ink-faint)]"
          />
          <kbd className="rounded border border-[var(--panel-border)] px-1.5 font-mono text-[11px] text-[var(--ink-soft)]">esc</kbd>
        </div>
        <div ref={list} role="listbox" className="overflow-y-auto p-2">
          {error && <div className="px-3 py-2 text-[14px] text-[#e11d48]">Search failed: {error}</div>}
          {exact.length > 0 && <Group title="Go to">{exact.map(row)}</Group>}
          {quests.length > 0 && <Group title="Quests">{quests.map(row)}</Group>}
          {deeds.length > 0 && <Group title="Deeds">{deeds.map(row)}</Group>}
          {res && !error && hits.length === 0 && (
            <div className="px-3 py-6 text-center text-[14px] text-[var(--ink-soft)]">{q.trim() ? `Nothing matches “${q.trim()}”.` : 'No quests yet.'}</div>
          )}
        </div>
        <div className="flex gap-4 border-t border-[var(--panel-border)] px-4 py-2 text-[12px] text-[var(--ink-faint)]">
          <span>↑↓ move</span>
          <span>↵ open</span>
          <span>esc close</span>
        </div>
      </div>
    </div>
  )
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="mb-1">
      <div className="px-3 pt-2 pb-1 text-[11px] font-bold tracking-[0.2em] text-[var(--ink-soft)] uppercase">{title}</div>
      {children}
    </div>
  )
}

function QuestHit({ q }: { q: QuestInfo }) {
  return (
    <>
      <Trophy size={17} className="shrink-0 text-[var(--gold)]" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[15px] font-semibold text-[var(--ink)]">{q.title}</span>
        <span className="block truncate font-mono text-[12px] text-[var(--ink-faint)]">{q.slug}</span>
      </span>
      <QuestStateChip state={q.state} />
      {q.archivedAt && <ArchivedChip />}
    </>
  )
}

function DeedHit({ d, exact, here }: { d: Card; exact: boolean; here?: string }) {
  const quests = d.alsoIn ?? []
  const kind = d.kind === 'issue' ? d.ref : d.kind === 'awaiting' ? 'Petition' : 'Errand'
  return (
    <>
      <span className={`w-14 shrink-0 font-mono text-[13px] font-semibold ${exact ? 'text-[var(--avail)]' : 'text-[var(--ink-soft)]'}`}>
        {d.key ?? `M${d.id}`}
      </span>
      <span className="min-w-0 flex-1">
        <span
          className={`flex items-center gap-1.5 truncate text-[15px] ${
            d.status === 'done' || d.status === 'cancelled' ? 'text-[var(--ink-soft)]' : 'font-semibold text-[var(--ink)]'
          } ${d.status === 'cancelled' ? 'line-through' : ''}`}
        >
          {d.final && <Crown size={14} className="shrink-0 text-[var(--gold)]" aria-label="crowning deed" />}
          <span className="truncate">{d.title}</span>
        </span>
        <span className="block truncate text-[12px] text-[var(--ink-faint)]">
          {kind} ·{' '}
          {quests.length === 0
            ? 'in no quest, so no chart to open'
            : quests.map((q) => (q.slug === here ? `${q.slug} (this chart)` : q.slug)).join(', ')}
        </span>
      </span>
      <span className="shrink-0 text-[12px] font-bold tracking-wider uppercase" style={{ color: stateColour[d.status] }}>
        {words[d.status]}
      </span>
    </>
  )
}
