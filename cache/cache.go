package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type entry struct {
	data      interface{}
	expiresAt time.Time
}

type Cache struct {
	lru *lru.Cache[string, *entry]
}

func New(size int) (*Cache, error) {
	l, err := lru.New[string, *entry](size)
	if err != nil {
		return nil, err
	}
	return &Cache{lru: l}, nil
}

func (c *Cache) Get(key string) (interface{}, bool) {
	e, ok := c.lru.Get(key)
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

func (c *Cache) Set(key string, val interface{}, ttl time.Duration) {
	c.lru.Add(key, &entry{
		data:      val,
		expiresAt: time.Now().Add(ttl),
	})
}
