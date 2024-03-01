/*
Copyright 2023 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cache

import (
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/util"
)

// item is the data to be cached.
type item struct {
	value   string
	expires int64
}

type MemCache struct {
	noCache bool
	items   map[string]*item
	mu      sync.Mutex
	expires int64
}

// Expired determines if it has expires.
func (i *item) Expired(time int64) bool {
	if i.expires == 0 {
		return true
	}
	return time > i.expires
}

func NewMemCache(noCache bool) ICache {
	c := &MemCache{
		noCache: noCache,
		items:   make(map[string]*item),
		expires: int64(time.Minute * 60 * 24),
	}
	util.Go(func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				c.mu.Lock()
				for k, v := range c.items {
					if v.Expired(time.Now().UnixNano()) {
						delete(c.items, k)
					}
				}
				c.mu.Unlock()
			}
		}
	})
	return c
}

func (s *MemCache) Store(key string, data string) error {
	s.mu.Lock()
	if _, ok := s.items[key]; !ok {
		s.items[key] = &item{
			value:   data,
			expires: s.expires,
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *MemCache) Load(key string) (string, error) {
	s.mu.Lock()
	var result string
	if v, ok := s.items[key]; ok {
		result = v.value
	}
	s.mu.Unlock()
	return result, nil
}

func (s *MemCache) List() ([]string, error) {
	var ret []string
	s.mu.Lock()
	for _, v := range s.items {
		ret = append(ret, v.value)
	}
	s.mu.Unlock()
	return ret, nil
}

func (s *MemCache) Exists(key string) bool {
	s.mu.Lock()
	if _, ok := s.items[key]; ok {
		return true
	}
	s.mu.Unlock()
	return false
}

func (s *MemCache) IsCacheDisabled() bool {
	return s.noCache
}
