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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util/boolptr"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	svcservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ListServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"
	removeApplicationLinked := c.Query("removeApplicationLinked") == "true"

	// authorization check
	// either they have the authorization, or they are system admins/project admins.
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.View {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.Resp, ctx.RespErr = commonservice.ListServiceTemplate(projectName, production, removeApplicationLinked, ctx.Logger)
}

func ListWorkloadTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"
	envName := c.Query("env")

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Env.View {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	// anyone with a token should be able to use this API
	ctx.Resp, ctx.RespErr = commonservice.ListWorkloadTemplate(projectName, envName, production, ctx.Logger)
}

func GetServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization check
	// TODO: Authorization leak for pm type in order to use collaboration mode in pm project
	if c.Param("type") != "pm" {
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
				ctx.UnAuthorized = true
				return
			}

			if production {
				if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
					!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.View {
					ctx.UnAuthorized = true
					return
				}
			} else {
				if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
					!ctx.Resources.ProjectAuthInfo[projectName].Service.View &&
					// special case for vm project
					!ctx.Resources.ProjectAuthInfo[projectName].Env.View {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.RespErr = commonservice.GetServiceTemplateWithStructure(c.Param("name"), c.Param("type"), projectName, setting.ProductStatusDeleting, revision, production, ctx.Logger)
}

func GetServiceTemplateOption(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authorization leak
	// this API is sometimes used in edit/create workflow scenario, thus giving the edit/create workflow permission
	// authorization check
	permitted := false
	projectName := c.Query("projectName")
	production := c.Query("production") == "true"
	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; ok {
		if production {
			// first check if the user is projectAdmin
			if projectAuthInfo.IsProjectAdmin {
				permitted = true
			} else if projectAuthInfo.Env.EditConfig ||
				projectAuthInfo.ProductionService.View {
				// then check if user has edit workflow permission
				permitted = true
			} else {
				// finally check if the permission is given by collaboration mode
				collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectName, types.ResourceTypeEnvironment, types.ProductionEnvActionEditConfig)
				if err == nil && collaborationAuthorizedEdit {
					permitted = true
				}
			}
		} else {
			// first check if the user is projectAdmin
			if projectAuthInfo.IsProjectAdmin {
				permitted = true
			} else if projectAuthInfo.Env.EditConfig ||
				projectAuthInfo.Service.View {
				// then check if user has edit workflow permission
				permitted = true
			} else {
				// finally check if the permission is given by collaboration mode
				collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectName, types.ResourceTypeEnvironment, types.EnvActionEditConfig)
				if err == nil && collaborationAuthorizedEdit {
					permitted = true
				}
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.RespErr = svcservice.GetServiceTemplateOption(c.Param("name"), projectName, revision, production, ctx.Logger)
}

type createServiceTemplateRequest struct {
	ProductName        string                           `json:"product_name" binding:"required"`
	ServiceName        string                           `json:"service_name" binding:"required"`
	Source             string                           `json:"source" binding:"required"`
	Type               string                           `json:"type" binding:"required"`
	Visibility         string                           `json:"visibility" binding:"required"`
	Yaml               string                           `json:"yaml" binding:"required"`
	VariableYaml       string                           `json:"variable_yaml"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs"`
}

// @Summary Create service template
// @Description Create service template
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	force	query		bool							true	"is force to create service template"
// @Param 	body 	body 		createServiceTemplateRequest 	true 	"body"
// @Success 200 	{object} 	svcservice.ServiceOption
// @Router /api/aslan/service/services [post]
func CreateServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(createServiceTemplateRequest)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateServiceTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateServiceTemplate json.Unmarshal err : %v", err)
	}

	production := c.Query("production") == "true"
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}

	// insert operation logs
	detail := fmt.Sprintf("服务名称:%s", args.ServiceName)
	detailEn := fmt.Sprintf("Service Name: %s", args.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", function, detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	projectName := args.ProductName

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		// TODO: Authorization leak
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Create &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Create &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ServiceTmpl json args")
		return
	}

	force, err := strconv.ParseBool(c.Query("force"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("force params error")
		return
	}

	svc := new(commonmodels.Service)
	svc.CreateBy = ctx.UserName
	svc.ProductName = args.ProductName
	svc.ServiceName = args.ServiceName
	svc.Source = args.Source
	svc.Type = args.Type
	svc.VariableYaml = args.VariableYaml
	svc.ServiceVariableKVs = args.ServiceVariableKVs
	svc.Yaml = args.Yaml

	ctx.Resp, ctx.RespErr = svcservice.CreateServiceTemplate(ctx.UserName, svc, force, production, ctx.Logger)
}

