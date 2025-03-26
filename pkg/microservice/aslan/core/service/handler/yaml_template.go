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
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	svcservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/types"
)

// @Summary Load service from yaml template
// @Description Load service from yaml template
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	production	query		bool										true	"is production"
// @Param 	body 		body 		svcservice.LoadServiceFromYamlTemplateReq 	true 	"body"
// @Success 200
// @Router /api/aslan/service/template/load [post]
func LoadServiceFromYamlTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(svcservice.LoadServiceFromYamlTemplateReq)

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	production := c.Query("production") == "true"
	detail := "项目管理-服务"
	if production {
		detail = "项目管理-生产服务"
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, req.ProjectName, "新增", detail, fmt.Sprintf("服务名称:%s", req.ServiceName), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[req.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[req.ProjectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[req.ProjectName].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[req.ProjectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[req.ProjectName].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = svcservice.LoadServiceFromYamlTemplate(ctx.UserName, req, false, production, ctx.Logger)
}

// @Summary Reload service from yaml template
// @Description Reload service from yaml template
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	production	query		bool										true	"is production"
// @Param 	body 		body 		svcservice.LoadServiceFromYamlTemplateReq 	true 	"body"
// @Success 200
// @Router /api/aslan/service/template/reload [post]
func ReloadServiceFromYamlTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(svcservice.LoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	production := c.Query("production") == "true"
	detail := "项目管理-服务"
	if production {
		detail = "项目管理-生产服务"
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, req.ProjectName, "更新", detail, fmt.Sprintf("服务名称:%s", req.ServiceName), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[req.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[req.ProjectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[req.ProjectName].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}
		} else {

			if !ctx.Resources.ProjectAuthInfo[req.ProjectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[req.ProjectName].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = svcservice.ReloadServiceFromYamlTemplate(ctx.UserName, req, production, ctx.Logger)
}

func PreviewServiceYamlFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(svcservice.LoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}
	req.ProjectName = c.Query("projectName")
	ctx.Resp, ctx.RespErr = svcservice.PreviewServiceFromYamlTemplate(req, ctx.Logger)
}
