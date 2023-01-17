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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type DeleteProductServicesRequest struct {
	ServiceNames []string `json:"service_names"`
}

type ChartInfoArgs struct {
	ChartInfos []*template.ServiceRender `json:"chart_infos"`
}

type NamespaceResource struct {
	Services  []*commonservice.ServiceResp `json:"services"`
	Ingresses []resource.Ingress           `json:"ingresses"`
}

type UpdateProductRegistryRequest struct {
	RegistryID string `json:"registry_id"`
}

func ListProducts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam
		return
	}

	envNames, found := internalhandler.GetResourcesInHeader(c)
	if found && len(envNames) == 0 {
		ctx.Resp = []*service.ProductResp{}
		return
	}

	ctx.Resp, ctx.Err = service.ListProducts(projectName, envNames, ctx.Logger)
}

func UpdateMultiProducts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := &service.UpdateEnvRequest{}
	err := c.ShouldBindQuery(request)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if request.ProjectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}
	updateMultiEnvWrapper(c, request, ctx)
}

func createProduct(c *gin.Context, param *service.CreateEnvRequest, createArgs []*service.CreateSingleProductArg, requestBody string, ctx *internalhandler.Context) {
	envNameList := make([]string, 0)
	for _, arg := range createArgs {
		if arg.EnvName == "" {
			ctx.Err = e.ErrInvalidParam.AddDesc("envName is empty")
			return
		}
		arg.ProductName = param.ProjectName
		envNameList = append(envNameList, arg.EnvName)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, param.ProjectName, setting.OperationSceneEnv, "新增", "环境", strings.Join(envNameList, "-"), requestBody, ctx.Logger, envNameList...)
	switch param.Type {
	case setting.K8SDeployType:
		ctx.Err = service.CreateYamlProduct(param.ProjectName, ctx.UserName, ctx.RequestID, createArgs, ctx.Logger)
	case setting.SourceFromExternal:
		ctx.Err = service.CreateHostProductionProduct(param.ProjectName, ctx.UserName, ctx.RequestID, createArgs, ctx.Logger)
	default:
		ctx.Err = service.CreateHelmProduct(param.ProjectName, ctx.UserName, ctx.RequestID, createArgs, ctx.Logger)
	}
}

func copyProduct(c *gin.Context, param *service.CreateEnvRequest, createArgs []*service.CreateSingleProductArg, requestBody string, ctx *internalhandler.Context) {
	envNameCopyList := make([]string, 0)
	envNames := make([]string, 0)
	for _, arg := range createArgs {
		if arg.EnvName == "" || arg.BaseEnvName == "" {
			ctx.Err = e.ErrInvalidParam.AddDesc("envName or baseEnvName is empty")
			return
		}
		arg.ProductName = param.ProjectName
		envNameCopyList = append(envNameCopyList, arg.BaseEnvName+"-->"+arg.EnvName)
		envNames = append(envNames, arg.EnvName)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, param.ProjectName, setting.OperationSceneEnv, "复制", "环境", strings.Join(envNameCopyList, ","), requestBody, ctx.Logger, envNames...)
	if param.Type == setting.K8SDeployType {
		ctx.Err = service.CopyYamlProduct(ctx.UserName, ctx.RequestID, param.ProjectName, createArgs, ctx.Logger)
	} else {
		ctx.Err = service.CopyHelmProduct(param.ProjectName, ctx.UserName, ctx.RequestID, createArgs, ctx.Logger)
	}
}

