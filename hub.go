package main

import (
	"slices"
	"sync"
)

// client is one open SSE connection: the viewer's name plus the channel that
// connection's writer loop is listening on.
type client struct {
	name string
	send chan []string
}

// hub tracks who is currently viewing which report.
//
// The core decision: presence is *derived* from the set of open connections
// rather than stored and expired. There is no "join" or "leave" message, no
// heartbeat, and no TTL sweeper — a closed tab is a cancelled request context
// is a departed viewer, which the net/http server hands us for free. That
// removes the whole class of bugs where someone is shown as viewing a report
// they closed ten minutes ago.
//
// Presence is keyed by report, so two people on different reports never see
// each other and a broadcast only wakes the connections that care.
//
// State is in-memory and single-process, as the brief allows. Two instances
// behind a load balancer would each only know their own connections; the fix
// is a shared bus (Redis pub/sub, NATS) that each instance publishes joins and
// leaves to and merges into its local view. The hub interface below does not
// change — only broadcast and viewers do.
type hub struct {
	mu      sync.Mutex
	reports map[string]map[*client]bool
}

func newHub() *hub {
	return &hub{reports: make(map[string]map[*client]bool)}
}

// join registers c as a viewer of reportID and pushes the updated list to
// everyone on that report, including c itself.
func (h *hub) join(reportID string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.reports[reportID] == nil {
		h.reports[reportID] = make(map[*client]bool)
	}
	h.reports[reportID][c] = true
	h.broadcastLocked(reportID)
}

// leave removes c and pushes the updated list to whoever is left.
func (h *hub) leave(reportID string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.reports[reportID], c)
	if len(h.reports[reportID]) == 0 {
		delete(h.reports, reportID)
	}
	h.broadcastLocked(reportID)
}

// viewers returns the distinct names currently viewing reportID, sorted.
//
// Distinct and sorted both matter: one person with two tabs is two connections
// but one viewer, and an unsorted list would reshuffle the UI on every update
// because Go randomises map iteration.
func (h *hub) viewers(reportID string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.viewersLocked(reportID)
}

func (h *hub) viewersLocked(reportID string) []string {
	seen := make(map[string]bool, len(h.reports[reportID]))
	names := make([]string, 0, len(h.reports[reportID]))
	for c := range h.reports[reportID] {
		if !seen[c.name] {
			seen[c.name] = true
			names = append(names, c.name)
		}
	}
	slices.Sort(names)
	return names
}

// broadcastLocked hands the current viewer list to every connection on the
// report. Each send channel is buffered to one and drained first, so a client
// whose writer loop is busy gets the newest list rather than blocking the hub:
// every message is a complete snapshot, so a superseded one is worth nothing.
// Safe because the hub mutex makes broadcasts the only writer, one at a time.
func (h *hub) broadcastLocked(reportID string) {
	names := h.viewersLocked(reportID)
	for c := range h.reports[reportID] {
		select {
		case <-c.send:
		default:
		}
		c.send <- names
	}
}
