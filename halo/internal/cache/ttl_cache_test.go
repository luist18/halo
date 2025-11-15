package cache

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTTLCacheGetOrComputeCachesValue(t *testing.T) {
	cache := NewTTLCache[string, int](50*time.Millisecond, WithCleanupInterval[string, int](0))
	defer cache.Close()

	var calls atomic.Int32

	compute := func() (int, error) {
		calls.Add(1)
		return 42, nil
	}

	val1, err := cache.GetOrCompute("key", compute)
	if err != nil {
		t.Fatalf("GetOrCompute returned error: %v", err)
	}

	val2, err := cache.GetOrCompute("key", compute)
	if err != nil {
		t.Fatalf("GetOrCompute returned error: %v", err)
	}

	if val1 != 42 || val2 != 42 {
		t.Fatalf("unexpected values: %d %d", val1, val2)
	}

	if calls.Load() != 1 {
		t.Fatalf("compute should be called once, got %d", calls.Load())
	}
}

func TestTTLCacheGetRefreshesTTL(t *testing.T) {
	ttl := 80 * time.Millisecond
	cache := NewTTLCache[string, string](ttl, WithCleanupInterval[string, string](0))
	defer cache.Close()

	cache.Set("key", "value")

	time.Sleep(ttl / 2)

	if _, ok := cache.Get("key"); !ok {
		t.Fatalf("expected value to exist after first half ttl")
	}

	time.Sleep(ttl/2 + 20*time.Millisecond)

	if val, ok := cache.Get("key"); !ok || val != "value" {
		t.Fatalf("expected value to remain after ttl refresh, ok=%v", ok)
	}
}

func TestTTLCacheEvictsExpiredEntries(t *testing.T) {
	evicted := make(chan struct{}, 1)
	cache := NewTTLCache[string, string](
		20*time.Millisecond,
		WithCleanupInterval[string, string](10*time.Millisecond),
		WithOnEvict[string, string](func(key, value string) {
			evicted <- struct{}{}
		}),
	)
	defer cache.Close()

	cache.Set("key", "value")

	time.Sleep(60 * time.Millisecond)

	if _, ok := cache.Get("key"); ok {
		t.Fatalf("expected entry to be evicted")
	}

	select {
	case <-evicted:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected eviction callback to be invoked")
	}
}
