package auth

import (
	"sync"
	"time"
)

type Blacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time
	done    chan struct{}
}

func NewBlacklist() *Blacklist {
	b := &Blacklist{
		entries: make(map[string]time.Time),
		done:    make(chan struct{}),
	}
	go b.prune()
	return b
}

func (b *Blacklist) Add(jti string, expiry time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = expiry
}

func (b *Blacklist) IsBlacklisted(jti string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	expiry, ok := b.entries[jti]
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

func (b *Blacklist) Stop() {
	close(b.done)
}

func (b *Blacklist) prune() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.mu.Lock()
			now := time.Now()
			for jti, expiry := range b.entries {
				if expiry.Before(now) {
					delete(b.entries, jti)
				}
			}
			b.mu.Unlock()
		case <-b.done:
			return
		}
	}
}
