import { useEffect, useRef, useState } from 'react'
import { Check, Palette } from 'lucide-react'

export const THEMES = [
  { id: 'parchment', name: 'Parchment' },
  { id: 'midnight', name: 'Midnight' },
  { id: 'medieval', name: 'Medieval' },
  { id: 'wartable', name: 'War table (preview)' },
] as const

export type ThemeId = (typeof THEMES)[number]['id']

const KEY = 'mikado.mock.theme'

function initial(): ThemeId {
  try {
    const saved = localStorage.getItem(KEY)
    if (THEMES.some((t) => t.id === saved)) return saved as ThemeId
  } catch {
    // storage may be unavailable; the default is fine
  }
  return 'parchment'
}

/** The mock's theme, remembered per browser so both pages agree. */
export function useTheme(): [ThemeId, (t: ThemeId) => void] {
  const [theme, setTheme] = useState<ThemeId>(initial)
  useEffect(() => {
    try {
      localStorage.setItem(KEY, theme)
    } catch {
      // not remembering is fine
    }
  }, [theme])
  return [theme, setTheme]
}

/** A small palette button that opens the theme list; the choice is rarely changed. */
export function ThemeMenu({ theme, onChange }: { theme: ThemeId; onChange: (t: ThemeId) => void }) {
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !box.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', close)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', close)
    }
  }, [open])
  return (
    <div ref={box} className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Theme"
        title="Theme"
        className="grid size-10 place-items-center rounded-md border border-[var(--panel-border)] text-[var(--ink-soft)] hover:text-[var(--ink)]"
      >
        <Palette size={18} />
      </button>
      {open && (
        <div
          role="menu"
          className="absolute top-full right-0 z-30 mt-1 w-44 rounded-lg border border-[var(--panel-border)] bg-[var(--panel)] p-1 shadow-lg"
        >
          {THEMES.map((t) => (
            <button
              key={t.id}
              role="menuitemradio"
              aria-checked={theme === t.id}
              onClick={() => {
                onChange(t.id)
                setOpen(false)
              }}
              className={`flex w-full items-center justify-between rounded-md px-3 py-1.5 text-left text-[14px] ${
                theme === t.id ? 'bg-[var(--chip)] font-semibold text-[var(--ink)]' : 'text-[var(--ink-soft)] hover:bg-[var(--chip)]'
              }`}
            >
              {t.name}
              {theme === t.id && <Check size={14} />}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
