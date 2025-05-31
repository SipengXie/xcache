package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	xcache "github.com/SipengXie/xcache"
)

func main() {
	// 创建一个高性能的xcache实例
	cache := xcache.NewXCache[string, interface{}](50).
		BucketCount(8).               // 8个bucket
		LRU().                        // LRU淘汰策略
		Expiration(time.Second * 10). // 10秒过期
		EvictedFunc(func(key string, value interface{}) {
			fmt.Printf("🗑️  淘汰: %s\n", key)
		}).
		AddedFunc(func(key string, value interface{}) {
			fmt.Printf("✅ 添加: %s\n", key)
		}).
		Build()

	fmt.Println("=== XCache 高级功能演示 ===")
	fmt.Printf("Bucket数量: %d\n", cache.GetBucketCount())
	fmt.Println()

	// 1. 展示bucket分布
	fmt.Println("=== Bucket分布演示 ===")
	testKeys := []string{"user:1", "user:2", "user:3", "order:1", "order:2", "product:1"}

	for _, key := range testKeys {
		bucketIndex := cache.GetBucketIndex(key)
		cache.Set(key, fmt.Sprintf("value-for-%s", key))
		fmt.Printf("Key '%s' -> Bucket %d\n", key, bucketIndex)
	}
	fmt.Println()

	// 2. 展示bucket统计
	fmt.Println("=== Bucket统计信息 ===")
	bucketStats := cache.GetBucketStats()
	for i := 0; i < cache.GetBucketCount(); i++ {
		stats := bucketStats[i]
		fmt.Printf("Bucket %d: 大小=%d, 命中率=%.1f%%\n",
			i, stats["len"], stats["hit_rate"].(float64)*100)
	}
	fmt.Println()

	// 3. 并发测试
	fmt.Println("=== 并发性能测试 ===")
	concurrentTest(cache)
	fmt.Println()

	// 4. 过期和淘汰演示
	fmt.Println("=== 过期和淘汰演示 ===")
	expirationDemo(cache)
	fmt.Println()

	// 5. 类型安全演示
	fmt.Println("=== 类型安全演示 ===")
	typeSafetyDemo()
	fmt.Println()

	// 6. 最终统计
	fmt.Println("=== 最终统计信息 ===")
	fmt.Printf("总命中次数: %d\n", cache.HitCount())
	fmt.Printf("总未命中次数: %d\n", cache.MissCount())
	fmt.Printf("总查找次数: %d\n", cache.LookupCount())
	fmt.Printf("总命中率: %.2f%%\n", cache.HitRate()*100)
	fmt.Printf("当前缓存大小: %d\n", cache.Len(true))
}

func concurrentTest(cache *xcache.XCache[string, interface{}]) {
	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 100

	start := time.Now()

	// 并发写入测试
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

	// 并发读取测试
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

	fmt.Printf("并发写入: %d次操作, 耗时: %v, 速度: %.0f ops/sec\n",
		totalWrites, writeTime, float64(totalWrites)/writeTime.Seconds())
	fmt.Printf("并发读取: %d次操作, 耗时: %v, 速度: %.0f ops/sec\n",
		totalReads, readTime, float64(totalReads)/readTime.Seconds())
}

func expirationDemo(cache *xcache.XCache[string, interface{}]) {
	// 设置一些会过期的值
	cache.SetWithExpire("temp1", "临时值1", time.Second*2)
	cache.SetWithExpire("temp2", "临时值2", time.Second*3)

	fmt.Println("设置了2个临时值，分别在2秒和3秒后过期")

	// 立即读取
	val1, _ := cache.Get("temp1")
	val2, _ := cache.Get("temp2")
	fmt.Printf("立即读取: temp1=%v, temp2=%v\n", val1, val2)

	// 等待2.5秒
	time.Sleep(time.Millisecond * 2500)

	// 再次读取
	_, err1 := cache.Get("temp1")
	val2, err2 := cache.Get("temp2")
	fmt.Printf("2.5秒后: temp1错误=%v, temp2=%v (错误=%v)\n", err1, val2, err2)

	// 等待1秒
	time.Sleep(time.Second * 1)

	// 最后读取
	_, err1 = cache.Get("temp1")
	_, err2 = cache.Get("temp2")
	fmt.Printf("3.5秒后: temp1错误=%v, temp2错误=%v\n", err1, err2)
}

func typeSafetyDemo() {
	// 演示不同类型的缓存

	// 字符串到整数的缓存
	intCache := xcache.NewXCache[string, int](10).
		BucketCount(4).
		Simple().
		Build()

	intCache.Set("count", 42)
	intCache.Set("age", 25)

	count, _ := intCache.Get("count")
	age, _ := intCache.Get("age")
	fmt.Printf("整数缓存: count=%d (类型:%T), age=%d (类型:%T)\n", count, count, age, age)

	// 自定义结构体缓存
	type User struct {
		Name string
		Age  int
		City string
	}

	userCache := xcache.NewXCache[int, User](10).
		BucketCount(4).
		LRU().
		LoaderFunc(func(userID int) (User, error) {
			// 模拟从数据库加载用户
			return User{
				Name: fmt.Sprintf("User%d", userID),
				Age:  20 + userID,
				City: "北京",
			}, nil
		}).
		Build()

	// 自动加载用户
	user1, _ := userCache.Get(1)
	user2, _ := userCache.Get(2)

	fmt.Printf("用户缓存: user1=%+v, user2=%+v\n", user1, user2)

	// 展示类型安全：编译时就能检查类型错误
	// userCache.Set(1, "这会编译错误") // 这行如果取消注释会导致编译错误
}