// CreateProduct creates new product
// Query param `type` determines the type of product
// Query param `scene` determines if the product is copied from some project
func CreateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	createParam := &service.CreateEnvRequest{}
	err := c.ShouldBindQuery(createParam)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if createParam.ProjectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		log.Infof("CreateProduct failed to get request data, err: %s", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if createParam.Type == setting.K8SDeployType || createParam.Type == setting.HelmDeployType || createParam.Type == setting.SourceFromExternal {
		createArgs := make([]*service.CreateSingleProductArg, 0)
		if err = json.Unmarshal(data, &createArgs); err != nil {
			log.Errorf("copyHelmProduct json.Unmarshal err : %s", err)
			ctx.Err = e.ErrInvalidParam.AddErr(err)
			return
		}

		allowedClusters, found := internalhandler.GetResourcesInHeader(c)
		if found {
			allowedSet := sets.NewString(allowedClusters...)
			for _, args := range createArgs {
				if !allowedSet.Has(args.ClusterID) {
					c.String(http.StatusForbidden, "permission denied for cluster %s", args.ClusterID)
					return
				}
			}
		}

		if createParam.Type == setting.HelmDeployType {
			for _, arg := range createArgs {
				arg.DefaultValues = arg.HelmDefaultValues
			}
		}

		if createParam.Scene == "copy" {
			copyProduct(c, createParam, createArgs, string(data), ctx)
		} else {
			createProduct(c, createParam, createArgs, string(data), ctx)
		}
		return
	} else {
		// 'auto = true' only happens in the onboarding progress of pm projects
		if createParam.Auto {
			ctx.Resp = service.AutoCreateProduct(createParam.ProjectName, createParam.EnvType, ctx.RequestID, ctx.Logger)
			return
		}

		args := new(commonmodels.Product)
		if err = json.Unmarshal(data, args); err != nil {
			log.Errorf("CreateProduct json.Unmarshal err : %v", err)
		}

		internalhandler.InsertDetailedOperationLog(c, ctx.UserName, args.ProductName, setting.OperationSceneEnv, "新增", "环境", args.EnvName, string(data), ctx.Logger, args.EnvName)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

		if err := c.BindJSON(args); err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
			return
		}

		if args.EnvName == "" {
			ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
			return
		}

		ctx.Err = service.CreateProduct(ctx.UserName, ctx.RequestID, args, ctx.Logger)
	}
}

type UpdateProductParams struct {
	ServiceNames []string `json:"service_names"`
	VariableYaml string   `json:"variable_yaml"`
	commonmodels.Product
}

// UpdateProduct update product variables, used for pm products
func UpdateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")

	args := new(UpdateProductParams)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("updateProductImpl c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("updateProductImpl json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "更新", "环境变量", envName, string(data), ctx.Logger, envName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	//force, _ := strconv.ParseBool(c.Query("force")
	//serviceNames := sets.String{}
	//for _, kv := range args.Vars {
	//	for _, s := range kv.Services {
	//		serviceNames.Insert(s)
	//	}
	//}

	ctx.Err = service.UpdateCVMProduct(envName, projectName, ctx.UserName, ctx.RequestID, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product %s %s: %v", envName, projectName, ctx.Err)
	}
}

func UpdateProductRegistry(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Param("name")
	projectName := c.Query("projectName")
	args := new(UpdateProductRegistryRequest)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("updateProductImpl c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("updateProductImpl json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "更新", "环境", envName, string(data), ctx.Logger, envName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = service.UpdateProductRegistry(envName, projectName, args.RegistryID, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product %s %s: %v", envName, args.RegistryID, ctx.Err)
	}
}

func UpdateProductRecycleDay(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	recycleDayStr := c.Query("recycleDay")

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "更新", "环境-环境回收", envName, "", ctx.Logger, envName)

	var (
		recycleDay int
		err        error
	)
	if recycleDayStr == "" || envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName or recycleDay不能为空")
		return
	}
	recycleDay, err = strconv.Atoi(recycleDayStr)
	if err != nil || recycleDay < 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("recycleDay必须是正整数")
		return
	}

	ctx.Err = service.UpdateProductRecycleDay(envName, projectName, recycleDay)
}

func UpdateProductAlias(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	arg := new(commonmodels.Product)
	if err := c.BindJSON(arg); err != nil {
		return
	}

	ctx.Err = service.UpdateProductAlias(c.Param("name"), c.Query("projectName"), arg.Alias)
}

func AffectedServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

	arg := new(service.K8sRendersetArg)
	if err := c.BindJSON(arg); err != nil {
		return
	}

	ctx.Resp, ctx.Err = service.GetAffectedServices(projectName, envName, arg, ctx.Logger)
}

