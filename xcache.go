package xcache

import (
	"fmt"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

const (
	DefaultBucketCount = 32
)

// XCache is a bucket-based cache that supports generics
type XCache[K comparable, V any] struct {
	buckets     []Cache
	bucketCount int
	bucketSize  int
	mu          sync.RWMutex
	stats       *stats
}

// XCacheBuilder is the builder for XCache
type XCacheBuilder[K comparable, V any] struct {
	bucketCount      int
	bucketSize       int
	tp               string
	loaderExpireFunc LoaderExpireFunc
	evictedFunc      EvictedFunc
	purgeVisitorFunc PurgeVisitorFunc
	addedFunc        AddedFunc
	expiration       *time.Duration
	deserializeFunc  DeserializeFunc
	serializeFunc    SerializeFunc
	clock            Clock
}

// NewXCache creates a new XCacheBuilder
func NewXCache[K comparable, V any](bucketSize int) *XCacheBuilder[K, V] {
	return &XCacheBuilder[K, V]{
		bucketCount: DefaultBucketCount,
		bucketSize:  bucketSize,
		tp:          TYPE_LRU, // Default to use LRU
		clock:       NewRealClock(),
	}
}

// BucketCount sets the number of buckets
func (cb *XCacheBuilder[K, V]) BucketCount(count int) *XCacheBuilder[K, V] {
	if count <= 0 {
		count = DefaultBucketCount
	}
	cb.bucketCount = count
	return cb
}

// EvictType sets the eviction type for each bucket
func (cb *XCacheBuilder[K, V]) EvictType(tp string) *XCacheBuilder[K, V] {
	cb.tp = tp
	return cb
}

// Simple sets eviction type to simple
func (cb *XCacheBuilder[K, V]) Simple() *XCacheBuilder[K, V] {
	return cb.EvictType(TYPE_SIMPLE)
}

// LRU sets eviction type to LRU
func (cb *XCacheBuilder[K, V]) LRU() *XCacheBuilder[K, V] {
	return cb.EvictType(TYPE_LRU)
}

// LFU sets eviction type to LFU
func (cb *XCacheBuilder[K, V]) LFU() *XCacheBuilder[K, V] {
	return cb.EvictType(TYPE_LFU)
}

// ARC sets eviction type to ARC
func (cb *XCacheBuilder[K, V]) ARC() *XCacheBuilder[K, V] {
	return cb.EvictType(TYPE_ARC)
}

// LIRS sets eviction type to LIRS
func (cb *XCacheBuilder[K, V]) LIRS() *XCacheBuilder[K, V] {
	return cb.EvictType(TYPE_LIRS)
}

// LoaderFunc sets a loader function
func (cb *XCacheBuilder[K, V]) LoaderFunc(loaderFunc func(K) (V, error)) *XCacheBuilder[K, V] {
	cb.loaderExpireFunc = func(k interface{}) (interface{}, *time.Duration, error) {
		key, ok := k.(K)
		if !ok {
			return nil, nil, fmt.Errorf("invalid key type")
		}
		v, err := loaderFunc(key)
		return v, nil, err
	}
	return cb
}

// LoaderExpireFunc sets a loader function with expiration
func (cb *XCacheBuilder[K, V]) LoaderExpireFunc(loaderExpireFunc func(K) (V, *time.Duration, error)) *XCacheBuilder[K, V] {
	cb.loaderExpireFunc = func(k interface{}) (interface{}, *time.Duration, error) {
		key, ok := k.(K)
		if !ok {
			return nil, nil, fmt.Errorf("invalid key type")
		}
		return loaderExpireFunc(key)
	}
	return cb
}

// EvictedFunc sets an evicted function
func (cb *XCacheBuilder[K, V]) EvictedFunc(evictedFunc func(K, V)) *XCacheBuilder[K, V] {
	cb.evictedFunc = func(key, value interface{}) {
		k, ok := key.(K)
		if !ok {
			return
		}
		v, ok := value.(V)
		if !ok {
			return
		}
		evictedFunc(k, v)
	}
	return cb
}

// PurgeVisitorFunc sets a purge visitor function
func (cb *XCacheBuilder[K, V]) PurgeVisitorFunc(purgeVisitorFunc func(K, V)) *XCacheBuilder[K, V] {
	cb.purgeVisitorFunc = func(key, value interface{}) {
		k, ok := key.(K)
		if !ok {
			return
		}
		v, ok := value.(V)
		if !ok {
			return
		}
		purgeVisitorFunc(k, v)
	}
	return cb
}

// AddedFunc sets an added function
func (cb *XCacheBuilder[K, V]) AddedFunc(addedFunc func(K, V)) *XCacheBuilder[K, V] {
	cb.addedFunc = func(key, value interface{}) {
		k, ok := key.(K)
		if !ok {
			return
		}
		v, ok := value.(V)
		if !ok {
			return
		}
		addedFunc(k, v)
	}
	return cb
}

// Expiration sets the default expiration time
func (cb *XCacheBuilder[K, V]) Expiration(expiration time.Duration) *XCacheBuilder[K, V] {
	cb.expiration = &expiration
	return cb
}

// Clock sets the clock
func (cb *XCacheBuilder[K, V]) Clock(clock Clock) *XCacheBuilder[K, V] {
	cb.clock = clock
	return cb
}

// Build creates the XCache instance
func (cb *XCacheBuilder[K, V]) Build() *XCache[K, V] {
	if cb.bucketSize <= 0 && cb.tp != TYPE_SIMPLE {
		panic("xcache: bucket size <= 0")
	}

	xcache := &XCache[K, V]{
		buckets:     make([]Cache, cb.bucketCount),
		bucketCount: cb.bucketCount,
		bucketSize:  cb.bucketSize,
		stats:       &stats{},
	}

	// Create cache instance for each bucket
	for i := 0; i < cb.bucketCount; i++ {
		cacheBuilder := New(cb.bucketSize).
			EvictType(cb.tp).
			Clock(cb.clock)

		if cb.loaderExpireFunc != nil {
			cacheBuilder = cacheBuilder.LoaderExpireFunc(cb.loaderExpireFunc)
		}
		if cb.evictedFunc != nil {
			cacheBuilder = cacheBuilder.EvictedFunc(cb.evictedFunc)
		}
		if cb.purgeVisitorFunc != nil {
			cacheBuilder = cacheBuilder.PurgeVisitorFunc(cb.purgeVisitorFunc)
		}
		if cb.addedFunc != nil {
			cacheBuilder = cacheBuilder.AddedFunc(cb.addedFunc)
		}
		if cb.expiration != nil {
			cacheBuilder = cacheBuilder.Expiration(*cb.expiration)
		}
		if cb.deserializeFunc != nil {
			cacheBuilder = cacheBuilder.DeserializeFunc(cb.deserializeFunc)
		}
		if cb.serializeFunc != nil {
			cacheBuilder = cacheBuilder.SerializeFunc(cb.serializeFunc)
		}

		xcache.buckets[i] = cacheBuilder.Build()
	}

	return xcache
}

// hashKey uses xxhash to hash the key for better performance and distribution
func (xc *XCache[K, V]) hashKey(key K) uint64 {
	keyStr := fmt.Sprintf("%v", key)
	return xxhash.Sum64String(keyStr)
}

// getBucket returns the bucket for the given key
func (xc *XCache[K, V]) getBucket(key K) Cache {
	hash := xc.hashKey(key)
	bucketIndex := hash % uint64(xc.bucketCount)
	return xc.buckets[bucketIndex]
}

// Set inserts or updates the specified key-value pair
func (xc *XCache[K, V]) Set(key K, value V) error {
	bucket := xc.getBucket(key)
	return bucket.Set(key, value)
}

// SetWithExpire inserts or updates the specified key-value pair with an expiration time
func (xc *XCache[K, V]) SetWithExpire(key K, value V, expiration time.Duration) error {
	bucket := xc.getBucket(key)
	return bucket.SetWithExpire(key, value, expiration)
}

// Get returns the value for the specified key if it is present in the cache
func (xc *XCache[K, V]) Get(key K) (V, error) {
	bucket := xc.getBucket(key)
	value, err := bucket.Get(key)
	if err != nil {
		var zero V
		if err == ErrKeyNotFoundError {
			xc.stats.IncrMissCount()
		}
		return zero, err
	}

	xc.stats.IncrHitCount()
	if v, ok := value.(V); ok {
		return v, nil
	}

	var zero V
	return zero, fmt.Errorf("type assertion failed")
}

// GetIFPresent returns the value for the specified key if it is present in the cache
func (xc *XCache[K, V]) GetIFPresent(key K) (V, error) {
	bucket := xc.getBucket(key)
	value, err := bucket.GetIFPresent(key)
	if err != nil {
		var zero V
		if err == ErrKeyNotFoundError {
			xc.stats.IncrMissCount()
		}
		return zero, err
	}

	xc.stats.IncrHitCount()
	if v, ok := value.(V); ok {
		return v, nil
	}

	var zero V
	return zero, fmt.Errorf("type assertion failed")
}

// Peek returns the value for the specified key if it is present in the cache
// without updating any eviction algorithm statistics or positions.
// This is a pure read operation that does not affect cache state.
// Note: This method does not update hit/miss statistics.
func (xc *XCache[K, V]) Peek(key K) (V, error) {
	bucket := xc.getBucket(key)
	value, err := bucket.Peek(key)
	if err != nil {
		var zero V
		return zero, err
	}

	if v, ok := value.(V); ok {
		return v, nil
	}

	var zero V
	return zero, fmt.Errorf("type assertion failed")
}

// GetAll returns a map containing all key-value pairs in the cache
func (xc *XCache[K, V]) GetAll(checkExpired bool) map[K]V {
	result := make(map[K]V)
	xc.mu.RLock()
	defer xc.mu.RUnlock()

	for _, bucket := range xc.buckets {
		bucketItems := bucket.GetALL(checkExpired)
		for k, v := range bucketItems {
			if key, ok := k.(K); ok {
				if value, ok := v.(V); ok {
					result[key] = value
				}
			}
		}
	}

	return result
}

// Remove removes the specified key from the cache
func (xc *XCache[K, V]) Remove(key K) bool {
	bucket := xc.getBucket(key)
	return bucket.Remove(key)
}

// Purge removes all key-value pairs from the cache
func (xc *XCache[K, V]) Purge() {
	for _, bucket := range xc.buckets {
		bucket.Purge()
	}
}

// Keys returns a slice containing all keys in the cache
func (xc *XCache[K, V]) Keys(checkExpired bool) []K {
	var keys []K
	xc.mu.RLock()
	defer xc.mu.RUnlock()

	for _, bucket := range xc.buckets {
		bucketKeys := bucket.Keys(checkExpired)
		for _, k := range bucketKeys {
			if key, ok := k.(K); ok {
				keys = append(keys, key)
			}
		}
	}

	return keys
}

// Len returns the number of items in the cache
func (xc *XCache[K, V]) Len(checkExpired bool) int {
	totalLen := 0
	for _, bucket := range xc.buckets {
		totalLen += bucket.Len(checkExpired)
	}
	return totalLen
}

// Has returns true if the key exists in the cache
func (xc *XCache[K, V]) Has(key K) bool {
	bucket := xc.getBucket(key)
	return bucket.Has(key)
}

// HitCount returns hit count
func (xc *XCache[K, V]) HitCount() uint64 {
	return xc.stats.HitCount()
}

// MissCount returns miss count
func (xc *XCache[K, V]) MissCount() uint64 {
	return xc.stats.MissCount()
}

// LookupCount returns lookup count
func (xc *XCache[K, V]) LookupCount() uint64 {
	return xc.stats.LookupCount()
}

// HitRate returns rate for cache hitting
func (xc *XCache[K, V]) HitRate() float64 {
	return xc.stats.HitRate()
}

// GetBucketCount returns the number of buckets
func (xc *XCache[K, V]) GetBucketCount() int {
	return xc.bucketCount
}

// GetBucketIndex returns the bucket index for the given key (for debugging)
func (xc *XCache[K, V]) GetBucketIndex(key K) int {
	hash := xc.hashKey(key)
	return int(hash % uint64(xc.bucketCount))
}

// GetBucketStats returns statistics for each bucket
func (xc *XCache[K, V]) GetBucketStats() map[int]map[string]interface{} {
	result := make(map[int]map[string]interface{})
	for i, bucket := range xc.buckets {
		result[i] = map[string]interface{}{
			"len":        bucket.Len(true),
			"hit_count":  bucket.HitCount(),
			"miss_count": bucket.MissCount(),
			"hit_rate":   bucket.HitRate(),
		}
	}
	return result
}
