package main

import (
	"net"
	"testing"
	"time"
)

func initCache(t *testing.T) *cache {
	t.Helper()

	return &cache{items: make(map[cacheKey][]record)}
}

// compares an uncached (real network) DNS resolution against a cache hit 
// for the same answer, to demonstrate the improvement in 
// performance the cache provides. 
func TestCacheLatencyImprovement(t *testing.T) {
	const host = "example.com"

	if _, err := net.LookupHost(host); err != nil {
		t.Skipf("skipping: no network access available: %v", err)
	}

	ch := initCache(t)
	q := question{Name: host, QType: 1, Class: 1}
	reply := message{qn: q, ans: []record{aRec}}
	ch.addRecords(reply, reply)

	const iterations = 20

	uncachedStart := time.Now()
	for range iterations {
		if _, err := net.LookupHost(host); err != nil {
			t.Fatalf("uncached lookup failed: %v", err)
		}
	}
	uncachedElapsed := time.Since(uncachedStart)

	cachedStart := time.Now()
	for range iterations {
		if _, ok := ch.returnRecords(q); !ok {
			t.Fatal("expected cache hit")
		}
	}
	cachedElapsed := time.Since(cachedStart)

	t.Logf("uncached (network) resolution: %v total, %v/op", uncachedElapsed, uncachedElapsed/iterations)
	t.Logf("cached resolution:             %v total, %v/op", cachedElapsed, cachedElapsed/iterations)
	t.Logf("cache is ~%.0fx faster", float64(uncachedElapsed)/float64(cachedElapsed))

	if cachedElapsed >= uncachedElapsed {
		t.Errorf("expected cached lookups (%v) to be faster than uncached network lookups (%v)", cachedElapsed, uncachedElapsed)
	}
}

// BenchmarkCacheHit isolates the raw cost of a cache lookup, useful for
// tracking cache performance on its own (run with: go test -bench=CacheHit).
func BenchmarkCacheHit(b *testing.B) {
	ch := &cache{items: make(map[cacheKey][]record)}
	q := question{Name: "example.com", QType: 1, Class: 1}
	reply := message{qn: q, ans: []record{aRec}}
	ch.addRecords(reply, reply)

	b.ResetTimer()
	for range b.N {
		if _, ok := ch.returnRecords(q); !ok {
			b.Fatal("expected cache hit")
		}
	}
}
