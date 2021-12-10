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
	"io/ioutil"
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

type UpdateEnvs struct {
	EnvNames   []string `json:"env_names"`
	UpdateType string   `json:"update_type"`
}

type ChartInfoArgs struct {
	ChartInfos []*template.RenderChart `json:"chart_infos"`
}

type NamespaceResource struct {
	Services  []*commonservice.ServiceResp `json:"services"`
	Ingresses []resource.Ingress           `json:"ingresses"`
}

type UpdateProductRegistryRequest struct {
	RegistryID string `json:"registry_id"`
	Namespace  string `json:"namespace"`
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

	switch c.Query(setting.Subresource) {
	case setting.IngressSubresource:
		ctx.Resp, ctx.Err = service.GetProductIngress(projectName, ctx.Logger)
		return
	}

	ctx.Resp, ctx.Err = service.ListProducts(projectName, envNames, ctx.Logger)
}

func UpdateMultiProducts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("helm") == "true" {
		updateMultiHelmEnv(c, ctx)
		return
	}

	if c.Query("auto") != "true" {
		ctx.Err = e.ErrInvalidParam.AddDesc("auto=true is required")
		return
	}

	args := new(UpdateEnvs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateMultiProducts c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateMultiProducts json.Unmarshal err : %v", err)
	}

	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		currentSet := sets.NewString(args.EnvNames...)
		if !allowedSet.IsSuperset(currentSet) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "自动更新", "集成环境", strings.Join(args.EnvNames, ","), string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Logger.Error(err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	force, _ := strconv.ParseBool(c.Query("force"))
	ctx.Resp, ctx.Err = service.AutoUpdateProduct(args.EnvNames, c.Query("projectName"), ctx.RequestID, force, ctx.Logger)
}

func createHelmProduct(c *gin.Context, ctx *internalhandler.Context) {
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}
	createArgs := make([]*service.CreateHelmProductArg, 0)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateHelmProduct c.GetRawData() err : %v", err)
	} else if err = json.Unmarshal(data, &createArgs); err != nil {
		log.Errorf("CreateHelmProduct json.Unmarshal err : %v", err)
	}
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	envNameList := make([]string, 0)
	for _, arg := range createArgs {
		if arg.EnvName == "" {
			ctx.Err = e.ErrInvalidParam.AddDesc("envName is empty")
			return
		}
		arg.ProductName = projectName
		envNameList = append(envNameList, arg.EnvName)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "新增", "集成环境", strings.Join(envNameList, "-"), string(data), ctx.Logger)

	ctx.Err = service.CreateHelmProduct(
		projectName, ctx.UserName, ctx.RequestID, createArgs, ctx.Logger,
	)
}

func CreateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("auto") == "true" {
		ctx.Resp = service.AutoCreateProduct(c.Query("projectName"), c.Query("envType"), ctx.RequestID, ctx.Logger)
		return
	}
	if c.Query("helm") == "true" {
		createHelmProduct(c, ctx)
		return
	}

	args := new(commonmodels.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProduct json.Unmarshal err : %v", err)
	}

	allowedClusters, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedClusters...)
		if !allowedSet.Has(args.ClusterID) {
			c.String(http.StatusForbidden, "permission denied for cluster %s", args.ClusterID)
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "集成环境", args.EnvName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.EnvName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	if args.RegistryID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("RegistryId can not be null!")
		return
	}

	args.UpdateBy = ctx.UserName
	ctx.Err = service.CreateProduct(
		ctx.UserName, ctx.RequestID, args, ctx.Logger,
	)
}

func UpdateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")

	args := new(commonmodels.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProduct json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "集成环境", envName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	force, _ := strconv.ParseBool(c.Query("force"))
	// update product asynchronously
	ctx.Err = service.UpdateProductV2(envName, projectName, ctx.UserName, ctx.RequestID, force, args.Vars, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product %s %s: %v", envName, projectName, ctx.Err)
	}
}

func UpdateProductRegistry(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	args := new(UpdateProductRegistryRequest)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProduct c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProduct json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "集成环境", args.Namespace, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = service.UpdateProductRegistry(args.Namespace, args.RegistryID, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product %s %s: %v", args.Namespace, args.RegistryID, ctx.Err)
	}
}

func UpdateProductRecycleDay(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Param("name")
	projectName := c.Query("projectName")
	recycleDayStr := c.Query("recycleDay")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "集成环境-环境回收", envName, "", ctx.Logger)

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

func UpdateHelmProductRenderset(c *gin.Context) {
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

	arg := new(service.EnvRendersetArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateHelmProductVariable c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, arg); err != nil {
		log.Errorf("UpdateHelmProductVariable json.Unmarshal err : %v", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "更新环境变量", "", string(data), ctx.Logger)

	ctx.Err = service.UpdateHelmProductRenderset(projectName, envName, ctx.UserName, ctx.RequestID, arg, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product Variable %s %s: %v", envName, projectName, ctx.Err)
	}
}

func updateMultiHelmEnv(c *gin.Context, ctx *internalhandler.Context) {
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can not be empty")
		return
	}

	args := new(service.UpdateMultiHelmProductArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProduct json.Unmarshal err : %v", err)
	}
	args.ProductName = projectName

	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		currentSet := sets.NewString(args.EnvNames...)
		if !allowedSet.IsSuperset(currentSet) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "集成环境", strings.Join(args.EnvNames, ","), string(data), ctx.Logger)

	ctx.Resp, ctx.Err = service.UpdateMultipleHelmEnv(
		ctx.RequestID, args, ctx.Logger,
	)
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

	envName := c.Param("Name")
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
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "删除", "集成环境", envName, "", ctx.Logger)
	ctx.Err = commonservice.DeleteProduct(ctx.UserName, envName, projectName, ctx.RequestID, ctx.Logger)
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