func EstimatedValues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

	serviceName := c.Query("serviceName")
	if serviceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("serviceName can't be empty!")
		return
	}

	arg := new(service.EstimateValuesArg)
	if err := c.ShouldBind(arg); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.GeneEstimatedValues(projectName, envName, serviceName, c.Query("scene"), c.Query("format"), arg, ctx.Logger)
}

func SyncHelmProductRenderset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}
	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can't be empty!")
		return
	}

	ctx.Err = service.SyncHelmProductEnvironment(projectName, envName, ctx.RequestID, ctx.Logger)
}

func generalRequestValidate(c *gin.Context) (string, string, error) {
	projectName := c.Query("projectName")
	if projectName == "" {
		return "", "", errors.New("projectName can't be empty")
	}

	envName := c.Param("name")
	if envName == "" {
		return "", "", errors.New("envName can't be empty")
	}
	return projectName, envName, nil
}

func UpdateHelmProductRenderset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	arg := new(service.EnvRendersetArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateHelmProductRenderset c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, arg); err != nil {
		log.Errorf("UpdateHelmProductRenderset json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "更新", "更新环境变量", "", string(data), ctx.Logger, envName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(arg)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = service.UpdateHelmProductRenderset(projectName, envName, ctx.UserName, ctx.RequestID, arg, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product Variable %s %s: %v", envName, projectName, ctx.Err)
	}
}

func UpdateHelmProductDefaultValues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	arg := new(service.EnvRendersetArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProductDefaultValues c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, arg); err != nil {
		log.Errorf("UpdateProductDefaultValues json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "更新", "更新全局变量", envName, string(data), ctx.Logger, envName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(arg)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	arg.DeployType = setting.HelmDeployType
	ctx.Err = service.UpdateProductDefaultValues(projectName, envName, ctx.UserName, ctx.RequestID, arg, ctx.Logger)
}

func UpdateK8sProductDefaultValues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	arg := new(service.K8sRendersetArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateK8sProductDefaultValues c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, arg); err != nil {
		log.Errorf("UpdateK8sProductDefaultValues json.Unmarshal err : %v", err)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "更新", "更新全局变量", envName, string(data), ctx.Logger, envName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(arg)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	envRenderArg := &service.EnvRendersetArg{
		DeployType:    setting.K8SDeployType,
		DefaultValues: arg.VariableYaml,
	}

	ctx.Err = service.UpdateProductDefaultValues(projectName, envName, ctx.UserName, ctx.RequestID, envRenderArg, ctx.Logger)
}

func UpdateHelmProductCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName, envName, err := generalRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	arg := new(service.EnvRendersetArg)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateHelmProductCharts c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, arg); err != nil {
		log.Errorf("UpdateHelmProductCharts json.Unmarshal err : %v", err)
	}

	serviceName := make([]string, 0)
	for _, cd := range arg.ChartValues {
		serviceName = append(serviceName, cd.ServiceName)
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "更新", "更新服务", fmt.Sprintf("%s:[%s]", envName, strings.Join(serviceName, ",")), string(data), ctx.Logger, envName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(arg)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = service.UpdateHelmProductCharts(projectName, envName, ctx.UserName, ctx.RequestID, arg, ctx.Logger)
}

func updateMultiEnvWrapper(c *gin.Context, request *service.UpdateEnvRequest, ctx *internalhandler.Context) {
	deployType, err := service.GetProductDeployType(request.ProjectName)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	log.Infof("update multiple envs for project: %s, deploy type: %s", request.ProjectName, deployType)
	switch deployType {
	case setting.PMDeployType:
		updateMultiCvmEnv(c, request, ctx)
	case setting.HelmDeployType:
		updateMultiHelmEnv(c, request, ctx)
	case setting.K8SDeployType:
		updateMultiK8sEnv(c, request, ctx)
	}
}

