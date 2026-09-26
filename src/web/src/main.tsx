import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import AtlasPage from './live/AtlasPage.tsx'
import JourneyPage from './live/JourneyPage.tsx'
import MockPage from './mock/MockPage.tsx'
import MockBoard from './mock/QuestBoard.tsx'
import GlowDemo from './mock/GlowDemo.tsx'
import { NoticePage } from './quest/Notice.tsx'

// Routing is the pathname, read once: every link is a full page load.
function route(path: string) {
  if (path === '/mock/glow') return <GlowDemo />
  if (path === '/mock') return <MockBoard />
  if (path.startsWith('/mock/quest/')) return <MockPage />
  if (path === '/') return <AtlasPage />
  const journey = path.match(/^\/journey\/([^/]+)\/?$/)
  if (journey) return <JourneyPage journeyKey={decodeURIComponent(journey[1])} />
  return (
    <NoticePage title="Nothing here" back={{ href: '/', label: 'Atlas' }}>
      <p>
        There is no page at <span className="font-mono">{path}</span>.
      </p>
    </NoticePage>
  )
}

createRoot(document.getElementById('root')!).render(<StrictMode>{route(location.pathname)}</StrictMode>)
