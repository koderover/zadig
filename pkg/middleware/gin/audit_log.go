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

	systemservice "github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/pkg/util/ginzap"
)

// OperationLogStatus update status of operation if necessary
func OperationLogStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer UpdateOperationLogStatus(c)
		c.Next()
	}
}

// 更新操作日志状态
func UpdateOperationLogStatus(c *gin.Context) {
	c.Next()
	if c.GetString("operationLogID") == "" {
		return
	}
	log := ginzap.WithContext(c).Sugar()
	err := systemservice.UpdateOperation(c.GetString("operationLogID"), c.Writer.Status(), log)
	if err != nil {
		log.Errorf("UpdateOperation err:%v", err)
	}
}
