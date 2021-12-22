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

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
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
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision), string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ServiceTmpl json args")
		return
	}
	args.CreateBy = ctx.UserName

	ctx.Resp, ctx.Err = svcservice.CreateServiceTemplate(ctx.UserName, args, ctx.Logger)
}

func UpdateServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ServiceTmplObject)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateServiceTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateServiceTemplate json.Unmarshal err : %v", err)
	}
	if args.Username != "system" {
		internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s,版本号:%d", args.ServiceName, args.Revision), "", ctx.Logger)
	}
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	args.Username = ctx.UserName
	ctx.Err = svcservice.UpdateServiceTemplate(args)
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

type K8sWorkloadsArgs struct {
	WorkLoads   []commonmodels.Workload `bson:"workLoads"        json:"workLoads"`
	EnvName     string                  `bson:"env_name"         json:"env_name"`
	ClusterID   string                  `bson:"cluster_id"       json:"cluster_id"`
	Namespace   string                  `bson:"namespace"        json:"namespace"`
	ProductName string                  `bson:"product_name"     json:"product_name"`
}

func UpdateWorkloads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(service.UpdateWorkloadsArgs)
	err := c.ShouldBindJSON(args)
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
	ctx.Err = service.UpdateWorkloads(c, ctx.RequestID, ctx.UserName, product, env, *args, ctx.Logger)
}

func CreateK8sWorkloads(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := new(K8sWorkloadsArgs)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid K8sWorkloadsArgs args")
		return
	}

	ctx.Err = svcservice.CreateK8sWorkLoads(c, ctx.RequestID, ctx.UserName, args.ProductName, args.WorkLoads, args.ClusterID, args.Namespace, args.EnvName, ctx.Logger)
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
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
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
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
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
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

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
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

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
