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

package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/webhook"
	e "github.com/koderover/zadig/lib/tool/errors"
)

func ProcessGitlabHook(c *gin.Context) {
	log := Logger(c)
	err := webhook.ProcessGitlabHook(c.Request, log)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}
	c.JSON(200, gin.H{"message": "success"})
}
