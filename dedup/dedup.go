package dedup

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

type Dedup struct {
	mu     sync.RWMutex
	items  map[string]time.Time
	ttl    time.Duration
	stopCh chan struct{}
}

func New(ttl time.Duration) *Dedup {
	d := &Dedup{
		items:  make(map[string]time.Time),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go d.cleanupLoop()
	return d
}

func (d *Dedup) Stop() {
	close(d.stopCh)
}

func (d *Dedup) Hash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

func (d *Dedup) Seen(hash string) bool {
	d.mu.RLock()
	_, ok := d.items[hash]
	d.mu.RUnlock()
	return ok
}

func (d *Dedup) Mark(hash string) {
	d.mu.Lock()
	d.items[hash] = time.Now()
	d.mu.Unlock()
}

func (d *Dedup) cleanupLoop() {
	ticker := time.NewTicker(d.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.mu.Lock()
			now := time.Now()
			for k, v := range d.items {
				if now.Sub(v) > d.ttl {
					delete(d.items, k)
				}
			}
			d.mu.Unlock()
		case <-d.stopCh:
			return
		}
	}
}
