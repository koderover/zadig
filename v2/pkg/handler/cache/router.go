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

type Router struct {
	handlers CacheHandler
}

func NewRouter() *Router {
	return &Router{
		handlers: NewHandlers(),
	}
}

func (r *Router) Inject(router *gin.RouterGroup) {
	router.GET("/:key", r.handlers.Get)
	router.GET("/", r.handlers.List)

	router.POST("/:key/:value", r.handlers.Set)

	router.DELETE("/:key", r.handlers.Delete)
	router.DELETE("/", r.handlers.Purge)
}
