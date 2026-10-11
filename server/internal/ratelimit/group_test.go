package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentBurstAndRefill(t *testing.T) {
	g := New(60, 10)
	now := time.Now()
	g.now = func() time.Time { return now }
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := g.Allow("user"); ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 10 {
		t.Fatalf("accepted %d", accepted.Load())
	}
	if ok, wait := g.Allow("user"); ok || wait != time.Second {
		t.Fatalf("bad retry delay %v", wait)
	}
	if ok, _ := g.Allow("other"); !ok {
		t.Fatal("budgets not isolated")
	}
	now = now.Add(time.Second)
	if ok, _ := g.Allow("user"); !ok {
		t.Fatal("bucket did not refill")
	}
	if ok, _ := g.Allow("user"); ok {
		t.Fatal("bucket overfilled")
	}
}
func TestCapacityAndIdleEviction(t *testing.T) {
	g := New(60, 1)
	g.capacity = 2
	now := time.Now()
	g.now = func() time.Time { return now }
	g.Allow("a")
	g.Allow("b")
	if ok, _ := g.Allow("c"); ok {
		t.Fatal("capacity exceeded")
	}
	now = now.Add(11 * time.Minute)
	if ok, _ := g.Allow("c"); !ok {
		t.Fatal("idle entries not evicted")
	}
	if len(g.buckets) > 2 {
		t.Fatal("unbounded state")
	}
}
