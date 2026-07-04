package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

// HandleScrubStream handles GET /api/pools/{name}/scrub/stream
// Streams real-time scrub progress via Server-Sent Events.
func (h *Handler) HandleScrubStream(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	poolName := ""
	for i, p := range parts {
		if p == "pools" && i+1 < len(parts) {
			poolName = parts[i+1]
			break
		}
	}
	if poolName == "" {
		http.Error(w, "pool name required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			statusJSON, err := zfs.GetScrubStatusJSON(poolName)
			if err != nil {
				fmt.Fprintf(w, "data: %s\n\n", json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error())))
			} else {
				fmt.Fprintf(w, "data: %s\n\n", statusJSON)
			}
			flusher.Flush()
		}
	}
}
