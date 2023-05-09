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
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/metrics"
)

func RegisterRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		startTime := time.Now().UnixMilli()
		path := c.Request.URL.Path
		c.Next()

		if strings.Contains(c.Request.URL.Path, "workflow/sse") {
			log.SugaredLogger().With("path", c.Request.URL.Path).
				With("cost", time.Since(start).String()).Infof("DEBUG-1")
		}
		metrics.RegisterRequest(startTime, c.Request.Method, path, c.Writer.Status())
	}
}
