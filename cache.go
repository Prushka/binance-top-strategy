package main

import (
	"sort"
	"strconv"
	"strings"
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

type MapCache[T any] struct {
	mutex       sync.Mutex
	Data        map[string]T
	FetchMethod func(key string) (T, error)
	HasExpired  func(value T) bool
}

func (c *MapCache[T]) Get(key string) (T, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, ok := c.Data[key]
	if !ok || c.HasExpired(c.Data[key]) {
		log.Infof("Cache expired %s, fetching new data", key)
		data, err := c.FetchMethod(key)
		if err != nil {
			log.Errorf("Error fetching data: %v", err)
			return c.Data[key], err
		}
		c.Data[key] = data
	}
	return c.Data[key], nil
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

func CreateMapCache[T any](fetchMethod func(key string) (T, error),
	hasExpired func(value T) bool) *MapCache[T] {
	cache := &MapCache[T]{
		FetchMethod: fetchMethod,
		HasExpired:  hasExpired,
		Data:        make(map[string]T),
	}
	return cache
}

var RoisCache = CreateMapCache[[]*Roi](
	func(key string) ([]*Roi, error) {
		split := strings.Split(key, "-")
		SID, _ := strconv.Atoi(split[0])
		UserId, _ := strconv.Atoi(split[1])
		roi, err := getStrategyRois(SID, UserId)
		if err != nil {
			return nil, err
		}
		for _, r := range roi {
			r.Time = r.Time / 1000
		}
		sort.Slice(roi, func(i, j int) bool {
			return roi[i].Time > roi[j].Time
		})
		return roi, nil
	},
	func(rois []*Roi) bool {
		latestTime := time.Unix(rois[0].Time*1000, 0)
		log.Infof("Latest time: %v", latestTime)
		log.Infof("Time since latest time: %v", time.Now().Sub(latestTime))
		if time.Now().Sub(latestTime) > 1*time.Hour {
			return true
		}
		return false
	},
)

var BracketsCache = CreateCache[*NotionalResponse](20*time.Minute,
	func() (*NotionalResponse, error) {
		return getBrackets()
	},
)