// used in cron, update service env status
func UpdateServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	if args.Username != "system" {
		detail := fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision)
		detailEn := fmt.Sprintf("Service Name: %s, Version: %d", args.ServiceName, args.Revision)
		internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)
	}

	// authorization checks
	projectName := args.ProductName
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Create &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args.Username = ctx.UserName
	ctx.RespErr = svcservice.UpdateServiceEnvStatus(args)
}

type updateServiceVariableRequest struct {
	VariableYaml       string                           `json:"variable_yaml" binding:"required"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs" binding:"required"`
}

// @Summary Update service varaible
// @Description Update service varaible
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	name		path		string							true	"service name"
// @Param 	projectName	query		string							true	"project name"
// @Param 	production	query		bool							true	"is production"
// @Param 	body  		body 		updateServiceVariableRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/service/services/{name}/variable [put]
func UpdateServiceVariable(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(updateServiceVariableRequest)
	servceTmplObjectargs := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	production := c.Query("production") == "true"
	function := "项目管理-服务变量"
	if production {
		function = "项目管理-生产服务变量"
	}

	// authorization
	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Create &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Create &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	servceTmplObjectargs.ProductName = c.Query("projectName")
	servceTmplObjectargs.ServiceName = c.Param("name")
	servceTmplObjectargs.Username = ctx.UserName
	servceTmplObjectargs.VariableYaml = req.VariableYaml
	servceTmplObjectargs.ServiceVariableKVs = req.ServiceVariableKVs

	detail := fmt.Sprintf("服务名称:%s", servceTmplObjectargs.ServiceName)
	detailEn := fmt.Sprintf("Service Name: %s", servceTmplObjectargs.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, servceTmplObjectargs.ProductName, "更新", function, detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.UpdateServiceVariables(servceTmplObjectargs, production)
}

func UpdateServiceHealthCheckStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	// authorization
	// TODO: authorization leak, currently disabled to use collaboration mode for PM environment
	//projectName := args.ProductName
	//if !ctx.Resources.IsSystemAdmin {
	//	if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
	//		ctx.UnAuthorized = true
	//		return
	//	}
	//	if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
	//		!ctx.Resources.ProjectAuthInfo[projectName].Env.EditConfig {
	//		ctx.UnAuthorized = true
	//		return
	//	}
	//}

	if args.Username != "system" {
		detail := fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision)
		detailEn := fmt.Sprintf("Service Name: %s, Version: %d", args.ServiceName, args.Revision)
		internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)
	}
	args.Username = ctx.UserName
	ctx.RespErr = svcservice.UpdateServiceHealthCheckStatus(args)
}

type ValidatorResp struct {
	Message string `json:"message"`
}

func YamlValidator(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(svcservice.YamlValidatorReq)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}
	resp := make([]*ValidatorResp, 0)
	errMsgList := svcservice.YamlValidator(args)
	for _, errMsg := range errMsgList {
		resp = append(resp, &ValidatorResp{Message: errMsg})
	}
	ctx.Resp = resp
}

func HelmReleaseNaming(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Create &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Edit {
				ctx.UnAuthorized = true
				return
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Create &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.ReleaseNamingRule)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}

	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "修改", function, args.ServiceName, args.ServiceName, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.UpdateReleaseNamingRule(ctx.UserName, ctx.RequestID, projectName, args, production, ctx.Logger)
}

func DeleteServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Delete {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Delete {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "删除", function, c.Param("name"), c.Param("name"), "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.DeleteServiceTemplate(c.Param("name"), c.Param("type"), projectName, production, ctx.Logger)
}

func ListServicePort(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.RespErr = svcservice.ListServicePort(c.Param("name"), c.Param("type"), c.Query("projectName"), setting.ProductStatusDeleting, revision, ctx.Logger)
}

func UpdateWorkloads(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(svcservice.UpdateWorkloadsArgs)

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateWorkloads c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkloads json.Unmarshal err : %v", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "配置", "环境", c.Query("env"), c.Query("env"), string(data), types.RequestBodyTypeJSON, ctx.Logger, c.Query("env"))

	err = c.ShouldBindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid UpdateWorkloadsArgs")
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"
	envName := c.Query("env")
	if projectName == "" || envName == "" {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Env.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		if !allowedSet.Has(envName) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	ctx.RespErr = svcservice.UpdateWorkloads(c, ctx.RequestID, ctx.UserName, projectName, envName, *args, production, ctx.Logger)
}

func CreateK8sWorkloads(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(svcservice.K8sWorkloadsArgs)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateK8sWorkloads c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateK8sWorkloads json.Unmarshal err : %v", err)
	}

	projectName := c.Query("projectName")
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "新增", "环境", args.EnvName, c.Query("env"), string(data), types.RequestBodyTypeJSON, ctx.Logger, args.EnvName)

	production := c.Query("production") == "true"
	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionEnv.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Env.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if production {
		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid K8sWorkloadsArgs args")
		return
	}

	ctx.RespErr = svcservice.CreateK8sWorkLoads(c, ctx.RequestID, ctx.UserName, args, production, ctx.Logger)
}

func GetServiceTemplateProductName(c *gin.Context) {
	args := new(commonmodels.Service)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}
	c.Set("productName", args.ProductName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Next()
}

func GetLabelSourceServiceInfo(c *gin.Context) {
	// get label binding id to be worked with
	labelBindingID := c.Param("id")

	bindingInfo, err := svcservice.GetLabelBindingInfo(labelBindingID)
	if err != nil {
		log.Errorf("failed to get label binding info, error: %s", err)
		return
	}

	c.Set("serviceName", bindingInfo.ServiceName)
	c.Set("projectKey", bindingInfo.ProjectKey)
	c.Set("production", bindingInfo.Production)

	c.Next()
}

func CreatePMService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Param("productName")

	args := new(svcservice.ServiceTmplBuildObject)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreatePMService c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreatePMService json.Unmarshal err : %v", err)
	}

	detail := fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision)
	detailEn := fmt.Sprintf("Service Name: %s, Version: %d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "新增", "项目管理-主机服务", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid service json args")
		return
	}
	if args.Build.Name == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("构建名称不能为空!")
		return
	}

	for _, heathCheck := range args.ServiceTmplObject.HealthChecks {
		if heathCheck.TimeOut < 2 || heathCheck.TimeOut > 60 {
			ctx.RespErr = e.ErrInvalidParam.AddDesc("超时时间必须在2-60之间")
			return
		}
		if heathCheck.Interval != 0 {
			if heathCheck.Interval < 2 || heathCheck.Interval > 60 {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("间隔时间必须在2-60之间")
				return
			}
		}
		if heathCheck.HealthyThreshold != 0 {
			if heathCheck.HealthyThreshold < 2 || heathCheck.HealthyThreshold > 10 {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("健康阈值必须在2-10之间")
				return
			}
		}
		if heathCheck.UnhealthyThreshold != 0 {
			if heathCheck.UnhealthyThreshold < 2 || heathCheck.UnhealthyThreshold > 10 {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("不健康阈值必须在2-10之间")
				return
			}
		}
	}

	ctx.RespErr = svcservice.CreatePMService(ctx.UserName, args, ctx.Logger)
}

func UpdatePmServiceTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Param("productName")

	args := new(commonservice.ServiceTmplBuildObject)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdatePmServiceTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdatePmServiceTemplate json.Unmarshal err : %v", err)
	}

	detail := fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision)
	detailEn := fmt.Sprintf("Service Name: %s, Version: %d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "项目管理-主机服务", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	for _, heathCheck := range args.ServiceTmplObject.HealthChecks {
		if heathCheck.TimeOut < 2 || heathCheck.TimeOut > 60 {
			ctx.RespErr = e.ErrInvalidParam.AddDesc("超时时间必须在2-60之间")
			return
		}
		if heathCheck.Interval != 0 {
			if heathCheck.Interval < 2 || heathCheck.Interval > 60 {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("间隔时间必须在2-60之间")
				return
			}
		}
		if heathCheck.HealthyThreshold != 0 {
			if heathCheck.HealthyThreshold < 2 || heathCheck.HealthyThreshold > 10 {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("健康阈值必须在2-10之间")
				return
			}
		}
		if heathCheck.UnhealthyThreshold != 0 {
			if heathCheck.UnhealthyThreshold < 2 || heathCheck.UnhealthyThreshold > 10 {
				ctx.RespErr = e.ErrInvalidParam.AddDesc("不健康阈值必须在2-10之间")
				return
			}
		}
	}
	ctx.RespErr = commonservice.UpdatePmServiceTemplate(ctx.UserName, args, ctx.Logger)
}

// @Summary convert varaible kv and yaml
// @Description convert varaible kv and yaml
// @Tags service
// @Accept json
// @Produce json
// @Param body body commonservice.ConvertVaraibleKVAndYamlArgs true "body"
// @Success 200 {object} commonservice.ConvertVaraibleKVAndYamlArgs
// @Router /api/aslan/service/services/variable/convert [post]
func ConvertVaraibleKVAndYaml(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ConvertVaraibleKVAndYamlArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ConvertVariableKVAndYamlArgs")
		return
	}

	resp, err := commonservice.ConvertVaraibleKVAndYaml(args)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = resp
}

