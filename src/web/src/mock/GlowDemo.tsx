import { useState } from 'react'
import './mock.css'
import { useTheme } from './theme'

// A throwaway page to judge one idea: the selected card glowing from its own
// edges, switching on like a bulb, instead of an outline. Click a card; click again to clear.

const cards = [
  { id: 'a', ref: 'saves#88', title: 'Save migration for the winter items', hero: 'cyd' },
  { id: 'b', ref: 'game#138', title: 'Winter map: snow tiles, then the frozen-lake level', hero: 'bo' },
]

export default function GlowDemo() {
  const [theme] = useTheme()
  const [selected, setSelected] = useState<string | null>(null)

  return (
    <div data-theme={theme} className="quest-theme grid min-h-screen place-items-center" onClick={() => setSelected(null)}>
      <div className="flex gap-24">
        {cards.map((c) => {
          const on = selected === c.id
          const dim = selected !== null && !on
          return (
            <div key={c.id} className="relative">
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  setSelected(on ? null : c.id)
                }}
                style={{ background: 'var(--plate)', borderColor: 'var(--plate-border)' }}
                className={`relative flex w-80 flex-col gap-1.5 rounded-lg border-2 px-4 py-3 text-left transition duration-300 ${
                  dim ? 'opacity-25' : ''
                } ${on ? 'quest-lit -translate-y-0.5' : ''}`}
              >
                <span className="self-start rounded bg-[var(--chip)] px-1 font-mono text-[12px] text-[var(--ink-soft)]">{c.ref}</span>
                <span className="text-[15px] leading-snug font-semibold">{c.title}</span>
                <span className="text-[13px] text-[var(--ink-soft)]">{c.hero}</span>
              </button>
            </div>
          )
        })}
      </div>
    </div>
  )
}
