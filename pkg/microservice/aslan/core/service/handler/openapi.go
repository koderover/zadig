package handler

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util/boolptr"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	svcservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func LoadServiceFromYamlTemplateOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(svcservice.OpenAPILoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}
	req.Production = false

	if err := req.Validate(); err != nil {
		ctx.RespErr = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", req.ProjectKey, "新增", "项目管理-测试服务", fmt.Sprintf("服务名称:%s", req.ServiceName), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[req.ProjectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[req.ProjectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[req.ProjectKey].Service.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = svcservice.OpenAPILoadServiceFromYamlTemplate(ctx.UserName, req, false, ctx.Logger)
}

func LoadProductionServiceFromYamlTemplateOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(svcservice.OpenAPILoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}
	req.Production = true

	if err := req.Validate(); err != nil {
		ctx.RespErr = err
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", req.ProjectKey, "新增", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", req.ServiceName), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	err := commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = svcservice.OpenAPILoadServiceFromYamlTemplate(ctx.UserName, req, false, ctx.Logger)
}

func CreateRawYamlServicesOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	req := new(svcservice.OpenAPICreateYamlServiceReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}
	if err = json.Unmarshal(data, req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("failed to unmarshal request body")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName+"(openapi)", projectKey, "新增", "项目管理-测试服务", fmt.Sprintf("服务名称:%s", req.ServiceName), string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = svcservice.CreateRawYamlServicesOpenAPI(ctx.UserName, projectKey, req, ctx.Logger)
}

func CreateRawProductionYamlServicesOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = fmt.Errorf("projectKey cannot be empty")
		return
	}

	req := new(svcservice.OpenAPICreateYamlServiceReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}
	if err = json.Unmarshal(data, req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("failed to unmarshal request body")
		return
	}
	req.Production = true

	internalhandler.InsertOperationLog(c, ctx.UserName+"(openapi)", projectKey, "新增", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", req.ServiceName), string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = svcservice.CreateRawYamlServicesOpenAPI(ctx.UserName, projectKey, req, ctx.Logger)
}

func UpdateServiceConfigOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(svcservice.OpenAPIUpdateServiceConfigArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid update service config json args")
		return
	}
	args.ProjectName = c.Query("projectKey")
	args.ServiceName = c.Param("name")
	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "(OpenAPI)"+"更新测试服务配置", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.ServiceName), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		// TODO: Authorization leak
		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = svcservice.OpenAPIUpdateServiceConfig(ctx.UserName, args, ctx.Logger)
}

func UpdateProductionServiceConfigOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(svcservice.OpenAPIUpdateServiceConfigArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid update service config json args")
		return
	}
	args.ProjectName = c.Query("projectKey")
	args.ServiceName = c.Param("name")
	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "(OpenAPI)"+"更新生产服务配置", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.ServiceName), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = svcservice.OpenAPIProductionUpdateServiceConfig(ctx.UserName, args, ctx.Logger)
}

func UpdateServiceVariableOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(svcservice.OpenAPIUpdateServiceVariableRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}
	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"更新测试服务变量", "项目管理-服务", fmt.Sprintf("服务名称:%s", serviceName), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = svcservice.OpenAPIUpdateServiceVariable(ctx.UserName, projectKey, serviceName, req, ctx.Logger)
}

func UpdateProductionServiceVariableOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(svcservice.OpenAPIUpdateServiceVariableRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}
	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"更新生产服务变量", "项目管理-服务", fmt.Sprintf("服务名称:%s", serviceName), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = svcservice.OpenAPIUpdateProductionServiceVariable(ctx.UserName, projectKey, serviceName, req, ctx.Logger)
}

func DeleteYamlServicesOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}
	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "OpenAPI"+"删除", "项目管理-测试服务", serviceName, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = svcservice.DeleteServiceTemplate(serviceName, "k8s", projectKey, false, ctx.Logger)
}

func DeleteProductionServicesOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}
	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "OpenAPI"+"删除", "项目管理-生产服务", serviceName, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = svcservice.DeleteServiceTemplate(serviceName, "k8s", projectKey, false, ctx.Logger)
}

func GetYamlServiceOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return
	}

	ctx.Resp, ctx.RespErr = svcservice.OpenAPIGetYamlService(projectKey, serviceName, ctx.Logger)
}

func GetYamlServiceLabelOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	productionStr := c.Query("production")

	var production *bool
	switch productionStr {
	case "true":
		production = boolptr.True()
	case "false":
		production = boolptr.False()
	default:
		production = nil
	}

	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return
	}

	ctx.Resp, ctx.RespErr = svcservice.OpenAPIListServiceLabels(projectKey, serviceName, production, ctx.Logger)
}

func GetProductionYamlServiceOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return
	}

	err := commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = svcservice.GetProductionYamlServiceOpenAPI(projectKey, serviceName, ctx.Logger)
}

func ListYamlServicesOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	resp, err := svcservice.ListServiceTemplateOpenAPI(projectKey, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp = resp
}

func ListProductionYamlServicesOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return
	}

	err := commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	resp, err := svcservice.ListProductionServiceTemplateOpenAPI(projectKey, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp = resp
}
