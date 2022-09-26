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

package gin

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/setting"
)

const timeISO8601 = "2006-01-02T15:04:05.000Z0700"

var sensitiveHeaders = sets.NewString("authorization", "cookie", "token", "session")

func RequestLog(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		var body []byte
		headers := make(map[string]string)
		// request body is a ReadCloser, it can be read only once.
		if c.Request != nil {
			if c.Request.Body != nil {
				var buf bytes.Buffer
				tee := io.TeeReader(c.Request.Body, &buf)
				body, _ = ioutil.ReadAll(tee)
				c.Request.Body = io.NopCloser(&buf)
			}

			for k := range c.Request.Header {
				if sensitiveHeaders.Has(strings.ToLower(k)) {
					continue
				}
				headers[k] = c.GetHeader(k)
			}
		}

		// Process request
		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		logger.Info("",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.ByteString("body", body),
			zap.Any("headers", headers),
			zap.Int("size", c.Writer.Size()),
			zap.String("clientIP", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String(setting.RequestID, c.GetString(setting.RequestID)),
			zap.String("start", start.Format(timeISO8601)),
			zap.Duration("latency", latency),
			zap.String("error", c.Errors.ByType(gin.ErrorTypePrivate).String()),
		)
	}
}
