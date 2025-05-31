package main

import (
	"fmt"
	"log"
	"time"

	xcache "github.com/SipengXie/xcache"
)

func main() {
	// 创建一个支持string key和interface{} value的XCache
	// 默认32个bucket，每个bucket大小为100
	cache := xcache.NewXCache[string, interface{}](100).
		BucketCount(32).             // 设置32个bucket（默认值）
		LRU().                       // 使用LRU淘汰策略
		Expiration(time.Minute * 5). // 设置5分钟过期时间
		EvictedFunc(func(key string, value interface{}) {
			fmt.Printf("淘汰了key: %s, value: %v\n", key, value)
		}).
		AddedFunc(func(key string, value interface{}) {
			fmt.Printf("添加了key: %s, value: %v\n", key, value)
		}).
		Build()

	// 基本使用示例
	fmt.Println("=== 基本使用示例 ===")

	// 设置值
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

	// 获取值
	user1, err := cache.Get("user:1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("user:1 = %v\n", user1)

	// 检查key是否存在
	if cache.Has("user:2") {
		fmt.Println("user:2 存在于缓存中")
	}

	// 获取所有keys
	keys := cache.Keys(true)
	fmt.Printf("所有keys: %v\n", keys)

	// 获取缓存大小
	fmt.Printf("缓存大小: %d\n", cache.Len(true))

	// 创建一个支持int key和string value的XCache
	fmt.Println("\n=== 泛型使用示例 ===")
	intCache := xcache.NewXCache[int, string](50).
		BucketCount(16).
		LFU().
		LoaderFunc(func(key int) (string, error) {
			// 自动加载数据的函数
			return fmt.Sprintf("value-for-key-%d", key), nil
		}).
		Build()

	// 使用loader function自动加载
	value, err := intCache.Get(123)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("自动加载的值: %s\n", value)

	// 手动设置值
	err = intCache.Set(456, "manually-set-value")
	if err != nil {
		log.Fatal(err)
	}

	// 获取所有数据
	allData := intCache.GetAll(true)
	fmt.Printf("所有数据: %v\n", allData)

	// 展示统计信息
	fmt.Println("\n=== 缓存统计信息 ===")
	fmt.Printf("命中次数: %d\n", cache.HitCount())
	fmt.Printf("未命中次数: %d\n", cache.MissCount())
	fmt.Printf("查找次数: %d\n", cache.LookupCount())
	fmt.Printf("命中率: %.2f%%\n", cache.HitRate()*100)

	// 演示过期功能
	fmt.Println("\n=== 过期功能演示 ===")
	tempCache := xcache.NewXCache[string, string](10).
		BucketCount(4).
		Simple().
		Build()

	// 设置带过期时间的值
	err = tempCache.SetWithExpire("temp-key", "temp-value", time.Second*2)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("设置临时值，2秒后过期")

	// 立即获取
	tempValue, err := tempCache.Get("temp-key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("立即获取: %s\n", tempValue)

	// 等待3秒后再获取
	time.Sleep(3 * time.Second)
	_, err = tempCache.Get("temp-key")
	if err != nil {
		fmt.Printf("3秒后获取失败: %v\n", err)
	}

	// 清空缓存
	fmt.Println("\n=== 清空缓存 ===")
	fmt.Printf("清空前缓存大小: %d\n", cache.Len(true))
	cache.Purge()
	fmt.Printf("清空后缓存大小: %d\n", cache.Len(true))
}
