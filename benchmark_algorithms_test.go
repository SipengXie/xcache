package xcache

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const (
	benchmarkCacheSize = 1000
	benchmarkDataSize  = 10000
)

// Benchmark basic operation performance
func BenchmarkAlgorithms_BasicOperations(b *testing.B) {
	algorithms := []struct {
		name string
		tp   string
	}{
		{"LIRS", TYPE_LIRS},
		{"LRU", TYPE_LRU},
		{"LFU", TYPE_LFU},
		{"ARC", TYPE_ARC},
	}

	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_Set", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i%benchmarkDataSize)
				cache.Set(key, fmt.Sprintf("value-%d", i))
			}
		})

		b.Run(fmt.Sprintf("%s_Get", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()

			// Pre-fill cache
			for i := 0; i < benchmarkCacheSize; i++ {
				cache.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i%benchmarkCacheSize)
				cache.Get(key)
			}
		})

		b.Run(fmt.Sprintf("%s_Mixed", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i%benchmarkDataSize)
				if i%4 == 0 { // 25% write operations
					cache.Set(key, fmt.Sprintf("value-%d", i))
				} else { // 75% read operations
					cache.Get(key)
				}
			}
		})
	}
}

// Benchmark performance under different access patterns
func BenchmarkAlgorithms_AccessPatterns(b *testing.B) {
	algorithms := []struct {
		name string
		tp   string
	}{
		{"LIRS", TYPE_LIRS},
		{"LRU", TYPE_LRU},
		{"LFU", TYPE_LFU},
		{"ARC", TYPE_ARC},
	}

	// 1. Sequential access pattern
	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_Sequential", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			hitCount := 0
			totalCount := 0

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("seq-key-%d", i%benchmarkDataSize)
				if _, err := cache.Get(key); err == nil {
					hitCount++
				} else {
					cache.Set(key, fmt.Sprintf("seq-value-%d", i))
				}
				totalCount++
			}

			hitRate := float64(hitCount) / float64(totalCount) * 100
			b.ReportMetric(hitRate, "hit_rate_%")
		})
	}

	// 2. Random access pattern
	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_Random", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			rand.Seed(42) // Fixed seed for reproducibility
			hitCount := 0
			totalCount := 0

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("rand-key-%d", rand.Intn(benchmarkDataSize))
				if _, err := cache.Get(key); err == nil {
					hitCount++
				} else {
					cache.Set(key, fmt.Sprintf("rand-value-%d", i))
				}
				totalCount++
			}

			hitRate := float64(hitCount) / float64(totalCount) * 100
			b.ReportMetric(hitRate, "hit_rate_%")
		})
	}

	// 3. Hotspot access pattern (80/20 rule)
	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_Hotspot", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			rand.Seed(42)
			hitCount := 0
			totalCount := 0
			hotDataSize := benchmarkDataSize / 5 // 20% of data

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var key string
				if rand.Float64() < 0.8 { // 80% access concentrated on 20% of data
					key = fmt.Sprintf("hot-key-%d", rand.Intn(hotDataSize))
				} else {
					key = fmt.Sprintf("cold-key-%d", rand.Intn(benchmarkDataSize))
				}

				if _, err := cache.Get(key); err == nil {
					hitCount++
				} else {
					cache.Set(key, fmt.Sprintf("value-%d", i))
				}
				totalCount++
			}

			hitRate := float64(hitCount) / float64(totalCount) * 100
			b.ReportMetric(hitRate, "hit_rate_%")
		})
	}

	// 4. Loop access pattern
	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_Loop", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			loopSize := benchmarkCacheSize * 2 // Loop size is twice the cache
			hitCount := 0
			totalCount := 0

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("loop-key-%d", i%loopSize)
				if _, err := cache.Get(key); err == nil {
					hitCount++
				} else {
					cache.Set(key, fmt.Sprintf("loop-value-%d", i))
				}
				totalCount++
			}

			hitRate := float64(hitCount) / float64(totalCount) * 100
			b.ReportMetric(hitRate, "hit_rate_%")
		})
	}
}

// Benchmark concurrent performance
func BenchmarkAlgorithms_Concurrent(b *testing.B) {
	algorithms := []struct {
		name string
		tp   string
	}{
		{"LIRS", TYPE_LIRS},
		{"LRU", TYPE_LRU},
		{"LFU", TYPE_LFU},
		{"ARC", TYPE_ARC},
	}

	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_Concurrent", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					key := fmt.Sprintf("concurrent-key-%d", i%benchmarkDataSize)
					if i%3 == 0 {
						cache.Set(key, fmt.Sprintf("concurrent-value-%d", i))
					} else {
						cache.Get(key)
					}
					i++
				}
			})
		})
	}
}