func updateMultiK8sEnv(c *gin.Context, request *service.UpdateEnvRequest, ctx *internalhandler.Context) {
	args := make([]*service.UpdateEnv, 0)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateMultiProducts c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, &args); err != nil {
		log.Errorf("UpdateMultiProducts json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	var envNames []string
	for _, arg := range args {
		envNames = append(envNames, arg.EnvName)
	}
	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		currentSet := sets.NewString(envNames...)
		if !allowedSet.IsSuperset(currentSet) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, request.ProjectName, setting.OperationSceneEnv, "更新", "环境", strings.Join(envNames, ","), string(data), ctx.Logger, envNames...)
	ctx.Resp, ctx.Err = service.UpdateMultipleK8sEnv(args, envNames, request.ProjectName, ctx.RequestID, request.Force, ctx.Logger)
}

func updateMultiHelmEnv(c *gin.Context, request *service.UpdateEnvRequest, ctx *internalhandler.Context) {
	args := new(service.UpdateMultiHelmProductArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProduct json.Unmarshal err : %v", err)
	}
	args.ProductName = request.ProjectName

	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		currentSet := sets.NewString(args.EnvNames...)
		if !allowedSet.IsSuperset(currentSet) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, request.ProjectName, setting.OperationSceneEnv, "更新", "环境", strings.Join(args.EnvNames, ","), string(data), ctx.Logger, args.EnvNames...)

	ctx.Resp, ctx.Err = service.UpdateMultipleHelmEnv(
		ctx.RequestID, ctx.UserName, args, ctx.Logger,
	)
}

func updateMultiCvmEnv(c *gin.Context, request *service.UpdateEnvRequest, ctx *internalhandler.Context) {
	args := make([]*service.UpdateEnv, 0)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateMultiProducts c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, &args); err != nil {
		log.Errorf("UpdateMultiProducts json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	var envNames []string
	for _, arg := range args {
		envNames = append(envNames, arg.EnvName)
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, request.ProjectName, setting.OperationSceneEnv, "更新", "环境", strings.Join(envNames, ","), string(data), ctx.Logger, envNames...)
	ctx.Resp, ctx.Err = service.UpdateMultiCVMProducts(envNames, request.ProjectName, ctx.UserName, ctx.RequestID, ctx.Logger)
}

func GetProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	name := c.Param("name")

	ctx.Resp, ctx.Err = service.GetProduct(ctx.UserName, name, projectName, ctx.Logger)
}

func GetProductInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")

	ctx.Resp, ctx.Err = service.GetProductInfo(ctx.UserName, envName, projectName, ctx.Logger)
}

func GetHelmChartVersions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")

	ctx.Resp, ctx.Err = service.GetHelmChartVersions(projectName, envName, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to get helmVersions %s %s: %v", envName, projectName, ctx.Err)
	}
}

func GetEstimatedRenderCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

	envName := c.Param("name")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can't be empty!")
		return
	}

	ctx.Resp, ctx.Err = service.GetEstimatedRenderCharts(projectName, envName, c.Query("serviceName"), ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to get estimatedRenderCharts %s %s: %v", envName, projectName, ctx.Err)
	}
}

func DeleteProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	is_delete, err := strconv.ParseBool(c.Query("is_delete"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalidParam is_delete")
		return
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "删除", "环境", envName, "", ctx.Logger, envName)
	ctx.Err = service.DeleteProduct(ctx.UserName, envName, projectName, ctx.RequestID, is_delete, ctx.Logger)
}

func DeleteProductServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(DeleteProductServicesRequest)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("DeleteProductServices c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("DeleteProductServices json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	projectName := c.Query("projectName")
	envName := c.Param("name")

	// For environment sharing, if the environment is the base environment and the service to be deleted has been deployed in the subenvironment,
	// we should prompt the user that `Delete the service in the subenvironment before deleting the service in the base environment`.
	svcsInSubEnvs, err := service.CheckServicesDeployedInSubEnvs(c, projectName, envName, args.ServiceNames)
	if err != nil {
		ctx.Err = err
		return
	}
	if len(svcsInSubEnvs) > 0 {
		data := make(map[string]interface{}, len(svcsInSubEnvs))
		for k, v := range svcsInSubEnvs {
			data[k] = v
		}

		ctx.Err = e.NewWithExtras(e.ErrDeleteSvcHasSvcsInSubEnv, "", data)
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneEnv, "删除", "环境的服务", fmt.Sprintf("%s:[%s]", envName, strings.Join(args.ServiceNames, ",")), "", ctx.Logger, envName)
	ctx.Err = service.DeleteProductServices(ctx.UserName, ctx.RequestID, envName, projectName, args.ServiceNames, ctx.Logger)
}

