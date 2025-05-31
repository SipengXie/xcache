package xcache

import (
	"fmt"
	"testing"
	"time"
)

func TestLIRSGet(t *testing.T) {
	size := 1000
	gc := buildTestCache(t, TYPE_LIRS, size)
	testSetCache(t, gc, size)
	testGetCache(t, gc, size)
}

func TestLoadingLIRSGet(t *testing.T) {
	size := 1000
	gc := buildTestLoadingCache(t, TYPE_LIRS, size, loader)
	testGetCache(t, gc, size)
}

func TestLIRSLength(t *testing.T) {
	gc := buildTestLoadingCache(t, TYPE_LIRS, 1000, loader)
	gc.Get("test1")
	gc.Get("test2")
	length := gc.Len(true)
	expectedLength := 2
	if length != expectedLength {
		t.Errorf("Expected length is %v, not %v", length, expectedLength)
	}
}

func TestLIRSEvictItem(t *testing.T) {
	cacheSize := 10
	numbers := 11
	gc := buildTestLoadingCache(t, TYPE_LIRS, cacheSize, loader)

	for i := 0; i < numbers; i++ {
		_, err := gc.Get(fmt.Sprintf("Key-%d", i))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestLIRSGetIFPresent(t *testing.T) {
	testGetIFPresent(t, TYPE_LIRS)
}

func TestLIRSHas(t *testing.T) {
	gc := buildTestLoadingCacheWithExpiration(t, TYPE_LIRS, 2, 10*time.Millisecond)

	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			gc.Get("test1")
			gc.Get("test2")

			if gc.Has("test0") {
				t.Fatal("should not have test0")
			}
			if !gc.Has("test1") {
				t.Fatal("should have test1")
			}
			if !gc.Has("test2") {
				t.Fatal("should have test2")
			}

			time.Sleep(20 * time.Millisecond)

			if gc.Has("test0") {
				t.Fatal("should not have test0")
			}
			if gc.Has("test1") {
				t.Fatal("should not have test1")
			}
			if gc.Has("test2") {
				t.Fatal("should not have test2")
			}
		})
	}
}

// Test LIRS specific behaviors

func TestLIRSSequentialAccess(t *testing.T) {
	// Test LIRS performance with sequential access pattern
	// LIRS should perform better than LRU for sequential scans
	cacheSize := 5
	gc := buildTestCache(t, TYPE_LIRS, cacheSize)

	// Fill cache with initial items
	for i := 0; i < cacheSize; i++ {
		gc.Set(fmt.Sprintf("frequent-%d", i), fmt.Sprintf("value-%d", i))
	}

	// Access frequent items multiple times to establish them as LIR blocks
	for j := 0; j < 3; j++ {
		for i := 0; i < cacheSize; i++ {
			gc.Get(fmt.Sprintf("frequent-%d", i))
		}
	}

	// Now simulate sequential scan with new items
	for i := 0; i < 10; i++ {
		gc.Set(fmt.Sprintf("scan-%d", i), fmt.Sprintf("scan-value-%d", i))
	}

	// Frequent items should still be available (LIRS advantage)
	hitCount := 0
	for i := 0; i < cacheSize; i++ {
		if gc.Has(fmt.Sprintf("frequent-%d", i)) {
			hitCount++
		}
	}

	// LIRS should retain some frequently accessed items
	if hitCount == 0 {
		t.Error("LIRS should retain some frequently accessed items during sequential scan")
	}
}

func TestLIRSLoopingAccess(t *testing.T) {
	// Test LIRS with looping access pattern
	cacheSize := 3
	gc := buildTestCache(t, TYPE_LIRS, cacheSize)

	// Create a loop larger than cache size
	loopSize := cacheSize + 2
	keys := make([]string, loopSize)
	for i := 0; i < loopSize; i++ {
		keys[i] = fmt.Sprintf("loop-key-%d", i)
	}

	// Execute the loop multiple times
	for round := 0; round < 5; round++ {
		for _, key := range keys {
			gc.Set(key, fmt.Sprintf("value-%s", key))
		}
	}

	// All items should have similar access patterns
	// LIRS should maintain a subset efficiently
	totalItems := gc.Len(true)
	if totalItems > cacheSize {
		t.Errorf("Cache size exceeded: expected <= %d, got %d", cacheSize, totalItems)
	}
}

