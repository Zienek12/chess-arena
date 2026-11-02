package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"chess-arena/arena/internal/events"
	"chess-arena/arena/internal/store"
)

// heartbeatInterval keeps idle event streams open through proxies.
const heartbeatInterval = 15 * time.Second

// Events can be subscribed to.
type Events interface {
	Subscribe() (<-chan store.Event, func())
}

// streamEvents sends change notifications as server-sent events. The optional
// tournament_id and game_id query parameters narrow the stream.
func (server *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	tournamentID, gameID := r.URL.Query().Get("tournament_id"), r.URL.Query().Get("game_id")
	stream, unsubscribe := server.options.Events.Subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// Browsers reconnect after this many milliseconds when the stream drops.
	fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
		case event, open := <-stream:
			if !open {
				return
			}
			if event.Type != events.Resync &&
				((tournamentID != "" && event.TournamentID != tournamentID) || (gameID != "" && event.GameID != gameID)) {
				continue
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
		}
		flusher.Flush()
	}
}
