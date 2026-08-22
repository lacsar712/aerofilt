package clock

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

type RealClock struct{}

func (RealClock) Now() time.Time                  { return time.Now().UTC() }
func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }

type ProcessClock struct {
	mu     sync.Mutex
	anchor time.Time
	offset time.Duration
}

func NewProcessClock(start time.Time) *ProcessClock {
	if start.IsZero() {
		start = time.Now().UTC()
	}
	return &ProcessClock{anchor: start.UTC()}
}

func (p *ProcessClock) Now() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.anchor.Add(p.offset)
}

func (p *ProcessClock) Since(t time.Time) time.Duration { return p.Now().Sub(t) }

func (p *ProcessClock) Advance(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset += d
}

func (p *ProcessClock) Reset(at time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.anchor = at.UTC()
	p.offset = 0
}
