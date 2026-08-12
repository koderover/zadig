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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

// OperationLogStatus update status of operation if necessary
func OperationLogStatus() gin.HandlerFunc {
	return operationLogStatus(func(operationLog *models.OperationLog) {
		go func() {
			if err := mongodb.NewOperationLogColl().Insert(operationLog); err != nil {
				log.Errorf("failed to insert operation log: %v", err)
			}
		}()
	})
}

func operationLogStatus(insertAsync func(*models.OperationLog)) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		value, ok := c.Get(setting.OperationLog)
		if !ok {
			return
		}
		operationLog, ok := value.(*models.OperationLog)
		if !ok || operationLog == nil {
			return
		}

		operationLog.Status = c.Writer.Status()
		insertAsync(operationLog)
	}
}
