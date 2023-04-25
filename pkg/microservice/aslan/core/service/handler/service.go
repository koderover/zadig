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
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func ListServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = commonservice.ListServiceTemplate(c.Query("projectName"), ctx.Logger)
}

func ListWorkloadTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = commonservice.ListWorkloadTemplate(c.Query("projectName"), c.Query("env"), ctx.Logger)
}

func GetServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = commonservice.GetServiceTemplate(c.Param("name"), c.Param("type"), c.Query("projectName"), setting.ProductStatusDeleting, revision, ctx.Logger)
}

func GetServiceTemplateOption(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.GetServiceTemplateOption(c.Param("name"), c.Query("projectName"), revision, ctx.Logger)
}

func CreateServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Service)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateServiceTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateServiceTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.ServiceName), string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ServiceTmpl json args")
		return
	}

	force, err := strconv.ParseBool(c.Query("force"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("force params error")
		return
	}

	args.CreateBy = ctx.UserName

	ctx.Resp, ctx.Err = svcservice.CreateServiceTemplate(ctx.UserName, args, force, ctx.Logger)
}

// UpdateServiceTemplate TODO figure out in which scene this function will be used
func UpdateServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	if args.Username != "system" {
		internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision), "", ctx.Logger)
	}
	args.Username = ctx.UserName
	ctx.Err = svcservice.UpdateServiceVisibility(args)
}

func UpdateServiceVariable(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	args.ProductName = c.Query("projectName")
	args.ServiceName = c.Param("name")
	args.Username = ctx.UserName
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务变量", fmt.Sprintf("服务名称:%s", args.ServiceName), "", ctx.Logger)
	ctx.Err = svcservice.UpdateServiceVariables(args)
}

func UpdateServiceHealthCheckStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	if args.Username != "system" {
		internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision), "", ctx.Logger)
	}
	args.Username = ctx.UserName
	ctx.Err = svcservice.UpdateServiceHealthCheckStatus(args)
}

type ValidatorResp struct {
	Message string `json:"message"`
}

func YamlValidator(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(svcservice.YamlValidatorReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}
	resp := make([]*ValidatorResp, 0)
	errMsgList := svcservice.YamlValidator(args)
	for _, errMsg := range errMsgList {
		resp = append(resp, &ValidatorResp{Message: errMsg})
	}
	ctx.Resp = resp
}

func YamlViewServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(svcservice.YamlViewServiceTemplateReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}
	args.ProjectName = c.Query("projectName")
	args.ServiceName = c.Param("name")
	if args.ProjectName == "" || args.ServiceName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName or serviceName can't be nil")
		return
	}
	ctx.Resp, ctx.Err = svcservice.YamlViewServiceTemplate(args)
}

func HelmReleaseNaming(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.ReleaseNamingRule)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "修改", "项目管理-服务", args.ServiceName, string(bs), ctx.Logger)

	ctx.Err = svcservice.UpdateReleaseNamingRule(ctx.UserName, ctx.RequestID, projectName, args, ctx.Logger)
}

func DeleteServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "删除", "项目管理-服务", c.Param("name"), "", ctx.Logger)

	ctx.Err = svcservice.DeleteServiceTemplate(c.Param("name"), c.Param("type"), c.Query("projectName"), c.DefaultQuery("isEnvTemplate", "true"), c.DefaultQuery("visibility", "public"), ctx.Logger)
}

func ListServicePort(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.ListServicePort(c.Param("name"), c.Param("type"), c.Query("projectName"), setting.ProductStatusDeleting, revision, ctx.Logger)
}

