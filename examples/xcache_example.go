package main

import (
	"fmt"
	"log"
	"time"

	xcache "github.com/SipengXie/xcache"
)

func main() {
	// Create an XCache that supports string key and interface{} value
	// Default 32 buckets, each bucket size is 100
	cache := xcache.NewXCache[string, interface{}](100).
		BucketCount(32).             // Set 32 buckets (default value)
		LRU().                       // Use LRU eviction strategy
		Expiration(time.Minute * 5). // Set 5 minutes expiration time
		EvictedFunc(func(key string, value interface{}) {
			fmt.Printf("Evicted key: %s, value: %v\n", key, value)
		}).
		AddedFunc(func(key string, value interface{}) {
			fmt.Printf("Added key: %s, value: %v\n", key, value)
		}).
		Build()

	// Basic usage examples
	fmt.Println("=== Basic Usage Examples ===")

	// Set values
	err := cache.Set("user:1", map[string]interface{}{
		"name": "Alice",
		"age":  30,
	})
	if err != nil {
		log.Fatal(err)
	}

	err = cache.Set("user:2", map[string]interface{}{
		"name": "Bob",
		"age":  25,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Get values
	user1, err := cache.Get("user:1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("user:1 = %v\n", user1)

	// Check if key exists
	if cache.Has("user:2") {
		fmt.Println("user:2 exists in cache")
	}

	// Get all keys
	keys := cache.Keys(true)
	fmt.Printf("All keys: %v\n", keys)

	// Get cache size
	fmt.Printf("Cache size: %d\n", cache.Len(true))

	// Create an XCache that supports int key and string value
	fmt.Println("\n=== Generic Usage Examples ===")
	intCache := xcache.NewXCache[int, string](50).
		BucketCount(16).
		LFU().
		LoaderFunc(func(key int) (string, error) {
			// Function to automatically load data
			return fmt.Sprintf("value-for-key-%d", key), nil
		}).
		Build()

	// Use loader function to automatically load data
	value, err := intCache.Get(123)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Auto-loaded value: %s\n", value)

	// Manually set value
	err = intCache.Set(456, "manually-set-value")
	if err != nil {
		log.Fatal(err)
	}

	// Get all data
	allData := intCache.GetAll(true)
	fmt.Printf("All data: %v\n", allData)

	// Show statistics
	fmt.Println("\n=== Cache Statistics ===")
	fmt.Printf("Hit count: %d\n", cache.HitCount())
	fmt.Printf("Miss count: %d\n", cache.MissCount())
	fmt.Printf("Lookup count: %d\n", cache.LookupCount())
	fmt.Printf("Hit rate: %.2f%%\n", cache.HitRate()*100)

	// Demonstrate expiration functionality
	fmt.Println("\n=== Expiration Functionality Demo ===")
	tempCache := xcache.NewXCache[string, string](10).
		BucketCount(4).
		Simple().
		Build()

	// Set value with expiration time
	err = tempCache.SetWithExpire("temp-key", "temp-value", time.Second*2)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Set temporary value, expires in 2 seconds")

	// Get immediately
	tempValue, err := tempCache.Get("temp-key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Immediate get: %s\n", tempValue)

	// Wait 3 seconds then get again
	time.Sleep(3 * time.Second)
	_, err = tempCache.Get("temp-key")
	if err != nil {
		fmt.Printf("Get after 3 seconds failed: %v\n", err)
	}

	// Clear cache
	fmt.Println("\n=== Clear Cache ===")
	fmt.Printf("Cache size before clear: %d\n", cache.Len(true))
	cache.Purge()
	fmt.Printf("Cache size after clear: %d\n", cache.Len(true))
}
