package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MemoryCache is an in memory cache.Cache for lite deployments.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]memoryItem
	ttl   time.Duration
	stop  chan struct{}
	once  sync.Once
}

type memoryItem struct {
	data    []byte
	expires time.Time
}

func NewMemoryCache(defaultTTL time.Duration) *MemoryCache {
	c := &MemoryCache{
		items: make(map[string]memoryItem),
		ttl:   defaultTTL,
		stop:  make(chan struct{}),
	}
	go c.janitor()
	return c
}

func (c *MemoryCache) Get(_ context.Context, key string, dest interface{}) error {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || it.expired() {
		return fmt.Errorf("key not found: %s", key)
	}
	return json.Unmarshal(it.data, dest)
}

func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}) error {
	return c.SetWithTTL(ctx, key, value, c.ttl)
}

func (c *MemoryCache) SetWithTTL(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}

	c.mu.Lock()
	c.items[key] = memoryItem{data: data, expires: exp}
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Exists(_ context.Context, key string) (bool, error) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	return ok && !it.expired(), nil
}

func (c *MemoryCache) Increment(_ context.Context, key string, ttl time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var n int64
	it, ok := c.items[key]
	if ok && !it.expired() {
		_ = json.Unmarshal(it.data, &n)
	}
	n++

	data, _ := json.Marshal(n)
	exp := time.Now().Add(ttl)
	if ok && !it.expired() && !it.expires.IsZero() {
		exp = it.expires // keep the original window across increments
	} else if ttl <= 0 {
		exp = time.Time{}
	}

	c.items[key] = memoryItem{data: data, expires: exp}
	return n, nil
}

func (c *MemoryCache) Close() error {
	c.once.Do(func() { close(c.stop) })
	return nil
}

func (it memoryItem) expired() bool {
	return !it.expires.IsZero() && time.Now().After(it.expires)
}

func (c *MemoryCache) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.evictExpired()
		case <-c.stop:
			return
		}
	}
}

func (c *MemoryCache) evictExpired() {
	c.mu.Lock()
	for k, it := range c.items {
		if it.expired() {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}
