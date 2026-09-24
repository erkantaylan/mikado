import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import BoardPage from './live/BoardPage.tsx'
import QuestPage from './live/QuestPage.tsx'
import MockPage from './mock/MockPage.tsx'
import MockBoard from './mock/QuestBoard.tsx'
import GlowDemo from './mock/GlowDemo.tsx'
import { NoticePage } from './quest/Notice.tsx'

// Routing is the pathname, read once: every link is a full page load.
function route(path: string) {
  if (path === '/mock/glow') return <GlowDemo />
  if (path === '/mock') return <MockBoard />
  if (path.startsWith('/mock/quest/')) return <MockPage />
  if (path === '/') return <BoardPage />
  const quest = path.match(/^\/quest\/([^/]+)\/?$/)
  if (quest) return <QuestPage slug={decodeURIComponent(quest[1])} />
  return (
    <NoticePage title="Nothing here" back={{ href: '/', label: 'Quest Board' }}>
      <p>
        There is no page at <span className="font-mono">{path}</span>.
      </p>
    </NoticePage>
  )
}

createRoot(document.getElementById('root')!).render(<StrictMode>{route(location.pathname)}</StrictMode>)
