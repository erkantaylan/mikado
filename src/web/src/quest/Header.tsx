import { useState, type ReactNode } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { TallyLine, type TallyProps } from './look'
import { Search, type SearchProps } from './Search'
import { ThemeMenu, type ThemeId } from './theme'
import { IdTag, Sign } from './wood'

// Every page's header, in two decks: the way back and the title on one line with Search and the theme
// on the right, then the tally across the whole width. A chevron at the tally's right end folds the
// first deck away, leaving the tally alone; Ctrl K still opens search then.

const FOLD_KEY = 'mikado.header.folded'

/** Whether the header is folded, remembered per browser for every page. */
function useFolded(): [boolean, () => void] {
  const [folded, setFolded] = useState(() => {
    try {
      return localStorage.getItem(FOLD_KEY) === 'folded'
    } catch {
      return false // storage may be unavailable; unfolded is fine
    }
  })
  const toggle = () =>
    setFolded((f) => {
      try {
        localStorage.setItem(FOLD_KEY, f ? 'open' : 'folded')
      } catch {
        // not remembering is fine
      }
      return !f
    })
  return [folded, toggle]
}

export type HeaderProps = {
  back: { label: string; href?: string } // the sign top left: where it leads, or (no href) the app's name
  title: ReactNode // the h1's content: the title and any chips beside it
  tally: TallyProps
  journey?: { key: string; title: string } // on a chart: the journey's id, at the tally's left end
  search?: SearchProps // turns on search (the mock has none)
  theme: ThemeId
  onTheme: (t: ThemeId) => void
}

export function Header({ back, title, tally, journey, search, theme, onTheme }: HeaderProps) {
  const [folded, toggle] = useFolded()
  const say = folded ? 'Show the title and buttons' : 'Hide the title and buttons'
  return (
    <header className="quest-header quest-decks border-b border-[var(--panel-border)] bg-[var(--panel)]" data-folded={folded || undefined}>
      {folded ? (
        // No button, but its shortcut still works.
        search && <Search {...search} button={false} />
      ) : (
        <div className="quest-deck">
          <div className="quest-nameplate flex min-w-0 items-center gap-3">
            <Sign {...back} />
            <h1 className="quest-display flex min-w-0 items-center gap-2 text-xl font-semibold">{title}</h1>
          </div>
          <div className="quest-tools">
            {search && <Search {...search} />}
            <ThemeMenu theme={theme} onChange={onTheme} />
          </div>
        </div>
      )}
      <TallyLine
        {...tally}
        // Its tooltip names the journey too, for when the title is folded away.
        lead={journey && <IdTag id={journey.key} title={`${journey.key} · ${journey.title}`} />}
        end={
          <button
            onClick={toggle}
            aria-label={say}
            title={say}
            aria-expanded={!folded}
            className="quest-tool quest-fold grid place-items-center rounded-md border border-[var(--panel-border)] text-[var(--ink-soft)] hover:text-[var(--ink)]"
          >
            {folded ? <ChevronDown size={16} /> : <ChevronUp size={16} />}
          </button>
        }
      />
    </header>
  )
}