func TestLIRSMixedWorkload(t *testing.T) {
	// Test LIRS with mixed workload: frequent and infrequent accesses
	cacheSize := 10
	gc := buildTestCache(t, TYPE_LIRS, cacheSize)

	// Add some frequently accessed items
	for i := 0; i < 3; i++ {
		gc.Set(fmt.Sprintf("hot-%d", i), fmt.Sprintf("hot-value-%d", i))
	}

	// Access hot items multiple times
	for round := 0; round < 5; round++ {
		for i := 0; i < 3; i++ {
			gc.Get(fmt.Sprintf("hot-%d", i))
		}
	}

	// Add many cold items
	for i := 0; i < 20; i++ {
		gc.Set(fmt.Sprintf("cold-%d", i), fmt.Sprintf("cold-value-%d", i))
	}

	// Hot items should still be in cache
	hotItemsInCache := 0
	for i := 0; i < 3; i++ {
		if gc.Has(fmt.Sprintf("hot-%d", i)) {
			hotItemsInCache++
		}
	}

	if hotItemsInCache == 0 {
		t.Error("LIRS should retain hot items even after cold item insertions")
	}
}

func TestLIRSExpiration(t *testing.T) {
	cacheSize := 5
	gc := New(cacheSize).
		LIRS().
		Expiration(100 * time.Millisecond).
		Build()

	// Add items with expiration
	for i := 0; i < cacheSize; i++ {
		gc.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	// Verify items are present
	if gc.Len(true) != cacheSize {
		t.Errorf("Expected %d items, got %d", cacheSize, gc.Len(true))
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Items should be expired
	if gc.Len(true) != 0 {
		t.Errorf("Expected 0 items after expiration, got %d", gc.Len(true))
	}
}

func TestLIRSPurge(t *testing.T) {
	cacheSize := 10
	purgeCount := 0
	gc := New(cacheSize).
		LIRS().
		LoaderFunc(loader).
		PurgeVisitorFunc(func(k, v interface{}) {
			purgeCount++
		}).
		Build()

	for i := 0; i < cacheSize; i++ {
		_, err := gc.Get(fmt.Sprintf("Key-%d", i))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	gc.Purge()

	if purgeCount != cacheSize {
		t.Errorf("Expected to purge %d items, purged %d", cacheSize, purgeCount)
	}

	if gc.Len(false) != 0 {
		t.Errorf("Expected cache to be empty after purge, got %d items", gc.Len(false))
	}
}

func TestLIRSConcurrentAccess(t *testing.T) {
	// Test LIRS under concurrent access
	cacheSize := 100
	gc := buildTestLoadingCache(t, TYPE_LIRS, cacheSize, loader)

	// Concurrent read/write test
	numGoroutines := 10
	numOperations := 100

	done := make(chan bool, numGoroutines*2)

	// Start writer goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("writer-%d-%d", id, j)
				gc.Set(key, fmt.Sprintf("value-%d-%d", id, j))
			}
			done <- true
		}(i)
	}

	// Start reader goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("reader-%d-%d", id, j%50) // Read some existing keys
				gc.Get(key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines*2; i++ {
		<-done
	}

	// Verify cache is still functional
	testKey := "test-after-concurrent"
	testValue := "test-value"
	gc.Set(testKey, testValue)

	retrievedValue, err := gc.Get(testKey)
	if err != nil {
		t.Errorf("Error retrieving value after concurrent access: %v", err)
	}
	if retrievedValue != testValue {
		t.Errorf("Expected %s, got %s", testValue, retrievedValue)
	}
}

func TestLIRSSpecificCases(t *testing.T) {
	// Test specific LIRS scenarios

	t.Run("HIR to LIR conversion", func(t *testing.T) {
		cacheSize := 3
		gc := buildTestCache(t, TYPE_LIRS, cacheSize)

		// Fill cache
		gc.Set("A", "valueA")
		gc.Set("B", "valueB")
		gc.Set("C", "valueC")

		// Access A multiple times to make it LIR
		for i := 0; i < 3; i++ {
			gc.Get("A")
		}

		// Add new item, should evict HIR block
		gc.Set("D", "valueD")

		// A should still be present (LIR block)
		if !gc.Has("A") {
			t.Error("LIR block A should still be present")
		}
	})

	t.Run("Stack pruning", func(t *testing.T) {
		cacheSize := 4
		gc := buildTestCache(t, TYPE_LIRS, cacheSize)

		// Add items in sequence
		for i := 0; i < 6; i++ {
			gc.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
		}

		// Cache should not exceed its size
		if gc.Len(true) > cacheSize {
			t.Errorf("Cache size exceeded: expected <= %d, got %d", cacheSize, gc.Len(true))
		}
	})
}

// Benchmark tests
func BenchmarkLIRSSet(b *testing.B) {
	gc := buildTestCache(nil, TYPE_LIRS, 1000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gc.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}
}

func BenchmarkLIRSGet(b *testing.B) {
	gc := buildTestCache(nil, TYPE_LIRS, 1000)

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		gc.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gc.Get(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkLIRSMixed(b *testing.B) {
	gc := buildTestCache(nil, TYPE_LIRS, 1000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%500)
		if i%2 == 0 {
			gc.Set(key, fmt.Sprintf("value-%d", i))
		} else {
			gc.Get(key)
		}
	}
}
