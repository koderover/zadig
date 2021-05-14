/*
Copyright 2021 The KodeRover Authors.

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

	"github.com/coocood/freecache"
)

const defaultSize = 1024 * 1024 * 10 // 10 Mi

var once sync.Once
var cache *freecache.Cache

// Set sets a key, value and expiration for a cache entry and stores it in the cache.
// If the key is larger than 65535 or value is larger than 1/1024 of the cache size,
// the entry will not be written to the cache. expireSeconds <= 0 means no expire,
// but it can be evicted when cache is full.
func Set(key, value []byte, expireSeconds int) error {
	once.Do(func() {
		cache = initCache(0)
	})
	return cache.Set(key, value, expireSeconds)
}

// Get returns the value or not found error.
func Get(key []byte) ([]byte, error) {
	once.Do(func() {
		cache = initCache(defaultSize)
	})
	return cache.Get(key)
}

func initCache(size int) *freecache.Cache {
	return freecache.NewCache(size)
}