// Benchmark large-scale performance
func BenchmarkAlgorithms_LargeScale(b *testing.B) {
	algorithms := []struct {
		name string
		tp   string
	}{
		{"LIRS", TYPE_LIRS},
		{"LRU", TYPE_LRU},
		{"LFU", TYPE_LFU},
		{"ARC", TYPE_ARC},
	}

	largeCacheSize := 10000
	largeDataSize := 100000

	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_LargeScale", algo.name), func(b *testing.B) {
			cache := New(largeCacheSize).EvictType(algo.tp).Build()
			hitCount := 0
			totalCount := 0

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("large-key-%d", i%largeDataSize)
				if _, err := cache.Get(key); err == nil {
					hitCount++
				} else {
					cache.Set(key, fmt.Sprintf("large-value-%d", i))
				}
				totalCount++

				// Report progress every 1000 operations
				if totalCount%1000 == 0 && b.N > 1000 {
					hitRate := float64(hitCount) / float64(totalCount) * 100
					b.ReportMetric(hitRate, "hit_rate_%")
				}
			}

			if totalCount > 0 {
				hitRate := float64(hitCount) / float64(totalCount) * 100
				b.ReportMetric(hitRate, "final_hit_rate_%")
			}
		})
	}
}

// Benchmark specific workload pattern
func BenchmarkAlgorithms_WorkloadPatterns(b *testing.B) {
	algorithms := []struct {
		name string
		tp   string
	}{
		{"LIRS", TYPE_LIRS},
		{"LRU", TYPE_LRU},
		{"LFU", TYPE_LFU},
		{"ARC", TYPE_ARC},
	}

	// Database-like access pattern - Time locality strong
	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_DatabaseLike", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			rand.Seed(42)
			hitCount := 0
			totalCount := 0

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Simulate database access: Recently accessed data is more likely to be accessed
				var key string
				recentness := rand.Float64()
				if recentness < 0.6 {
					// 60% access recent 100 keys
					key = fmt.Sprintf("db-key-%d", benchmarkDataSize-rand.Intn(100))
				} else if recentness < 0.9 {
					// 30% access medium time data
					key = fmt.Sprintf("db-key-%d", benchmarkDataSize-rand.Intn(1000))
				} else {
					// 10% access old data
					key = fmt.Sprintf("db-key-%d", rand.Intn(benchmarkDataSize))
				}

				if _, err := cache.Get(key); err == nil {
					hitCount++
				} else {
					cache.Set(key, fmt.Sprintf("db-value-%d", i))
				}
				totalCount++
			}

			hitRate := float64(hitCount) / float64(totalCount) * 100
			b.ReportMetric(hitRate, "hit_rate_%")
		})
	}

	// Web cache-like access pattern - Frequency locality strong
	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_WebCacheLike", algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()
			rand.Seed(42)
			hitCount := 0
			totalCount := 0

			// Predefine some "popular pages"
			popularPages := make([]string, 50)
			for i := range popularPages {
				popularPages[i] = fmt.Sprintf("popular-page-%d", i)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var key string
				popularity := rand.Float64()
				if popularity < 0.7 {
					// 70% access popular pages
					key = popularPages[rand.Intn(len(popularPages))]
				} else {
					// 30% access random pages
					key = fmt.Sprintf("random-page-%d", rand.Intn(benchmarkDataSize))
				}

				if _, err := cache.Get(key); err == nil {
					hitCount++
				} else {
					cache.Set(key, fmt.Sprintf("web-content-%d", i))
				}
				totalCount++
			}

			hitRate := float64(hitCount) / float64(totalCount) * 100
			b.ReportMetric(hitRate, "hit_rate_%")
		})
	}
}

// Performance comparison utility function
func runAlgorithmComparison(b *testing.B, name string, workloadFunc func(Cache) (int, int)) {
	algorithms := []struct {
		name string
		tp   string
	}{
		{"LIRS", TYPE_LIRS},
		{"LRU", TYPE_LRU},
		{"LFU", TYPE_LFU},
		{"ARC", TYPE_ARC},
	}

	results := make(map[string]float64)

	for _, algo := range algorithms {
		b.Run(fmt.Sprintf("%s_%s", name, algo.name), func(b *testing.B) {
			cache := New(benchmarkCacheSize).EvictType(algo.tp).Build()

			b.ResetTimer()
			start := time.Now()

			hitCount, totalCount := workloadFunc(cache)

			duration := time.Since(start)
			hitRate := float64(hitCount) / float64(totalCount) * 100

			results[algo.name] = hitRate

			b.ReportMetric(hitRate, "hit_rate_%")
			b.ReportMetric(float64(duration.Nanoseconds())/float64(totalCount), "ns_per_op")
		})
	}
}

// Main comparison benchmark
func BenchmarkAlgorithms_Comparison(b *testing.B) {
	b.Log("=== Cache algorithm performance comparison ===")

	// Zipf distribution access pattern (simulate real world access pattern)
	runAlgorithmComparison(b, "ZipfDistribution", func(cache Cache) (int, int) {
		rand.Seed(42)
		hitCount := 0
		totalCount := 0

		// Generate access sequence of Zipf distribution
		for i := 0; i < 50000; i++ {
			// Simplified Zipf distribution: Access probability of rank k ∝ 1/k
			rank := int(1.0/rand.Float64()) % benchmarkDataSize
			key := fmt.Sprintf("zipf-key-%d", rank)

			if _, err := cache.Get(key); err == nil {
				hitCount++
			} else {
				cache.Set(key, fmt.Sprintf("zipf-value-%d", i))
			}
			totalCount++
		}

		return hitCount, totalCount
	})
}
