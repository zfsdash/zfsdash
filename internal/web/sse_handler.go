package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zfsdash/zfsdash/internal/events"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

// ScrubSSEHandler streams real-time scrub progress via Server-Sent Events.
// GET /api/pools/{name}/scrub/stream
func (s *Server) ScrubSSEHandler(w http.ResponseWriter, r *http.Request) {
	poolName := r.PathValue("name")
	if poolName == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}

	// Ensure the client supports SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial state immediately
	initial, err := zfs.ParseScrubStatus(poolName)
	if err == nil && initial != nil {
		sendSSEEvent(w, flusher, initial)
	}

	// Get or create event buffer for this pool
	buf := s.events.Buffer(poolName)
	ch := buf.Subscribe()
	defer buf.Unsubscribe(ch)

	// Keepalive ticker — SSE connections drop after 30s without data
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			sendSSEEvent(w, flusher, ev)
		case <-keepalive.C:
			// Send a comment to keep the connection alive
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func sendSSEEvent(w http.ResponseWriter, f http.Flusher, ev *events.ScrubProgressEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}