type addServiceLabelReq struct {
	LabelID     string `json:"label_id,omitempty"`
	ProjectKey  string `json:"project_key,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Production  string `json:"production,omitempty"`
	Value       string `json:"value,omitempty"`
}

// @Summary 服务添加标签
// @Description 全部参数都是必传
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	body 			body 		addServiceLabelReq 	  true 	"body"
// @Success 200
// @Router /api/aslan/service/labels [post]
func AddServiceLabel(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(addServiceLabelReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	var production *bool
	switch req.Production {
	case "true":
		production = boolptr.True()
	case "false":
		production = boolptr.False()
	default:
		production = nil
	}
	function := "项目管理-服务标签"
	if boolptr.IsTrue(production) {
		function = "项目管理-生产服务标签"
	}

	// authorization
	projectName := req.ProjectKey
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if boolptr.IsTrue(production) {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Edit &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	requestByte, err := json.Marshal(req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("failed to decode request body, error: " + err.Error())
		return
	}

	detail := fmt.Sprintf("服务名称:%s", req.ServiceName)
	detailEn := fmt.Sprintf("Service Name: %s", req.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "新建", function, detail, detailEn, string(requestByte), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.AddServiceLabel(req.LabelID, projectName, req.ServiceName, production, req.Value, ctx.Logger)
}

type updateServiceLabelReq struct {
	Value string `json:"value,omitempty"`
}

func UpdateServiceLabel(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(updateServiceLabelReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	productionInterface, ok := c.Get("production")
	if !ok {
		ctx.RespErr = fmt.Errorf("production not set by previous function")
		return
	}

	production, ok := productionInterface.(bool)
	if !ok {
		ctx.RespErr = fmt.Errorf("production is not boolean")
		return
	}

	function := "项目管理-服务标签"
	if production {
		function = "项目管理-生产服务标签"
	}

	// authorization
	projectNameInterface, ok := c.Get("projectKey")
	if !ok {
		ctx.RespErr = fmt.Errorf("project key not set by previous function")
		return
	}

	projectName, ok := projectNameInterface.(string)
	if !ok {
		ctx.RespErr = fmt.Errorf("project key is not string")
		return
	}

	serviceNameInterface, ok := c.Get("serviceName")
	if !ok {
		ctx.RespErr = fmt.Errorf("service name not set by previous function")
		return
	}

	serviceName, ok := serviceNameInterface.(string)
	if !ok {
		ctx.RespErr = fmt.Errorf("service name is not string")
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Edit &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	labelBindingID := c.Param("id")

	requestByte, err := json.Marshal(req)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("failed to decode request body, error: " + err.Error())
		return
	}

	detail := fmt.Sprintf("服务名称:%s", serviceName)
	detailEn := fmt.Sprintf("Service Name: %s", serviceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", function, detail, detailEn, string(requestByte), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.UpdateServiceLabel(labelBindingID, req.Value, ctx.Logger)
}

func DeleteServiceLabel(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	productionInterface, ok := c.Get("production")
	if !ok {
		ctx.RespErr = fmt.Errorf("production not set by previous function")
		return
	}

	production, ok := productionInterface.(bool)
	if !ok {
		ctx.RespErr = fmt.Errorf("production is not boolean")
		return
	}

	function := "项目管理-服务标签"
	if production {
		function = "项目管理-生产服务标签"
	}

	// authorization
	projectNameInterface, ok := c.Get("projectKey")
	if !ok {
		ctx.RespErr = fmt.Errorf("project key not set by previous function")
		return
	}

	projectName, ok := projectNameInterface.(string)
	if !ok {
		ctx.RespErr = fmt.Errorf("project key is not string")
		return
	}

	serviceNameInterface, ok := c.Get("serviceName")
	if !ok {
		ctx.RespErr = fmt.Errorf("service name not set by previous function")
		return
	}

	serviceName, ok := serviceNameInterface.(string)
	if !ok {
		ctx.RespErr = fmt.Errorf("service name is not string")
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if production {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Edit &&
				!ctx.Resources.ProjectAuthInfo[projectName].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Edit &&
				!ctx.Resources.ProjectAuthInfo[projectName].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	labelBindingID := c.Param("id")

	detail := fmt.Sprintf("服务名称:%s", serviceName)
	detailEn := fmt.Sprintf("Serivce Name: %s", serviceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "删除", function, detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.DeleteServiceLabel(labelBindingID, ctx.Logger)
}

type listServiceLabelReq struct {
	ProjectKey  string `json:"project_key,omitempty" form:"project_key,omitempty"`
	ServiceName string `json:"service_name,omitempty" form:"service_name,omitempty"`
	Production  string `json:"production,omitempty" form:"production,omitempty"`
}

// @Summary 获取服务的标签列表
// @Description
// @Tags 	service
// @Accept 	json
// @Produce json
// @Param 	projectKey		query		string							true	"项目标识"
// @Param 	serviceName		query		string							true	"服务名称"
// @Param 	production		query		string							true	"是否为生产服务，true:生产服务，false:非生产服务"
// @Success 200 			{object} 	svcservice.ServiceLabelResp
// @Router /api/aslan/service/labels [get]
func ListServiceLabels(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(listServiceLabelReq)
	if err := c.ShouldBindQuery(req); err != nil {
		ctx.RespErr = err
		return
	}

	var production *bool
	switch req.Production {
	case "true":
		production = boolptr.True()
	case "false":
		production = boolptr.False()
	default:
		production = nil
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.BusinessDirectory.View {
			if _, ok := ctx.Resources.ProjectAuthInfo[req.ProjectKey]; !ok {
				ctx.UnAuthorized = true
				return
			}

			if boolptr.IsTrue(production) {
				if !ctx.Resources.ProjectAuthInfo[req.ProjectKey].IsProjectAdmin &&
					!ctx.Resources.ProjectAuthInfo[req.ProjectKey].ProductionService.View {
					ctx.UnAuthorized = true
					return
				}
			} else {
				if !ctx.Resources.ProjectAuthInfo[req.ProjectKey].IsProjectAdmin &&
					!ctx.Resources.ProjectAuthInfo[req.ProjectKey].Service.View {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	ctx.Resp, ctx.RespErr = svcservice.ListServiceLabels(req.ProjectKey, req.ServiceName, production, ctx.Logger)
}
