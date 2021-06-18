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
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/ginzap"
)

// RequestID adds a unique request id to the context
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := util.UUID()
		ginzap.NewContext(c, zap.String(setting.RequestID, reqID))

		c.Set(setting.RequestID, reqID)

		c.Next()
	}
}
