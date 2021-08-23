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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// Response handle response
func Response() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer handleResponse(c)
		c.Next()
	}
}

func handleResponse(c *gin.Context) {
	// skip set response when previous middlewares  already do
	if c.Writer.Written() {
		return
	}

	if v, ok := c.Get(setting.ResponseError); ok {
		c.JSON(e.ErrorMessage(v.(error)))
		return
	}

	if v, ok := c.Get(setting.ResponseData); ok {
		c.JSON(200, v)
	} else {
		c.JSON(200, gin.H{"message": "success"})
	}

}
