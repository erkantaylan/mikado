import { createContext } from 'react'
import type { Share } from '../quest/look'
import type { ThemeId } from '../quest/theme'

// What the header comparison pages (HeaderDemo, HeaderCDemo, HeaderMDemo) share: made-up data and the frame width.

export type Count = { title: string; done: number; total: number; shares: Share[]; extra: Share[] }

export type Case = {
  label: string
  chart: boolean // a journey's chart (emblem, the way back, a side panel); else the Atlas
  crumb: string
  jkey?: string
  title: string
  crown?: string // the crowning quest, for the subtitle
  holds?: string
  tally: Count
}

const chartTally: Count = {
  title: 'Main quest',
  done: 3,
  total: 8,
  shares: [
    { key: 'done', n: 3, label: 'fulfilled' },
    { key: 'underway', n: 1, label: 'underway' },
    { key: 'open', n: 1, label: 'open' },
    { key: 'awaiting', n: 0, label: 'awaiting reply' },
    { key: 'sealed', n: 3, label: 'sealed' },
  ],
  extra: [{ key: 'cancelled', n: 1, label: 'abandoned' }],
}

export const cases: Case[] = [
  { label: 'Chart, short title', chart: true, crumb: 'Kitchen', jkey: 'J3', title: 'Make an omelette', crown: 'Q9', holds: '8 quests · 4 side quests', tally: chartTally },
  {
    label: 'Chart, long title',
    chart: true,
    crumb: 'Side projects',
    jkey: 'J12',
    title: 'mikado can hook in any task tool, not only GitHub, and every quest source speaks one protocol',
    crown: 'Q31',
    holds: '8 quests · 2 side quests',
    tally: chartTally,
  },
  {
    label: 'Atlas',
    chart: false,
    crumb: 'mikado',
    title: 'Atlas',
    tally: {
      title: '4 journeys underway',
      done: 5,
      total: 22,
      shares: [
        { key: 'done', n: 5, label: 'fulfilled' },
        { key: 'open', n: 4, label: 'open now' },
        { key: 'awaiting', n: 1, label: 'awaiting reply' },
        { key: 'sealed', n: 12, label: 'sealed' },
      ],
      extra: [],
    },
  },
]

export const pct = (t: Count) => (t.total ? Math.round((t.done / t.total) * 100) : 0)
export const all = (t: Count) => [...t.shares, ...t.extra]
/** Every count in words, for a tooltip where the variant hides some of them. */
export const spelled = (t: Count) => `${t.title} ${t.done}/${t.total} · ` + all(t).map((s) => `${s.n} ${s.label}`).join(' · ')

// The page's theme, for the theme buttons inside the previews.
export const PageTheme = createContext<[ThemeId, (t: ThemeId) => void]>(['wartable', () => {}])

export const WIDTHS = ['1100', '1366', '1600', 'full'] as const
export type Width = (typeof WIDTHS)[number]

/** The frame width kept in ?w=. */
export function initialWidth(): Width {
  const w = new URLSearchParams(location.search).get('w')
  return WIDTHS.find((x) => x === w) ?? '1366'
}

export function keepWidth(w: Width) {
  history.replaceState(null, '', `?w=${w}`)
}
