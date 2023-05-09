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

package gin

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/tool/metrics"
)

func RegisterRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Header.Get("Accept") == "text/event-stream" {
			return
		}
		startTime := time.Now().UnixMilli()
		path := c.Request.URL.Path
		c.Next()

		metrics.RegisterRequest(startTime, c.Request.Method, path, c.Writer.Status())
	}
}
