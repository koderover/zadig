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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

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

// ListProducts list all product information
// Args: projectName, which is formerly known as productName, is the primary key of the project in our system
func ListProducts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("projectName") != "" {
		isProduction := c.Query("production")
		var envFilter string
		switch isProduction {
		case "true":
			envFilter = setting.ProdENV
		case "false":
			envFilter = setting.TestENV
		default:
			envFilter = isProduction
		}
		ctx.Resp, ctx.Err = service.ListProductsV2(c.Query("projectName"), envFilter, ctx.User.Name, ctx.User.ID, ctx.User.IsSuperUser, ctx.Logger)
		return
	}
	ctx.Resp, ctx.Err = service.ListProducts(c.Query("productName"), c.Query("envType"), ctx.User.Name, ctx.User.ID, ctx.User.IsSuperUser, ctx.Logger)
}

func AutoCreateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp = service.AutoCreateProduct(c.Param("productName"), c.Query("envType"), ctx.RequestID, ctx.Logger)
}

func AutoUpdateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(UpdateEnvs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("AutoUpdateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("AutoUpdateProduct json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "自动更新", "集成环境", strings.Join(args.EnvNames, ","), string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Logger.Error(err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	force, _ := strconv.ParseBool(c.Query("force"))
	ctx.Resp, ctx.Err = service.AutoUpdateProduct(args.EnvNames, c.Param("productName"), ctx.User.ID, ctx.User.IsSuperUser, ctx.RequestID, force, ctx.Logger)
}

func CreateHelmProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Param("productName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	args := new(service.CreateHelmProductArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProduct json.Unmarshal err : %v", err)
	}
	args.ProductName = c.Param("productName")

	if args.EnvName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "新增", "集成环境", args.EnvName, string(data), ctx.Logger)

	ctx.Err = service.CreateHelmProduct(
		ctx.Username, ctx.RequestID, args, ctx.Logger,
	)
}

func CreateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProduct json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "新增", "集成环境", args.EnvName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.EnvName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	args.UpdateBy = ctx.Username
	ctx.Err = service.CreateProduct(
		ctx.Username, ctx.RequestID, args, ctx.Logger,
	)
}

func UpdateProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Param("productName")

	args := new(commonmodels.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProduct json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, productName, "更新", "集成环境", envName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	force, _ := strconv.ParseBool(c.Query("force"))
	// update product asynchronously
	ctx.Err = service.UpdateProductV2(envName, productName, ctx.Username, ctx.RequestID, force, args.Vars, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product %s %s: %v", envName, productName, ctx.Err)
	}
}

func UpdateProductRecycleDay(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Param("productName")
	recycleDayStr := c.Query("recycleDay")

	internalhandler.InsertOperationLog(c, ctx.Username, productName, "更新", "集成环境-环境回收", envName, "", ctx.Logger)

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

	ctx.Err = service.UpdateProductRecycleDay(envName, productName, recycleDay)
}

func UpdateHelmProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	updateType := c.Query("updateType")
	productName := c.Param("productName")

	ctx.Err = service.UpdateHelmProduct(productName, envName, updateType, ctx.Username, ctx.RequestID, nil, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product %s %s: %v", envName, productName, ctx.Err)
	}
}

func UpdateHelmProductRenderCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Param("productName")
	if productName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can't be empty!")
		return
	}

	envName := c.Query("envName")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can't be empty!")
		return
	}

	arg := new(service.EnvRenderChartArg)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateHelmProductVariable c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, arg); err != nil {
		log.Errorf("UpdateHelmProductVariable json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "更新", "更新环境变量", "", string(data), ctx.Logger)

	ctx.Err = service.UpdateHelmProductRenderCharts(productName, envName, ctx.Username, ctx.RequestID, arg.ChartValues, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product Variable %s %s: %v", envName, productName, ctx.Err)
	}
}

func UpdateHelmProductVariable(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Param("productName")

	args := new(ChartInfoArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateHelmProductVariable c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateHelmProductVariable json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "更新", "helm集成环境变量", "", string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Logger.Error(err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = service.UpdateHelmProductVariable(productName, envName, ctx.Username, ctx.RequestID, args.ChartInfos, args.ChartInfos, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to update product Variable %s %s: %v", envName, productName, ctx.Err)
	}
}

func UpdateMultiHelmEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Param("productName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
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
	args.ProductName = c.Param("productName")

	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "更新", "集成环境", strings.Join(args.EnvNames, ","), string(data), ctx.Logger)

	ctx.Resp, ctx.Err = service.UpdateMultipleHelmEnv(
		ctx.Username, ctx.RequestID, ctx.User.ID, ctx.User.IsSuperUser, args, ctx.Logger,
	)
}

func UpdateMultiHelmProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(UpdateEnvs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateMultiHelmProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateMultiHelmProduct json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "更新helm环境", "集成环境", strings.Join(args.EnvNames, ","), string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Logger.Error(err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp = service.UpdateMultiHelmProduct(args.EnvNames, args.UpdateType, c.Param("productName"), ctx.User.ID, ctx.User.IsSuperUser, ctx.RequestID, ctx.Logger)
}

func GetProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Param("productName")

	ctx.Resp, ctx.Err = service.GetProduct(ctx.Username, envName, productName, ctx.Logger)
}

func GetProductInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Param("productName")

	ctx.Resp, ctx.Err = service.GetProductInfo(ctx.Username, envName, productName, ctx.Logger)
}

func GetProductIngress(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Param("productName")
	ctx.Resp, ctx.Err = service.GetProductIngress(ctx.Username, productName, ctx.Logger)
}

func ListRenderCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Param("productName")

	ctx.Resp, ctx.Err = service.ListRenderCharts(productName, envName, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to listRenderCharts %s %s: %v", envName, productName, ctx.Err)
	}
}

func GetHelmChartVersions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	envName := c.Query("envName")
	productName := c.Param("productName")

	ctx.Resp, ctx.Err = service.GetHelmChartVersions(productName, envName, ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to get helmVersions %s %s: %v", envName, productName, ctx.Err)
	}
}

func GetEstimatedRenderCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Query("productName")
	if productName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can't be empty!")
	}

	envName := c.Query("envName")
	if envName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can't be empty!")
	}

	ctx.Resp, ctx.Err = service.GetEstimatedRenderCharts(productName, envName, c.Query("serviceName"), ctx.Logger)
	if ctx.Err != nil {
		ctx.Logger.Errorf("failed to get estimatedRenderCharts %s %s: %v", envName, productName, ctx.Err)
	}
}

func DeleteProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	envName := c.Query("envName")

	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("productName"), "删除", "集成环境", envName, "", ctx.Logger)
	ctx.Err = commonservice.DeleteProduct(ctx.Username, envName, c.Param("productName"), ctx.RequestID, ctx.Logger)
}

func EnvShare(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	productName := c.Param("productName")

	args := new(service.ProductParams)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProduct c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProduct json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, productName, "更新", "集成环境-环境授权", args.EnvName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if productName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can't be empty!")
		return
	}

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.EnvName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can't be empty!")
		return
	}

	ctx.Err = service.UpdateProductPublic(productName, args, ctx.Logger)
}

func ListGroups(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Param("productName")
	envName := c.Query("envName")
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

	ctx.Resp, count, ctx.Err = service.ListGroups(serviceName, envName, productName, perPage, page, ctx.Logger)
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

// TODO: envName must be a param, while productName can be a query
type workloadQueryArgs struct {
	Env     string `json:"env"     form:"env"`
	PerPage int    `json:"perPage" form:"perPage,default=10"`
	Page    int    `json:"page"    form:"page,default=1"`
	Filter  string `json:"filter"  form:"filter"`
}

func ListWorkloadsInEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &workloadQueryArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	count, services, err := commonservice.ListWorkloadsInEnv(args.Env, c.Param("productName"), args.Filter, args.PerPage, args.Page, ctx.Logger)
	ctx.Resp = &NamespaceResource{
		Services: services,
	}
	ctx.Err = err
	c.Writer.Header().Set("X-Total", strconv.Itoa(count))
}
