# Report Presence

Shows who is currently viewing a report, live. Viewers appear when they open a
report and disappear when they close it, with no action from anyone else.

Go backend (standard library only) + React/Vite frontend, streaming over
Server-Sent Events.

## Requirements

- Go 1.26.3 or later
- Node.js 20.19 or later in the 20.x release, or Node.js 22.12 or later
- npm

## Start the backend

Open a terminal in the repository root.

```sh
go run .
```

The backend listens at `http://localhost:8000`.

## Start the frontend

Open a second terminal in the repository root.

```sh
cd frontend
npm ci
npm run dev
```

Open `http://localhost:5173`.

Press `Ctrl+C` in each terminal to stop the servers.

## Try it

1. Open `http://localhost:5173` and enter a name, e.g. `Cole`.
2. Open a **second tab** at the same URL and enter `Sarah`. Both tabs now list
   Cole and Sarah, without either tab being reloaded.
3. Close Sarah's tab. She vanishes from Cole's list about as fast as you can
   look at it.
4. Switch to **Q3 Campaign Recap** in one tab. That viewer leaves the first
   report and appears on the second — presence is per report, not global.

The name is stored in `sessionStorage`, which is scoped per tab, so each new tab
is a different person. That is what makes this demoable in a single browser.

## How it works

```
browser                          Go server
  EventSource  ──GET /reports/{id}/presence?name=Cole──▶  register connection
             ◀──data: ["Cole","Sarah"]──────────────────  broadcast to report
  tab closed ──request context cancelled───────────────▶  drop + rebroadcast
```

### Presence is derived from connections, not stored

There is no join or leave message, no heartbeat, and no TTL sweeper. A viewer is
present exactly as long as their stream is open. When the tab closes, `net/http`
cancels the request context, the handler returns, and a deferred `leave` drops
them and rebroadcasts.

This is the decision the rest of the design falls out of. The alternative —
POST on arrival, POST on departure, expire stale entries on a timer — has to
answer "how long until we admit someone is gone?", and every answer is wrong:
too short and you evict live viewers, too long and you show ghosts. Tying
presence to the connection makes the question disappear rather than tuning it.

### SSE rather than WebSockets

Presence is a one-way server-to-client stream, so the client-to-server half of a
duplex channel would buy nothing. SSE is plain `net/http` with no dependency on
the server side, native `EventSource` on the client, and it reconnects on its own
after a dropped connection, which a raw WebSocket does not. The moment a feature
needs client-to-server chatter — live cursors, comments, typing indicators — that
is the point to switch.

### Presence is keyed by report

`map[reportID]map[*client]bool`. Two people on different reports never see each
other, and a broadcast only wakes the connections on the report that changed
rather than every connected client.

### One person is one viewer

Names are de-duplicated when the list is serialised, so two tabs is one entry,
and closing one of them does not evict you. The list is sorted, otherwise Go's
randomised map iteration would reshuffle the UI on every update.

### Slow clients do not block the hub

Each connection has a one-slot buffered channel that the broadcaster drains
before writing. A connection whose writer is busy gets the newest list instead
of stalling everyone else: every message is a complete snapshot, so a superseded
one is worth nothing.

## Tests

```sh
go test ./...
```

`presence_test.go` drives the real HTTP handler over a `httptest` server rather
than poking the hub directly, so it covers the part that actually breaks: a
viewer arriving shows up for everyone already watching, a viewer dropping their
connection disappears, two tabs collapse to one entry, closing one of two tabs
does not evict you, and reports stay isolated. Every assertion blocks on a real
broadcast rather than sleeping, so there are no timing races.

Not run here: `go test -race`, which needs cgo and a C compiler that this
machine does not have. Worth running wherever one is available.

## Deliberately out of scope

| Not done | Why, and when it would matter |
|---|---|
| Real authentication | Mocked per the brief. The name arrives as a query parameter because `EventSource` cannot set headers; real auth would read the viewer from a session cookie and ignore anything the client claims. Until then two people can type the same name and collapse into one entry — identity should be a stable user ID, not a display name. |
| Multi-instance support | State is in-memory, so two instances behind a load balancer would each only know their own connections. Fix is a shared bus (Redis pub/sub, NATS) that instances publish joins and leaves to and merge into their local view; the hub's callers do not change, only `broadcast` and `viewers` do. |
| Report catalogue | Two hardcoded reports in `frontend/src/App.tsx`. Presence does not care what a report ID refers to. |
| Avatars, "viewing for 3m", idle vs active | Polish. The payload is a list of names; anything richer is a bigger struct in the same broadcast. |
| Connection caps / rate limiting | One long-lived connection per open tab is the natural limit. A public deployment would want a per-IP cap. |
