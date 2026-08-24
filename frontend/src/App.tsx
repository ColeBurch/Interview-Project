import { useEffect, useState } from 'react'

// Hardcoded stand-ins for whatever the real report catalogue is. Two of them,
// because presence being scoped per report is the part worth demonstrating.
const REPORTS = [
  { id: 'weekly-performance', title: 'Weekly Performance Report' },
  { id: 'q3-campaign', title: 'Q3 Campaign Recap' },
]

const NAME_KEY = 'viewerName'

/**
 * Subscribes to the presence stream for one report.
 *
 * The open connection *is* the presence claim, so there is nothing to announce
 * on the way in and nothing to clean up on the way out: React's effect cleanup
 * closes the stream, the server sees the request context cancel, and the viewer
 * disappears for everyone else. Switching reports closes one stream and opens
 * the next, which reads on the server as a leave followed by a join.
 *
 * EventSource reconnects by itself if the backend restarts or the network
 * blips, so there is no retry logic here either.
 */
function usePresence(reportId: string, name: string) {
  const [snapshot, setSnapshot] = useState({ reportId: '', viewers: [] as string[], live: false })

  useEffect(() => {
    if (!name) return

    const source = new EventSource(
      `/api/reports/${encodeURIComponent(reportId)}/presence?name=${encodeURIComponent(name)}`,
    )
    // The first frame lands as soon as the server registers the join, so an
    // arrived frame is a better liveness signal than the socket opening.
    source.onmessage = (event) =>
      setSnapshot({ reportId, viewers: JSON.parse(event.data) as string[], live: true })
    source.onerror = () => setSnapshot((previous) => ({ ...previous, live: false }))

    return () => source.close()
  }, [reportId, name])

  // Discard a snapshot belonging to the report we just navigated away from,
  // rather than clearing state inside the effect and cascading a render.
  const current = snapshot.reportId === reportId
  return { viewers: current ? snapshot.viewers : [], live: current && snapshot.live }
}

function NameGate({ onSubmit }: { onSubmit: (name: string) => void }) {
  return (
    <form
      className="card"
      onSubmit={(event) => {
        event.preventDefault()
        const entered = String(new FormData(event.currentTarget).get('name') ?? '').trim()
        if (entered) onSubmit(entered)
      }}
    >
      <h1>Who&rsquo;s viewing?</h1>
      <p className="muted">Mocked sign-in — just enter a name.</p>
      <label htmlFor="name">Your name</label>
      <input id="name" name="name" required autoFocus autoComplete="off" maxLength={40} />
      <button type="submit">Open reports</button>
    </form>
  )
}

export default function App() {
  // sessionStorage, not localStorage: it is scoped per tab, so a second tab is
  // a second person. That is what makes this demoable in one browser.
  const [name, setName] = useState(() => sessionStorage.getItem(NAME_KEY) ?? '')
  const [reportId, setReportId] = useState(REPORTS[0].id)
  const { viewers, live } = usePresence(reportId, name)

  const signIn = (entered: string) => {
    sessionStorage.setItem(NAME_KEY, entered)
    setName(entered)
  }

  const signOut = () => {
    sessionStorage.removeItem(NAME_KEY)
    setName('')
  }

  if (!name) return <main><NameGate onSubmit={signIn} /></main>

  const report = REPORTS.find((candidate) => candidate.id === reportId) ?? REPORTS[0]

  return (
    <main>
      <nav>
        {REPORTS.map((candidate) => (
          <button
            key={candidate.id}
            type="button"
            className={candidate.id === report.id ? 'tab tab-active' : 'tab'}
            onClick={() => setReportId(candidate.id)}
          >
            {candidate.title}
          </button>
        ))}
      </nav>

      <section className="card">
        <h1>{report.title}</h1>

        <h2>
          Currently viewing
          <span className={live ? 'dot dot-live' : 'dot'} title={live ? 'Live' : 'Reconnecting'} />
        </h2>

        {/* The list changes without the user acting, so announce it. */}
        <ul aria-live="polite">
          {viewers.length === 0 && <li className="muted">Connecting&hellip;</li>}
          {viewers.map((viewer) => (
            <li key={viewer}>
              {viewer}
              {viewer === name && <span className="muted"> (you)</span>}
            </li>
          ))}
        </ul>
      </section>

      <button type="button" className="link" onClick={signOut}>
        Sign out
      </button>
    </main>
  )
}
