// Package ratelimit provides bounded, concurrency-safe per-identity token buckets.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

type bucket struct {
	tokens  float64
	updated time.Time
}
type Group struct {
	mu        sync.Mutex
	buckets   map[string]bucket
	perSecond float64
	burst     float64
	capacity  int
	idle      time.Duration
	now       func() time.Time
}

func New(perMinute, burst int) *Group {
	return NewWithClock(perMinute, burst, time.Now)
}

// NewWithClock allows deterministic verification of sustained polling budgets.
// The clock must be safe for concurrent calls and must not move backwards.
func NewWithClock(perMinute, burst int, now func() time.Time) *Group {
	if now == nil {
		panic("rate limit clock is required")
	}
	if perMinute <= 0 || burst <= 0 {
		panic("rate limits must be positive")
	}
	refill := time.Duration(float64(burst) / float64(perMinute) * float64(time.Minute))
	return &Group{buckets: make(map[string]bucket), perSecond: float64(perMinute) / 60, burst: float64(burst), capacity: 10000, idle: max(10*time.Minute, refill), now: now}
}

// Allow returns a retry delay without reserving future capacity on rejection.
func (g *Group) Allow(key string) (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	b, exists := g.buckets[key]
	if !exists {
		if len(g.buckets) >= g.capacity {
			for k, v := range g.buckets {
				if now.Sub(v.updated) >= g.idle {
					delete(g.buckets, k)
				}
			}
		}
		if len(g.buckets) >= g.capacity {
			return false, time.Minute
		}
		b = bucket{tokens: g.burst, updated: now}
	}
	b.tokens = math.Min(g.burst, b.tokens+math.Max(0, now.Sub(b.updated).Seconds())*g.perSecond)
	b.updated = now
	if b.tokens < 1 {
		g.buckets[key] = b
		return false, time.Duration(math.Ceil((1 - b.tokens) / g.perSecond * float64(time.Second)))
	}
	b.tokens--
	g.buckets[key] = b
	return true, 0
}
