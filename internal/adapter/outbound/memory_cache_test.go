package outbound

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// Scenario: read a missing key, then insert and overwrite it; expect the latest cached value.
func TestMemoryCacheGetSet(t *testing.T) {
	cache := NewMemoryCache(time.Minute, time.Hour)
	t.Cleanup(cache.Close)
	if value, err := cache.Get("token"); err != nil || value != nil {
		t.Fatalf("miss = (%v, %v), want (nil, nil)", value, err)
	}
	if err := cache.Set("token", "first"); err != nil {
		t.Fatal(err)
	}
	if value, err := cache.Get("token"); err != nil || value != "first" {
		t.Fatalf("hit = (%v, %v), want (first, nil)", value, err)
	}
	if err := cache.Set("token", "second"); err != nil {
		t.Fatal(err)
	}
	if value, err := cache.Get("token"); err != nil || value != "second" {
		t.Fatalf("overwrite = (%v, %v), want (second, nil)", value, err)
	}
}

// Scenario: let a cached value pass its TTL; expect reads to miss before periodic cleanup removes the entry.
func TestMemoryCacheExpires(t *testing.T) {
	cache := NewMemoryCache(20*time.Millisecond, time.Hour)
	t.Cleanup(cache.Close)
	if err := cache.Set("token", "value"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if value, err := cache.Get("token"); err != nil || value != nil {
		t.Fatalf("expired entry = (%v, %v), want (nil, nil)", value, err)
	}
	cache.mu.RLock()
	_, ok := cache.entries["token"]
	cache.mu.RUnlock()
	if !ok {
		t.Fatal("Get removed the expired entry")
	}
}

// Scenario: disable cleanup and let a value expire; expect reads to miss while the entry remains stored.
func TestMemoryCacheExpiresWithoutPeriodicCleanup(t *testing.T) {
	cache := NewMemoryCache(20*time.Millisecond, 0)
	t.Cleanup(cache.Close)
	if err := cache.Set("token", "value"); err != nil {
		t.Fatal(err)
	}
	if value, err := cache.Get("token"); err != nil || value != "value" {
		t.Fatalf("fresh entry = (%v, %v), want (value, nil)", value, err)
	}
	time.Sleep(30 * time.Millisecond)
	if value, err := cache.Get("token"); err != nil || value != nil {
		t.Fatalf("expired entry = (%v, %v), want (nil, nil)", value, err)
	}
	cache.mu.RLock()
	_, exists := cache.entries["token"]
	cache.mu.RUnlock()
	if !exists {
		t.Fatal("Get removed expired entry without periodic cleanup")
	}
}

// Scenario: let the cleanup ticker run after expiry; expect the expired entry to be removed.
func TestMemoryCachePeriodicCleanup(t *testing.T) {
	cache := NewMemoryCache(20*time.Millisecond, 10*time.Millisecond)
	t.Cleanup(cache.Close)
	if err := cache.Set("token", "value"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		cache.mu.RLock()
		_, exists := cache.entries["token"]
		cache.mu.RUnlock()
		if !exists {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("periodic cleanup did not remove the expired entry")
}

// Scenario: close the cache twice and then add an entry; expect cleanup to remain stopped.
func TestMemoryCacheCloseStopsCleanup(t *testing.T) {
	cache := NewMemoryCache(20*time.Millisecond, 10*time.Millisecond)
	cache.Close()
	cache.Close()
	if err := cache.Set("token", "value"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	cache.mu.RLock()
	_, exists := cache.entries["token"]
	cache.mu.RUnlock()
	if !exists {
		t.Fatal("cleanup continued after Close")
	}
}

// Scenario: use zero or negative TTL; expect writes to be ignored and reads to miss.
func TestMemoryCacheNonPositiveTTL(t *testing.T) {
	for _, ttl := range []time.Duration{
		// Scenario: use zero TTL; expect writes to be ignored.
		0,
		// Scenario: use negative TTL; expect writes to be ignored.
		-time.Second,
	} {
		cache := NewMemoryCache(ttl, time.Hour)
		t.Cleanup(cache.Close)
		if err := cache.Set("token", "value"); err != nil {
			t.Fatal(err)
		}
		if value, err := cache.Get("token"); err != nil || value != nil {
			t.Fatalf("TTL %s: entry = (%v, %v), want (nil, nil)", ttl, value, err)
		}
	}
}

// Scenario: concurrently write and read distinct keys; expect each goroutine to retrieve its own value.
func TestMemoryCacheConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache(time.Minute, time.Hour)
	t.Cleanup(cache.Close)
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			key := strconv.Itoa(i)
			if err := cache.Set(key, i); err != nil {
				t.Errorf("set %s: %v", key, err)
				return
			}
			if value, err := cache.Get(key); err != nil || value != i {
				t.Errorf("get %s = (%v, %v), want (%d, nil)", key, value, err, i)
			}
		}(i)
	}
	workers.Wait()
}

// Scenario: concurrently refresh and read an expiring key; expect safe access without errors.
func TestMemoryCacheConcurrentExpiryAndRefresh(t *testing.T) {
	cache := NewMemoryCache(time.Nanosecond, time.Millisecond)
	t.Cleanup(cache.Close)
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			for n := 0; n < 100; n++ {
				if i%2 == 0 {
					if err := cache.Set("token", n); err != nil {
						t.Errorf("set token: %v", err)
					}
				} else if _, err := cache.Get("token"); err != nil {
					t.Errorf("get token: %v", err)
				}
			}
		}(i)
	}
	workers.Wait()
}
