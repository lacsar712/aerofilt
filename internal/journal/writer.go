package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Writer struct {
	mu     sync.Mutex
	plant  string
	lines  []string
	cap    int
	file   *os.File
	closed bool
}

func MemoryOnly(plant string, capacity int) *Writer {
	if capacity <= 0 {
		capacity = 128
	}
	return &Writer{plant: plant, cap: capacity}
}

func (w *Writer) Append(event string, payload any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("journal closed")
	}
	data, err := json.Marshal(map[string]any{"plant": w.plant, "event": event, "payload": payload})
	if err != nil {
		return err
	}
	line := string(data)
	w.lines = append(w.lines, line)
	if len(w.lines) > w.cap {
		w.lines = w.lines[len(w.lines)-w.cap:]
	}
	if w.file != nil {
		_, err = w.file.WriteString(line + "\n")
	}
	return err
}

func (w *Writer) Lines() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.lines))
	copy(out, w.lines)
	return out
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
