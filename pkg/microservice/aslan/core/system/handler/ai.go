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

package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary Get System AI review config
// @Description Get System AI review config
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 		{object} 	commonmodels.AIReviewConfig
// @Router /api/aslan/system/ai/review [get]
func GetSystemAIReviewConfig(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetSystemAIReviewConfig(ctx)
}

// @Summary Update System AI review config
// @Description Update System AI review config
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	body 			body 		commonmodels.AIReviewConfig 	true 	"body"
// @Success 200
// @Router /api/aslan/system/ai/review [put]
func UpdateSystemAIReviewConfig(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.AIReviewConfig)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid update ai review config json args")
		return
	}

	ctx.RespErr = service.UpdateSystemAIReviewConfig(ctx, args)
}
