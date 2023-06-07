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
	"fmt"
	"sync"
)

type MemCache struct {
	noCache  bool
	mapCache sync.Map
}

func (s *MemCache) Store(key string, data string) error {
	s.mapCache.Store(key, data)
	return nil
}

func (s *MemCache) Load(key string) (string, error) {
	data, ok := s.mapCache.Load(key)
	if !ok {
		return "", fmt.Errorf("key %s not found", key)
	}
	ret, ok := data.(string)
	if !ok {
		return "", fmt.Errorf("key %s's value %v is not a string", key, data)
	}
	return ret, nil
}

func (s *MemCache) List() ([]string, error) {
	var ret []string
	s.mapCache.Range(func(key, value interface{}) bool {
		data, ok := value.(string)
		if !ok {
			return true
		}
		ret = append(ret, data)
		return true
	})
	return ret, nil
}

func (s *MemCache) Exists(key string) bool {
	_, ok := s.mapCache.Load(key)
	if !ok {
		return false
	}
	return true
}

func (s *MemCache) IsCacheDisabled() bool {
	return s.noCache
}
