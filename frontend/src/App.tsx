import { useEffect, useState, type FormEvent } from 'react'

const REPORTS = [
  { id: 'weekly-performance', title: 'Weekly Performance Report' },
  { id: 'monthly-revenue', title: 'Monthly Revenue Report' },
]

type Status = 'connecting' | 'live' | 'disconnected'

function useViewers(reportId: string, name: string) {
  const [viewers, setViewers] = useState<string[]>([])
  const [status, setStatus] = useState<Status>('connecting')

  useEffect(() => {
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${protocol}://${location.host}/api/reports/${reportId}/presence?name=${encodeURIComponent(name)}`

    let socket: WebSocket
    let retry: ReturnType<typeof setTimeout> | undefined
    let closed = false

    const connect = () => {
      socket = new WebSocket(url)
      socket.onopen = () => setStatus('live')
      socket.onmessage = (event) => setViewers(JSON.parse(event.data) as string[])
      socket.onclose = () => {
        if (closed) return
        setStatus('disconnected')
        retry = setTimeout(connect, 1000)
      }
    }
    connect()

    return () => {
      closed = true
      clearTimeout(retry)
      socket.close()
    }
  }, [reportId, name])

  return { viewers, status }
}

function Report({ reportId, name, onBack }: { reportId: string; name: string; onBack: () => void }) {
  const { viewers, status } = useViewers(reportId, name)
  const report = REPORTS.find((r) => r.id === reportId)!

  return (
    <main>
      <p>
        Signed in as <strong>{name}</strong> · <button type="button" onClick={onBack}>Change report</button>
      </p>
      <h1>{report.title}</h1>
      <h2>Currently viewing ({viewers.length})</h2>
      {status === 'connecting' && <p>Connecting…</p>}
      {status === 'disconnected' && <p>Connection lost. Reconnecting…</p>}
      <ul className="viewers">
        {viewers.map((viewer) => (
          <li key={viewer}>
            {viewer}
            {viewer === name && ' (you)'}
          </li>
        ))}
      </ul>
    </main>
  )
}

function App() {
  const [name, setName] = useState('')
  const [draftName, setDraftName] = useState('')
  const [reportId, setReportId] = useState<string | null>(null)

  const signIn = (event: FormEvent) => {
    event.preventDefault()
    const trimmed = draftName.trim()
    if (trimmed) setName(trimmed)
  }

  if (!name) {
    return (
      <main>
        <h1>Enter your name</h1>
        <form onSubmit={signIn}>
          <input
            autoFocus
            value={draftName}
            onChange={(e) => setDraftName(e.target.value)}
            placeholder="Your name"
          />
          <button type="submit">Continue</button>
        </form>
      </main>
    )
  }

  if (!reportId) {
    return (
      <main>
        <p>
          Signed in as <strong>{name}</strong>
        </p>
        <h1>Reports</h1>
        <ul className="reports">
          {REPORTS.map((report) => (
            <li key={report.id}>
              <button type="button" onClick={() => setReportId(report.id)}>
                {report.title}
              </button>
            </li>
          ))}
        </ul>
      </main>
    )
  }

  return <Report key={reportId} reportId={reportId} name={name} onBack={() => setReportId(null)} />
}

export default App
