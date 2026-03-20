package cache

import (
	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
)

// DNSCache 包装了 LRU 缓存实现
type DNSCache struct {
	cache *lru.Cache
}

// New 创建一个新的 DNS 缓存实例
func New(limit int) (*DNSCache, error) {
	cache, err := lru.NewWithEvict(limit, func(key interface{}, value interface{}) {
		log.WithFields(log.Fields{"key": key}).Info("evicted cache")
	})
	if err != nil {
		return nil, err
	}
	return &DNSCache{cache: cache}, nil
}

// Get 获取缓存内容
func (c *DNSCache) Get(key interface{}) (interface{}, bool) {
	return c.cache.Get(key)
}

// Add 添加缓存内容
func (c *DNSCache) Add(key, value interface{}) bool {
	return c.cache.Add(key, value)
}

// Remove 删除缓存内容
func (c *DNSCache) Remove(key interface{}) {
	c.cache.Remove(key)
}

// Len 返回缓存条目数量
func (c *DNSCache) Len() int {
	return c.cache.Len()
}