func UpdateWorkloads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(svcservice.UpdateWorkloadsArgs)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateWorkloads c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkloads json.Unmarshal err : %v", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "配置", "环境", c.Query("env"), string(data), ctx.Logger, c.Query("env"))

	err = c.ShouldBindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid UpdateWorkloadsArgs")
		return
	}
	product := c.Query("projectName")
	env := c.Query("env")
	if product == "" || env == "" {
		ctx.Err = e.ErrInvalidParam
		return
	}
	allowedEnvs, found := internalhandler.GetResourcesInHeader(c)
	if found {
		allowedSet := sets.NewString(allowedEnvs...)
		if !allowedSet.Has(env) {
			c.String(http.StatusForbidden, "not all input envs are allowed, allowed envs are %v", allowedEnvs)
			return
		}
	}

	ctx.Err = svcservice.UpdateWorkloads(c, ctx.RequestID, ctx.UserName, product, env, *args, ctx.Logger)
}

func CreateK8sWorkloads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(svcservice.K8sWorkloadsArgs)

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateK8sWorkloads c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateK8sWorkloads json.Unmarshal err : %v", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "新增", "环境", args.EnvName, string(data), ctx.Logger, args.EnvName)

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid K8sWorkloadsArgs args")
		return
	}

	ctx.Err = svcservice.CreateK8sWorkLoads(c, ctx.RequestID, ctx.UserName, args, ctx.Logger)
}

func ListAvailablePublicServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = svcservice.ListAvailablePublicServices(c.Query("projectName"), ctx.Logger)
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

func GetServiceTemplateObjectProductName(c *gin.Context) {
	args := new(commonservice.ServiceTmplObject)
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

func CreatePMService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(svcservice.ServiceTmplBuildObject)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreatePMService c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreatePMService json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Param("productName"), "新增", "项目管理-物理机部署服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision), string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid service json args")
		return
	}
	if args.Build.Name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("构建名称不能为空!")
		return
	}

	for _, heathCheck := range args.ServiceTmplObject.HealthChecks {
		if heathCheck.TimeOut < 2 || heathCheck.TimeOut > 60 {
			ctx.Err = e.ErrInvalidParam.AddDesc("超时时间必须在2-60之间")
			return
		}
		if heathCheck.Interval != 0 {
			if heathCheck.Interval < 2 || heathCheck.Interval > 60 {
				ctx.Err = e.ErrInvalidParam.AddDesc("间隔时间必须在2-60之间")
				return
			}
		}
		if heathCheck.HealthyThreshold != 0 {
			if heathCheck.HealthyThreshold < 2 || heathCheck.HealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("健康阈值必须在2-10之间")
				return
			}
		}
		if heathCheck.UnhealthyThreshold != 0 {
			if heathCheck.UnhealthyThreshold < 2 || heathCheck.UnhealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("不健康阈值必须在2-10之间")
				return
			}
		}
	}

	ctx.Err = svcservice.CreatePMService(ctx.UserName, args, ctx.Logger)
}

func UpdatePmServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ServiceTmplBuildObject)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdatePmServiceTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdatePmServiceTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Param("productName"), "更新", "项目管理-主机服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Revision), "", ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	for _, heathCheck := range args.ServiceTmplObject.HealthChecks {
		if heathCheck.TimeOut < 2 || heathCheck.TimeOut > 60 {
			ctx.Err = e.ErrInvalidParam.AddDesc("超时时间必须在2-60之间")
			return
		}
		if heathCheck.Interval != 0 {
			if heathCheck.Interval < 2 || heathCheck.Interval > 60 {
				ctx.Err = e.ErrInvalidParam.AddDesc("间隔时间必须在2-60之间")
				return
			}
		}
		if heathCheck.HealthyThreshold != 0 {
			if heathCheck.HealthyThreshold < 2 || heathCheck.HealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("健康阈值必须在2-10之间")
				return
			}
		}
		if heathCheck.UnhealthyThreshold != 0 {
			if heathCheck.UnhealthyThreshold < 2 || heathCheck.UnhealthyThreshold > 10 {
				ctx.Err = e.ErrInvalidParam.AddDesc("不健康阈值必须在2-10之间")
				return
			}
		}
	}
	ctx.Err = commonservice.UpdatePmServiceTemplate(ctx.UserName, args, ctx.Logger)
}
