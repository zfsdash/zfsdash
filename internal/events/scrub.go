package events

import (
	"sync"
	"time"
)

// ScrubProgressEvent represents a snapshot of a running scrub.
type ScrubProgressEvent struct {
	Pool           string    `json:"pool"`
	ElapsedSeconds int64     `json:"elapsed_seconds"`
	ProgressPct    float64   `json:"progress_pct"`
	BytesPerSecond float64   `json:"bytes_per_second"`
	ETASeconds     int64     `json:"eta_seconds"`
	State          string    `json:"state"` // running, paused, finished, error
	Error          string    `json:"error,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

const bufferSize = 100

// EventBuffer is a fixed-size circular buffer of scrub events.
type EventBuffer struct {
	events [bufferSize]*ScrubProgressEvent
	pos    int
	full   bool
	mu     sync.RWMutex
	subs   []chan *ScrubProgressEvent
	subMu  sync.Mutex
}

func NewEventBuffer() *EventBuffer { return &EventBuffer{} }

func (eb *EventBuffer) Append(ev *ScrubProgressEvent) {
	eb.mu.Lock()
	eb.events[eb.pos] = ev
	eb.pos = (eb.pos + 1) % bufferSize
	if eb.pos == 0 {
		eb.full = true
	}
	eb.mu.Unlock()

	// Fan out to subscribers
	eb.subMu.Lock()
	for _, ch := range eb.subs {
		select {
		case ch <- ev:
		default: // drop if subscriber is slow
		}
	}
	eb.subMu.Unlock()
}

// Subscribe returns a channel that receives new events. Call Unsubscribe when done.
func (eb *EventBuffer) Subscribe() chan *ScrubProgressEvent {
	ch := make(chan *ScrubProgressEvent, 16)
	eb.subMu.Lock()
	eb.subs = append(eb.subs, ch)
	eb.subMu.Unlock()
	return ch
}

func (eb *EventBuffer) Unsubscribe(ch chan *ScrubProgressEvent) {
	eb.subMu.Lock()
	defer eb.subMu.Unlock()
	for i, s := range eb.subs {
		if s == ch {
			eb.subs = append(eb.subs[:i], eb.subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// Latest returns the most recent event or nil.
func (eb *EventBuffer) Latest() *ScrubProgressEvent {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	if eb.events[0] == nil {
		return nil
	}
	idx := (eb.pos - 1 + bufferSize) % bufferSize
	return eb.events[idx]
}

// EventManager tracks per-pool scrub event buffers.
type EventManager struct {
	buffers map[string]*EventBuffer
	mu      sync.RWMutex
}

func NewEventManager() *EventManager {
	return &EventManager{buffers: make(map[string]*EventBuffer)}
}

func (em *EventManager) Buffer(pool string) *EventBuffer {
	em.mu.Lock()
	defer em.mu.Unlock()
	if b, ok := em.buffers[pool]; ok {
		return b
	}
	b := NewEventBuffer()
	em.buffers[pool] = b
	return b
}
