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

import "github.com/gin-gonic/gin"

type Cacher interface {
	// Get gets data based on `key`.
	Get(key string) (value interface{}, exists bool)

	// Set sets data `key:values`.
	Set(key string, value interface{})

	// Delete deletes data based on `key`.
	Delete(key string)

	// List lists all of data.
	List() map[string]interface{}

	// Purge purges all of data.
	Purge()
}

type CacheHandler interface {
	// Get is a gin handler that gets cache data.
	Get(c *gin.Context)

	// Set is a gin handler that sets cache data.
	Set(c *gin.Context)

	// Delete is a gin handler that deletes cache data.
	Delete(c *gin.Context)

	// List is a gin handler that lists cache data.
	List(c *gin.Context)

	// Purge is a gin handler that purges cache data.
	Purge(c *gin.Context)
}
