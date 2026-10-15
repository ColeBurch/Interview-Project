# Interview Project

## Requirements

Install these tools before you start:

- Go 1.26.3 or later
- Node.js 20.19 or later in the 20.x release, or Node.js 22.12 or later
- npm

## Start the backend

Open a terminal in the repository root.

```sh
go run .
```

Frontend (second terminal):

```sh
cd frontend
npm ci
npm run dev
```

Open `http://localhost:5173` in two or more browser windows, enter different
names, open the same report.

Tests:

```sh
go test ./...
cd frontend && npm run lint
```

## Architecture

```
Browser (React)                Go backend
┌───────────────────┐          ┌──────────────────────────────┐
│ WebSocket         │ upgrade  │ presenceHandler              │
│ /api/reports/{id} ├─────────►│   join(report, name)         │
│  /presence?name=  │◄─────────┤   push name list on change   │
│                   │ ["…"]    │   leave() on close/ctx.Done  │
└───────────────────┘          │ presence: map[report]set     │
                               └──────────────────────────────┘
```

### Decisions

**The connection is the presence.** Opening the WebSocket joins the report;
when the socket closes (tab closed, network dropped) the read loop fails, the
handler returns and `leave()` removes the viewer. No explicit leave request,
no stale entries when a tab is killed. This is the single most important
choice: presence derived from connection lifecycle cannot drift from reality.

**WebSocket via `github.com/coder/websocket`.** Go's stdlib has no WebSocket
server. `coder/websocket` is pure Go, small, context-aware, and the
maintained successor of `nhooyr.io/websocket`. The handler reads in one
goroutine (drains pings, detects close) and writes in the request goroutine.
The Vite dev proxy forwards upgrades with `ws: true`. The client reconnects
after one second on unexpected close; a reconnect simply re-joins.

**Full snapshot, not deltas.** Every change broadcasts the complete name list.
Clients replace state instead of applying patches, so a dropped message or a
reconnect cannot leave the UI inconsistent.

**In-memory, per report.** `presence` is a mutex-guarded
`map[reportID]set[viewer]`. Each viewer owns a buffered channel; a slow client
drops an intermediate update but always receives the latest one on the next
change.

**Identity is a name string.** Auth is mocked as required. The same name in
two tabs is two connections but one list entry, so a user with several tabs is
not shown twice.

## Evolving the system

1. **Real identity.** Replace `?name=` with a session or JWT. The viewer key
   becomes a user ID; display name is looked up server side.
2. **Multiple backend instances.** In-memory state does not survive horizontal
   scaling. Move the presence set to Redis (`SADD`/`SREM` + pub/sub, or a key
   per viewer with TTL refreshed by heartbeat) so any instance can serve any
   stream and still see all viewers.
3. **Restart resilience.** The client reconnects on close; because each
   connection re-joins, a restarted backend rebuilds state from live clients
   with no persistence needed. Add exponential backoff with jitter before
   this runs at scale.
4. **Deltas with sequence IDs.** If reports gain hundreds of viewers, send
   join/leave events instead of full snapshots and let a reconnecting client
   request replay from its last seen ID.
5. **Richer presence.** Cursor position, "editing" vs "viewing", idle state:
   the per-viewer struct grows and the broadcast payload becomes objects
   instead of names. The socket is already bidirectional, so clients send
   these as messages on the same connection.
6. **Heartbeats.** Add server-side ping/pong with a read deadline so a
   half-open connection (client vanished without a close frame) is reaped
   within seconds rather than waiting on TCP keepalive.
6. **Observability.** Gauge of connections per report and per instance;
   alert on connection churn.
