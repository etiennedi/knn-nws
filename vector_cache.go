package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// func newCache() *shardedVectorCache {
// 	shards := 200
// 	shardSize := 20

// 	cache := &shardedVectorCache{
// 		caches:     make([]map[int]*cacheItem, shards),
// 		shardLocks: make([]*sync.RWMutex, shards),
// 		targetSize: shardSize,
// 		// purgeSize:  5000,
// 		shards: shards,
// 	}

// 	for i := range cache.shardLocks {
// 		cache.caches[i] = map[int]*cacheItem{}
// 		cache.shardLocks[i] = &sync.RWMutex{}
// 	}

// 	return cache
// }

// func (c *shardedVectorCache) shard(item int) int {
// 	return item % c.shards
// }

// func (c *shardedVectorCache) get(i int) []float32 {

// 	shard := c.shard(i)

// 	before := time.Now()
// 	c.shardLocks[shard].RLock()
// 	m.addCacheReadLocking(before)
// 	item, ok := c.caches[shard][i]
// 	c.shardLocks[shard].RUnlock()

// 	if !ok {
// 		vec, err := readVectorFromBolt(int64(i))
// 		if err != nil {
// 			fmt.Printf("bolt read error: %v\n", err)
// 		}

// 		before := time.Now()
// 		c.shardLocks[shard].Lock()
// 		m.addCacheLocking(before)
// 		if len(c.caches[shard]) >= c.targetSize {
// 			c.purge(shard)
// 		}
// 		c.caches[shard][i] = &cacheItem{vector: vec, item: i, count: 1, lastUsed: time.Now()}
// 		c.shardLocks[shard].Unlock()
// 		return vec
// 	}

// 	// before = time.Now()
// 	// item.Lock()
// 	// m.addCacheItemLocking(before)
// 	// defer item.Unlock()

// 	// item.count += 1
// 	// item.lastUsed = time.Now()
// 	return item.vector
// }

// func (c *shardedVectorCache) purge(shard int) {
// 	// fmt.Printf("purging cache for shard %d!\n", shard)
// 	before := time.Now()
// 	defer m.addCachePurging(before)
// 	c.caches[shard] = map[int]*cacheItem{}

// 	// list := make([]*cacheItem, len(c.cache))
// 	// i := 0
// 	// for _, item := range c.cache {
// 	// 	list[i] = item
// 	// 	i++
// 	// }

// 	// sort.Slice(list, func(a, b int) bool { return list[a].lastUsed.Before(list[b].lastUsed) })
// 	// for i := 0; i < c.purgeSize; i++ {
// 	// 	delete(c.cache, list[i].item)
// 	// }

// }

// // func (c *shardedVectorCache) printCounts() {
// // 	list := make([]*cacheItem, len(c.cache))
// // 	i := 0
// // 	for _, item := range c.cache {
// // 		list[i] = item
// // 		i++
// // 	}

// // 	sort.Slice(list, func(a, b int) bool { return list[a].count > list[b].count })

// // 	now := time.Now()
// // 	for _, item := range list {
// // 		fmt.Printf("item %d - count %d - last used %s\n", item.item, item.count, time.Since(now))
// // 	}
// // }

type syncCache struct {
	cache   sync.Map
	count   int32
	maxSize int
}

func newCache() *syncCache {
	return &syncCache{
		cache:   sync.Map{},
		count:   0,
		maxSize: 10000,
	}

}

func (c *syncCache) get(i int) []float32 {
	before := time.Now()
	vec, ok := c.cache.Load(i)
	m.addCacheReadLocking(before)
	if !ok {
		before := time.Now()
		vec, err := readVectorFromBolt(int64(i))
		m.addReadingDisk(before)
		if err != nil {
			fmt.Printf("bolt read error: %v\n", err)
		}

		if c.count >= int32(c.maxSize) {
			before := time.Now()
			c.cache.Range(func(key, value interface{}) bool {
				c.cache.Delete(key)
				atomic.AddInt32(&c.count, -1)

				return true
			})
			m.addCachePurging(before)
		}

		before = time.Now()
		c.cache.Store(i, vec)
		m.addCacheLocking(before)
		atomic.AddInt32(&c.count, 1)
		return vec
	}

	return vec.([]float32)
}
