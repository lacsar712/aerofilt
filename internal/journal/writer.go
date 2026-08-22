// Package journal appends Biofilter filter operation events for post-run review.
package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Event is one journal record.
type Event struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	FilterID string    `json:"filterId"`
	Payload   any       `json:"payload"`
}

// Writer appends JSONL events to disk with an in-memory ring for queries.
type Writer struct {
	mu    sync.Mutex
	path  string
	face  string
	ring  []Event
	limit int
	file  *os.File
}

// Open creates or appends a journal file.
func Open(path, filterID string, memLimit int) (*Writer, error) {
	if memLimit < 32 {
		memLimit = 32
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{path: path, face: filterID, ring: make([]Event, 0, memLimit), limit: memLimit, file: f}, nil
}

// Close flushes and closes the underlying file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// Append writes one event.
func (w *Writer) Append(kind string, payload any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	ev := Event{At: time.Now().UTC(), Kind: kind, FilterID: w.face, Payload: payload}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if w.file != nil {
		if _, err := w.file.Write(append(b, '\n')); err != nil {
			return fmt.Errorf("journal write: %w", err)
		}
	}
	w.ring = append(w.ring, ev)
	if len(w.ring) > w.limit {
		w.ring = w.ring[len(w.ring)-w.limit:]
	}
	return nil
}

// Recent returns in-memory events.
func (w *Writer) Recent() []Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]Event, len(w.ring))
	copy(out, w.ring)
	return out
}

// Path returns journal file path.
func (w *Writer) Path() string { return w.path }

// MemoryOnly creates a journal writer without disk backing.
func MemoryOnly(filterID string, memLimit int) *Writer {
	if memLimit < 32 {
		memLimit = 32
	}
	return &Writer{face: filterID, ring: make([]Event, 0, memLimit), limit: memLimit}
}
