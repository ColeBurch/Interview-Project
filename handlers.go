package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

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

func (a *app) presenceHandler(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("id")
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		a.logger.Error("websocket accept failed", "err", err)
		return
	}
	defer conn.CloseNow()

	v, leave := a.presence.join(reportID, name)
	defer leave()
	a.logger.Info("viewer joined", "report", reportID, "name", name)
	defer a.logger.Info("viewer left", "report", reportID, "name", name)

	// Write-only connection: CloseRead discards client messages, answers pings,
	// and cancels ctx when the client disconnects.
	ctx := conn.CloseRead(r.Context())

	for {
		select {
		case <-ctx.Done():
			return
		case names := <-v.updates:
			if err := wsjson.Write(ctx, conn, names); err != nil {
				return
			}
		}
	}
}
