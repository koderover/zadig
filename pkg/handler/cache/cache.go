/*
Copyright 2022 The KodeRover Authors.

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
	"fmt"
	"sync"
)

type cacheStore struct {
	store *sync.Map
}

func NewCache() Cacher {
	return &cacheStore{
		store: &sync.Map{},
	}
}

func (c *cacheStore) Get(key string) (value interface{}, exists bool) {
	return c.store.Load(key)
}

func (c *cacheStore) Set(key string, value interface{}) {
	c.store.Store(key, value)
}

func (c *cacheStore) Delete(key string) {
	c.store.Delete(key)
}

func (c *cacheStore) List() map[string]interface{} {
	data := map[string]interface{}{}
	c.store.Range(func(key, value interface{}) bool {
		data[fmt.Sprint(key)] = value

		return true
	})

	return data
}

func (c *cacheStore) Purge() {
	// Note:
	// Don't use `c.store = &sync.Map{}` because it's dangerous when there's a race condition on `c.store.`
	c.store.Range(func(key, value interface{}) bool {
		c.store.Delete(key)

		return true
	})
}