func ListGroups(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	serviceName := c.Query("serviceName")
	perPageStr := c.Query("perPage")
	pageStr := c.Query("page")
	var (
		count   int
		perPage int
		err     error
		page    int
	)
	if perPageStr == "" {
		perPage = setting.PerPage
	} else {
		perPage, err = strconv.Atoi(perPageStr)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(fmt.Sprintf("pageStr args err :%s", err))
			return
		}
	}

	if pageStr == "" {
		page = 1
	} else {
		page, err = strconv.Atoi(pageStr)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(fmt.Sprintf("page args err :%s", err))
			return
		}
	}

	ctx.Resp, count, ctx.Err = service.ListGroups(serviceName, envName, projectName, perPage, page, ctx.Logger)
	c.Writer.Header().Set("X-Total", strconv.Itoa(count))
}

type ListWorkloadsArgs struct {
	Namespace    string `json:"namespace"    form:"namespace"`
	ClusterID    string `json:"clusterId"    form:"clusterId"`
	WorkloadName string `json:"workloadName" form:"workloadName"`
	PerPage      int    `json:"perPage"      form:"perPage,default:20"`
	Page         int    `json:"page"         form:"page,default:1"`
}

func ListWorkloads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(ListWorkloadsArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	count, services, err := commonservice.ListWorkloads("", args.ClusterID, args.Namespace, "", args.PerPage, args.Page, ctx.Logger, func(workloads []*commonservice.Workload) []*commonservice.Workload {
		workloadStat, _ := mongodb.NewWorkLoadsStatColl().Find(args.ClusterID, args.Namespace)
		workloadM := map[string]commonmodels.Workload{}
		for _, workload := range workloadStat.Workloads {
			workloadM[workload.Name] = workload
		}

		// add services in external env data
		servicesInExternalEnv, _ := mongodb.NewServicesInExternalEnvColl().List(&mongodb.ServicesInExternalEnvArgs{
			Namespace: args.Namespace,
			ClusterID: args.ClusterID,
		})

		for _, serviceInExternalEnv := range servicesInExternalEnv {
			workloadM[serviceInExternalEnv.ServiceName] = commonmodels.Workload{
				EnvName:     serviceInExternalEnv.EnvName,
				ProductName: serviceInExternalEnv.ProductName,
			}
		}

		for index, currentWorkload := range workloads {
			if existWorkload, ok := workloadM[currentWorkload.Name]; ok {
				workloads[index].EnvName = existWorkload.EnvName
				workloads[index].ProductName = existWorkload.ProductName
			}
		}

		var resp []*commonservice.Workload
		for _, workload := range workloads {
			if args.WorkloadName != "" && strings.Contains(workload.Name, args.WorkloadName) {
				resp = append(resp, workload)
			} else if args.WorkloadName == "" {
				resp = append(resp, workload)
			}
		}

		return resp
	})
	ctx.Resp = &NamespaceResource{
		Services: services,
	}
	ctx.Err = err
	c.Writer.Header().Set("X-Total", strconv.Itoa(count))
}

type workloadQueryArgs struct {
	ProjectName string `json:"projectName"     form:"projectName"`
	PerPage     int    `json:"perPage" form:"perPage,default=10"`
	Page        int    `json:"page"    form:"page,default=1"`
	Filter      string `json:"filter"  form:"filter"`
}

func ListWorkloadsInEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")

	args := &workloadQueryArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	count, services, err := commonservice.ListWorkloadsInEnv(envName, args.ProjectName, args.Filter, args.PerPage, args.Page, ctx.Logger)
	ctx.Resp = &NamespaceResource{
		Services: services,
	}
	ctx.Err = err
	c.Writer.Header().Set("X-Total", strconv.Itoa(count))
}
