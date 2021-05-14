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

package xlog

import (
	"github.com/gin-gonic/gin"
)

// DefaultKey ...
const DefaultKey = "xlog"

// DefaultLogger is get default logger middleware in gin.Context
func DefaultLogger(c *gin.Context) *Logger {
	v, ok := c.Get(DefaultKey)
	if !ok {
		xl := New(c.Writer, c.Request)
		c.Set(DefaultKey, xl)
		return xl
	}
	return v.(*Logger)
}
