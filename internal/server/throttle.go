// Package server provides a PostgreSQL Wire Protocol v3.0 compatible server.
// throttle.go implements connection rate limiting.
package server

import (
	"sync/atomic"
)

// Throttle tracks active connections and enforces a maximum.
type Throttle struct {
	active atomic.Int64
	max    int64
}

// NewThrottle creates a Throttle. max <= 0 means no limit.
func NewThrottle(max int64) *Throttle {
	return &Throttle{max: max}
}

// TryAcquire returns true if the connection is accepted.
func (t *Throttle) TryAcquire() bool {
	if t.max <= 0 {
		return true
	}
	cur := t.active.Add(1)
	if cur > t.max {
		t.active.Add(-1)
		return false
	}
	return true
}

// Release decrements the active connection count.
func (t *Throttle) Release() {
	t.active.Add(-1)
}

// Active returns the current active connection count.
func (t *Throttle) Active() int64 {
	return t.active.Load()
}
