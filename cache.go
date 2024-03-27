package main

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Cache[T any] struct {
	Data        T
	LastFetched time.Time
	TTL         time.Duration
	FetchMethod func() (T, error)
	mutex       sync.Mutex
}

func (c *Cache[T]) Get() (T, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.LastFetched.Add(c.TTL).Before(time.Now()) {
		log.Info("Cache expired, fetching new data")
		data, err := c.FetchMethod()
		if err != nil {
			return c.Data, err
		}
		c.Data = data
		c.LastFetched = time.Now()
	}
	return c.Data, nil
}

func CreateCache[T any](ttl time.Duration, fetchMethod func() (T, error)) *Cache[T] {
	cache := &Cache[T]{
		TTL:         ttl,
		FetchMethod: fetchMethod,
	}
	return cache
}

var BracketsCache = CreateCache[*NotionalResponse](20*time.Minute,
	func() (*NotionalResponse, error) {
		return getBrackets()
	},
)
