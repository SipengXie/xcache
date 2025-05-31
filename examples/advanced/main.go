package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	xcache "github.com/SipengXie/xcache"
)

func main() {
	// Create a high-performance xcache instance
	cache := xcache.NewXCache[string, interface{}](50).
		BucketCount(8).               // 8 buckets
		LRU().                        // LRU eviction strategy
		Expiration(time.Second * 10). // 10 seconds expiration
		EvictedFunc(func(key string, value interface{}) {
			fmt.Printf("🗑️  Evicted: %s\n", key)
		}).
		AddedFunc(func(key string, value interface{}) {
			fmt.Printf("✅ Added: %s\n", key)
		}).
		Build()

	fmt.Println("=== XCache Advanced Features Demo ===")
	fmt.Printf("Bucket count: %d\n", cache.GetBucketCount())
	fmt.Println()

	// 1. Demonstrate bucket distribution
	fmt.Println("=== Bucket Distribution Demo ===")
	testKeys := []string{"user:1", "user:2", "user:3", "order:1", "order:2", "product:1"}

	for _, key := range testKeys {
		bucketIndex := cache.GetBucketIndex(key)
		cache.Set(key, fmt.Sprintf("value-for-%s", key))
		fmt.Printf("Key '%s' -> Bucket %d\n", key, bucketIndex)
	}
	fmt.Println()

	// 2. Show bucket statistics
	fmt.Println("=== Bucket Statistics ===")
	bucketStats := cache.GetBucketStats()
	for i := 0; i < cache.GetBucketCount(); i++ {
		stats := bucketStats[i]
		fmt.Printf("Bucket %d: size=%d, hit_rate=%.1f%%\n",
			i, stats["len"], stats["hit_rate"].(float64)*100)
	}
	fmt.Println()

	// 3. Concurrent testing
	fmt.Println("=== Concurrent Performance Test ===")
	concurrentTest(cache)
	fmt.Println()

	// 4. Expiration and eviction demo
	fmt.Println("=== Expiration and Eviction Demo ===")
	expirationDemo(cache)
	fmt.Println()

	// 5. Type safety demo
	fmt.Println("=== Type Safety Demo ===")
	typeSafetyDemo()
	fmt.Println()

	// 6. Final statistics
	fmt.Println("=== Final Statistics ===")
	fmt.Printf("Total hit count: %d\n", cache.HitCount())
	fmt.Printf("Total miss count: %d\n", cache.MissCount())
	fmt.Printf("Total lookup count: %d\n", cache.LookupCount())
	fmt.Printf("Total hit rate: %.2f%%\n", cache.HitRate()*100)
	fmt.Printf("Current cache size: %d\n", cache.Len(true))
}

func concurrentTest(cache *xcache.XCache[string, interface{}]) {
	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 100

	start := time.Now()

	// Concurrent write test
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent:%d:%d", goroutineID, j)
				value := fmt.Sprintf("value:%d:%d", goroutineID, j)
				cache.Set(key, value)
			}
		}(i)
	}
	wg.Wait()

	writeTime := time.Since(start)
	totalWrites := numGoroutines * operationsPerGoroutine

	// Concurrent read test
	start = time.Now()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent:%d:%d", goroutineID, rand.Intn(operationsPerGoroutine))
				cache.Get(key)
			}
		}(i)
	}
	wg.Wait()

	readTime := time.Since(start)
	totalReads := numGoroutines * operationsPerGoroutine

	fmt.Printf("Concurrent writes: %d operations, time: %v, speed: %.0f ops/sec\n",
		totalWrites, writeTime, float64(totalWrites)/writeTime.Seconds())
	fmt.Printf("Concurrent reads: %d operations, time: %v, speed: %.0f ops/sec\n",
		totalReads, readTime, float64(totalReads)/readTime.Seconds())
}

func expirationDemo(cache *xcache.XCache[string, interface{}]) {
	// Set some values that will expire
	cache.SetWithExpire("temp1", "temporary value 1", time.Second*2)
	cache.SetWithExpire("temp2", "temporary value 2", time.Second*3)

	fmt.Println("Set 2 temporary values, expiring in 2 and 3 seconds respectively")

	// Read immediately
	val1, _ := cache.Get("temp1")
	val2, _ := cache.Get("temp2")
	fmt.Printf("Immediate read: temp1=%v, temp2=%v\n", val1, val2)

	// Wait 2.5 seconds
	time.Sleep(time.Millisecond * 2500)

	// Read again
	_, err1 := cache.Get("temp1")
	val2, err2 := cache.Get("temp2")
	fmt.Printf("After 2.5 seconds: temp1 error=%v, temp2=%v (error=%v)\n", err1, val2, err2)

	// Wait 1 more second
	time.Sleep(time.Second * 1)

	// Final read
	_, err1 = cache.Get("temp1")
	_, err2 = cache.Get("temp2")
	fmt.Printf("After 3.5 seconds: temp1 error=%v, temp2 error=%v\n", err1, err2)
}

func typeSafetyDemo() {
	// Demonstrate different types of caches

	// String to integer cache
	intCache := xcache.NewXCache[string, int](10).
		BucketCount(4).
		Simple().
		Build()

	intCache.Set("count", 42)
	intCache.Set("age", 25)

	count, _ := intCache.Get("count")
	age, _ := intCache.Get("age")
	fmt.Printf("Integer cache: count=%d (type:%T), age=%d (type:%T)\n", count, count, age, age)

	// Custom struct cache
	type User struct {
		Name string
		Age  int
		City string
	}

	userCache := xcache.NewXCache[int, User](10).
		BucketCount(4).
		LRU().
		LoaderFunc(func(userID int) (User, error) {
			// Simulate loading user from database
			return User{
				Name: fmt.Sprintf("User%d", userID),
				Age:  20 + userID,
				City: "Beijing",
			}, nil
		}).
		Build()

	// Automatically load users
	user1, _ := userCache.Get(1)
	user2, _ := userCache.Get(2)

	fmt.Printf("User cache: user1=%+v, user2=%+v\n", user1, user2)

	// Demonstrate type safety: type errors can be caught at compile time
	// userCache.Set(1, "This would cause compile error") // This line would cause compile error if uncommented
}
