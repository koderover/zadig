package handler

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util/boolptr"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	svcservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @summary 从代码库创建 K8s YAML 服务
// @description 从代码库创建 K8s YAML 服务
// @tags 	OpenAPI
// @accept 	json
// @produce json
// @Param 	codehostId 		path 		int 							  true 		"代码源 ID"
// @Param   repoName 		query 		string 							  false 	"代码库名称，repoName 和 repoUUID 至少传一个"
// @Param   repoUUID 		query 		string 							  false 	"代码库唯一标识"
// @Param   branchName 		query 		string 							  true 		"服务配置所在分支"
// @Param   remoteName 		query 		string 							  false 	"远端名称，Gerrit 场景使用，普通 Git 仓库可为空"
// @Param   repoOwner 		query 		string 							  true 		"仓库拥有者/组织名"
// @Param   namespace 		query 		string 							  false 	"仓库命名空间，为空默认使用 repoOwner"
// @Param   production 		query 		bool 							  false 	"是否创建生产服务，true 表示生产服务，false 表示测试服务"
// @Param 	body 			body 		svcservice.OpenAPILoadServiceFromCodeHostReq true 	"K8s YAML 服务创建参数"
// @success 200 {object} svcservice.OpenAPILoadHelmServiceResp
// @router /openapi/service/loader/load/:codehostId [post]
func LoadServiceTemplateFromCodeHostOpenAPI(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	codehostID, err := strconv.Atoi(c.Param("codehostId"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	req := new(svcservice.OpenAPILoadServiceFromCodeHostReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}
	if err = json.Unmarshal(data, req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid LoadServiceReq json args")
		return
	}

	req.CodehostID = codehostID
	req.RepoName = c.Query("repoName")
	req.RepoUUID = c.Query("repoUUID")
	req.BranchName = c.Query("branchName")
	req.RemoteName = c.Query("remoteName")
	req.RepoOwner = c.Query("repoOwner")
	req.Namespace = c.Query("namespace")
	req.Production = c.Query("production") == "true"
	if err := req.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	function := "项目管理-测试服务"
	if req.Production {
		function = "项目管理-生产服务"
	}

	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", req.ProductName, "新增", function, "", "", string(data), types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[req.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if req.Production {
			if !ctx.Resources.ProjectAuthInfo[req.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[req.ProductName].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[req.ProductName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[req.ProductName].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if req.Production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = svcservice.OpenAPILoadServiceFromCodeHost(ctx.UserName, req, ctx.Logger)
}

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
	detail := fmt.Sprintf("服务名称:%s", req.ServiceName)
	detailEn := fmt.Sprintf("Service Name: %s", req.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", req.ProjectKey, "新增", "项目管理-测试服务", detail, detailEn, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

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
	detail := fmt.Sprintf("服务名称:%s", req.ServiceName)
	detailEn := fmt.Sprintf("Service Name:%s", req.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", req.ProjectKey, "新增", "项目管理-生产服务", detail, detailEn, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

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

	detail := fmt.Sprintf("服务名称:%s", req.ServiceName)
	detailEn := fmt.Sprintf("Service Name: %s", req.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openapi)", projectKey, "新增", "项目管理-测试服务", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

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

	detail := fmt.Sprintf("服务名称:%s", req.ServiceName)
	detailEn := fmt.Sprintf("Service Name: %s", req.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openapi)", projectKey, "新增", "项目管理-生产服务", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

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
	detail := fmt.Sprintf("服务名称:%s", args.ServiceName)
	detailEn := fmt.Sprintf("Service: %s", args.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "(OpenAPI)"+"更新测试服务配置", "项目管理-服务", detail, detailEn, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

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
	detail := fmt.Sprintf("服务名称:%s", args.ServiceName)
	detailEn := fmt.Sprintf("Service Name:%s", args.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "(OpenAPI)"+"更新生产服务配置", "项目管理-服务", detail, detailEn, string(bs), types.RequestBodyTypeJSON, ctx.Logger)

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

	detail := fmt.Sprintf("服务名称:%s", serviceName)
	detailEn := fmt.Sprintf("Service Name:%s", serviceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"更新测试服务变量", "项目管理-服务", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)

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

	detail := fmt.Sprintf("服务名称:%s", serviceName)
	detailEn := fmt.Sprintf("Service Name: %s", serviceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"更新生产服务变量", "项目管理-服务", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)

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

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "项目管理-测试服务", serviceName, serviceName, "", types.RequestBodyTypeJSON, ctx.Logger)

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
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "项目管理-生产服务", serviceName, serviceName, "", types.RequestBodyTypeJSON, ctx.Logger)

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

	ctx.RespErr = svcservice.DeleteServiceTemplate(serviceName, "k8s", projectKey, true, ctx.Logger)
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

// @summary 新建 Helm 服务
// @description 支持从代码库、公有代码库和 Chart 仓库新建 Helm 服务
// @tags 	OpenAPI
// @accept 	json
// @produce json
// @Param   projectKey		query		 string								 true	"项目标识"
// @Param   body 			body 		 svcservice.OpenAPILoadHelmServiceReq true 	"Helm 服务创建参数"
// @success 200
// @router /openapi/service/helm/load [post]
func LoadHelmServiceOpenAPI(c *gin.Context) {
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

	args := new(svcservice.OpenAPILoadHelmServiceReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("failed to unmarshal request body")
		return
	}

	function := "项目管理-测试服务"
	if args.Production {
		function = "项目管理-生产服务"
	}

	detail := fmt.Sprintf("服务名称:%s", getHelmServiceNameFromArgs(args))
	detailEn := fmt.Sprintf("Service Name: %s", getHelmServiceNameFromArgs(args))
	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", projectKey, "新增", function, detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if args.Production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if args.Production {
		if err = commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.Resp, ctx.RespErr = svcservice.OpenAPILoadHelmService(ctx, projectKey, args)
}

func getHelmServiceNameFromArgs(args *svcservice.OpenAPILoadHelmServiceReq) string {
	if args.Name != "" {
		return args.Name
	}

	switch createFrom := args.CreateFrom.(type) {
	case *svcservice.OpenAPICreateFromChartRepo:
		return createFrom.ChartName
	case map[string]interface{}:
		if chartName, ok := createFrom["chartName"].(string); ok {
			return chartName
		}
	}

	return ""
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

// @summary 使用模版新建 Helm 服务
// @description 使用模版新建 Helm 服务
// @tags 	OpenAPI
// @accept 	json
// @produce json
// @Param   projectKey		query		 string								                     true	"项目标识"
// @Param   body 			body 		 svcservice.OpenAPILoadHelmServiceFromTemplateReq        true 	"body"
// @success 200
// @router /openapi/service/template/load/helm [post]
func LoadHelmServiceFromTemplateOpenAPI(c *gin.Context) {
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

	req := new(svcservice.OpenAPILoadHelmServiceFromTemplateReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid request body")
		return
	}
	if err = json.Unmarshal(data, req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("failed to unmarshal request body")
		return
	}

	function := "项目管理-测试服务"
	if req.Production {
		function = "项目管理-生产服务"
	}

	detail := fmt.Sprintf("服务名称:%s", req.ServiceName)
	detailEn := fmt.Sprintf("Service Name: %s", req.ServiceName)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"新增", function, detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if req.Production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Create {
				ctx.UnAuthorized = true
				return
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.RespErr = svcservice.OpenAPILoadHelmServiceFromTemplate(ctx, projectKey, req)
}
