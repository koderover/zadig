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
	"net/http"

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
	// skip if response or status is already set
	if c.Writer.Written() || c.Writer.Status() != http.StatusOK {
		return
	}

	if v, ok := c.Get(setting.ResponseError); ok {
		c.JSON(e.ErrorMessage(v.(error)))
		return
	}

	if v, ok := c.Get(setting.ResponseData); ok {
		setResponse(v, c)
	} else {
		c.JSON(200, gin.H{"message": "success"})
	}
}

func setResponse(resp interface{}, c *gin.Context) {
	switch r := resp.(type) {
	case string:
		c.String(200, r)
	case []byte:
		c.String(200, string(r))
	default:
		c.JSON(200, r)
	}
}
