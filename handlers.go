package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const maxNameLen = 40

func (a *app) helloHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"message": "Hello, Frontend!",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// presenceHandler streams the list of people currently viewing a report as
// Server-Sent Events.
//
// SSE rather than WebSockets: presence is a one-way server-to-client stream, so
// the extra half of a duplex channel would buy nothing. SSE is plain net/http
// with zero dependencies on this side, native EventSource on the other, and it
// reconnects on its own after a dropped connection — which a raw WebSocket does
// not. If a later feature needs client-to-server chatter (cursors, live
// comments), that is the point to reach for WebSockets.
//
// Auth is mocked per the brief: the viewer's name arrives as a query parameter
// because EventSource cannot set request headers. Real auth would identify the
// viewer from a session cookie and ignore anything the client claims.
func (a *app) presenceHandler(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("reportID")

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	// Trust boundary: this name is echoed to every other viewer's browser, so
	// bound it. Truncate by rune so a multi-byte character is never split.
	if runes := []rune(name); len(runes) > maxNameLen {
		name = string(runes[:maxNameLen])
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Proxies that buffer would defeat the point of a stream.
	w.Header().Set("X-Accel-Buffering", "no")

	rc := http.NewResponseController(w)

	c := &client{name: name, send: make(chan []string, 1)}
	a.hub.join(reportID, c)
	// The deferred leave is the whole disconnect story: when the tab closes the
	// request context is cancelled, this handler returns, and the viewer is
	// gone. No timers, nothing to reap.
	defer a.hub.leave(reportID, c)

	a.logger.Info("viewer joined", "report", reportID, "name", name)
	defer a.logger.Info("viewer left", "report", reportID, "name", name)

	for {
		select {
		case names := <-c.send:
			data, err := json.Marshal(names)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}
