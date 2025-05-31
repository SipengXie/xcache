package xcache

import (
	"testing"
	"time"
)

func TestXCachePeek(t *testing.T) {
	// Create a small LRU cache to test peek functionality
	cache := NewXCache[string, int](5).
		BucketCount(2).
		LRU().
		Build()

	// Test 1: Peek on empty cache should return error
	_, err := cache.Peek("nonexistent")
	if err != ErrKeyNotFoundError {
		t.Errorf("Expected ErrKeyNotFoundError, got %v", err)
	}

	// Test 2: Set some values
	cache.Set("key1", 100)
	cache.Set("key2", 200)
	cache.Set("key3", 300)

	// Test 3: Peek should return values without affecting LRU order
	value, err := cache.Peek("key1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if value != 100 {
		t.Errorf("Expected 100, got %v", value)
	}

	// Test 4: Get key1 to move it to front of LRU
	cache.Get("key1")

	// Test 5: Add more items to trigger eviction
	cache.Set("key4", 400)
	cache.Set("key5", 500)
	cache.Set("key6", 600) // This should evict key2 and key3

	// Test 6: key1 should still exist (was accessed via Get)
	value, err = cache.Peek("key1")
	if err != nil {
		t.Errorf("key1 should still exist: %v", err)
	}
	if value != 100 {
		t.Errorf("Expected 100, got %v", value)
	}

	// Test 7: Peek should not affect hit/miss statistics
	hitCountBefore := cache.HitCount()
	missCountBefore := cache.MissCount()

	cache.Peek("key1")        // Should not increment hit count
	cache.Peek("nonexistent") // Should not increment miss count

	hitCountAfter := cache.HitCount()
	missCountAfter := cache.MissCount()

	if hitCountAfter != hitCountBefore {
		t.Errorf("Peek should not affect hit count. Before: %d, After: %d", hitCountBefore, hitCountAfter)
	}
	if missCountAfter != missCountBefore {
		t.Errorf("Peek should not affect miss count. Before: %d, After: %d", missCountBefore, missCountAfter)
	}
}

func TestCachePeekAllTypes(t *testing.T) {
	testTypes := []string{TYPE_SIMPLE, TYPE_LRU, TYPE_LFU, TYPE_ARC, TYPE_LIRS}

	for _, cacheType := range testTypes {
		t.Run(cacheType, func(t *testing.T) {
			cache := New(10).EvictType(cacheType).Build()

			// Test basic peek functionality
			cache.Set("test", "value")

			// Peek should return the value
			value, err := cache.Peek("test")
			if err != nil {
				t.Errorf("%s: Unexpected error: %v", cacheType, err)
			}
			if value != "value" {
				t.Errorf("%s: Expected 'value', got %v", cacheType, value)
			}

			// Peek non-existent key should return error
			_, err = cache.Peek("nonexistent")
			if err != ErrKeyNotFoundError {
				t.Errorf("%s: Expected ErrKeyNotFoundError, got %v", cacheType, err)
			}
		})
	}
}

func TestPeekWithExpiration(t *testing.T) {
	cache := New(10).
		LRU().
		Expiration(10 * time.Millisecond).
		Build()

	// Set a value that will expire
	cache.Set("expiring", "value")

	// Peek before expiration
	value, err := cache.Peek("expiring")
	if err != nil {
		t.Errorf("Unexpected error before expiration: %v", err)
	}
	if value != "value" {
		t.Errorf("Expected 'value', got %v", value)
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Peek after expiration should return error
	_, err = cache.Peek("expiring")
	if err != ErrKeyNotFoundError {
		t.Errorf("Expected ErrKeyNotFoundError after expiration, got %v", err)
	}
}
