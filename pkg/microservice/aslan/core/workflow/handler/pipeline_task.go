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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func GetArtifactFile(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	notHistoryFileFlag, err := strconv.ParseBool(c.DefaultQuery("notHistoryFileFlag", "false"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid notHistoryFileFlag")
		return
	}

	resp, err := workflow.GetArtifactFileContent(c.Param("pipelineName"), taskID, notHistoryFileFlag, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	if notHistoryFileFlag {
		c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, setting.ArtifactResultOut))
	} else {
		c.Writer.Header().Set("Content-Disposition", `attachment; filename="artifact.tar.gz"`)
	}

	c.Data(200, "application/octet-stream", resp)
}
