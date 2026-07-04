package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

type eventBus struct {
	mu   sync.RWMutex
	subs map[chan *zfs.ZFSEvent]struct{}
}

var globalEventBus = &eventBus{
	subs: make(map[chan *zfs.ZFSEvent]struct{}),
}

func (b *eventBus) subscribe() chan *zfs.ZFSEvent {
	ch := make(chan *zfs.ZFSEvent, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBus) unsubscribe(ch chan *zfs.ZFSEvent) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *eventBus) publish(ev *zfs.ZFSEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func startEventBus(ctx context.Context) {
	go func() {
		ch := make(chan *zfs.ZFSEvent, 256)
		for {
			err := zfs.StreamEvents(ctx, ch)
			if ctx.Err() != nil {
				return
			}
			slog.Warn("zpool events stream ended, restarting", "err", err)
			time.Sleep(5 * time.Second)
			for len(ch) > 0 {
				<-ch
			}
		}
	}()
}

func (s *Handler) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := globalEventBus.subscribe()
	defer globalEventBus.unsubscribe(sub)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case ev, ok := <-sub:
			if !ok {
				return
			}
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
