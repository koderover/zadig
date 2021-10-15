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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	deliveryservice "github.com/koderover/zadig/pkg/microservice/aslan/core/delivery/service"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetProductNameByDelivery(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	id := c.Param("id")
	if id == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}
	args := new(commonrepo.DeliveryVersionArgs)
	args.ID = id
	deliveryVersion, err := deliveryservice.GetDeliveryVersion(args, ctx.Logger)
	if err != nil {
		log.Errorf("GetDeliveryVersion err : %v", err)
		return
	}
	c.Set("productName", deliveryVersion.ProductName)
	c.Next()
}

func GetDeliveryVersion(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	//params validate
	ID := c.Param("id")
	if ID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}
	version := new(commonrepo.DeliveryVersionArgs)
	version.ID = ID
	ctx.Resp, ctx.Err = deliveryservice.GetDeliveryVersion(version, ctx.Logger)
}

type ReleaseInfo struct {
	VersionInfo    *commonmodels.DeliveryVersion      `json:"versionInfo"`
	BuildInfo      []*commonmodels.DeliveryBuild      `json:"buildInfo"`
	DeployInfo     []*commonmodels.DeliveryDeploy     `json:"deployInfo"`
	TestInfo       []*commonmodels.DeliveryTest       `json:"testInfo"`
	DistributeInfo []*commonmodels.DeliveryDistribute `json:"distributeInfo"`
	SecurityInfo   []*DeliverySecurityStats           `json:"securityStatsInfo"`
}

type DeliverySecurityStats struct {
	ImageName                 string                    `json:"imageName"`
	ImageID                   string                    `json:"imageId"`
	DeliverySecurityStatsInfo DeliverySecurityStatsInfo `json:"deliverySecurityStatsInfo"`
}

type DeliverySecurityStatsInfo struct {
	Total      int `json:"total"`
	Unknown    int `json:"unkown"`
	Negligible int `json:"negligible"`
	Low        int `json:"low"`
	Medium     int `json:"medium"`
	High       int `json:"high"`
	Critical   int `json:"critical"`
}

func ListDeliveryVersion(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	orgIDStr := c.Query("orgId")
	orgID, err := strconv.Atoi(orgIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("orgId can't be empty!")
		return
	}

	taskIDStr := c.Query("taskId")
	var taskID = 0
	if taskIDStr != "" {
		taskID, err = strconv.Atoi(taskIDStr)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
			return
		}
	}

	perPageStr := c.Query("per_page")
	pageStr := c.Query("page")
	var (
		perPage int
		page    int
	)
	if perPageStr == "" {
		perPage = 20
	} else {
		perPage, err = strconv.Atoi(perPageStr)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(fmt.Sprintf("perPage args err :%s", err))
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

	serviceName := c.Query("serviceName")

	version := new(commonrepo.DeliveryVersionArgs)
	version.OrgID = orgID
	version.ProductName = c.Query("productName")
	version.WorkflowName = c.Query("workflowName")
	version.TaskID = taskID
	version.PerPage = perPage
	version.Page = page
	deliveryVersions, err := deliveryservice.FindDeliveryVersion(version, ctx.Logger)

	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	releaseInfos := make([]*ReleaseInfo, 0)
	for _, deliveryVersion := range deliveryVersions {
		releaseInfo := new(ReleaseInfo)
		//versionInfo
		releaseInfo.VersionInfo = deliveryVersion

		//deployInfo
		deliveryDeployArgs := new(commonrepo.DeliveryDeployArgs)
		deliveryDeployArgs.ReleaseID = deliveryVersion.ID.Hex()
		deliveryDeploys, err := deliveryservice.FindDeliveryDeploy(deliveryDeployArgs, ctx.Logger)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		}
		// 查询条件serviceName
		if serviceName != "" {
			match := false
			for _, deliveryDeploy := range deliveryDeploys {
				if deliveryDeploy.ServiceName == serviceName {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		// 将serviceName替换为服务名/服务组件的形式，用于前端展示
		for _, deliveryDeploy := range deliveryDeploys {
			if deliveryDeploy.ContainerName != "" {
				deliveryDeploy.ServiceName = deliveryDeploy.ServiceName + "/" + deliveryDeploy.ContainerName
			}
		}
		releaseInfo.DeployInfo = deliveryDeploys

		//buildInfo
		deliveryBuildArgs := new(commonrepo.DeliveryBuildArgs)
		deliveryBuildArgs.ReleaseID = deliveryVersion.ID.Hex()
		deliveryBuilds, err := deliveryservice.FindDeliveryBuild(deliveryBuildArgs, ctx.Logger)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		}
		releaseInfo.BuildInfo = deliveryBuilds

		//testInfo
		deliveryTestArgs := new(commonrepo.DeliveryTestArgs)
		deliveryTestArgs.ReleaseID = deliveryVersion.ID.Hex()
		deliveryTests, err := deliveryservice.FindDeliveryTest(deliveryTestArgs, ctx.Logger)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		}
		releaseInfo.TestInfo = deliveryTests

		//securityStatsInfo
		deliverySecurityStatss := make([]*DeliverySecurityStats, 0)
		if pipelineTask, err := workflowservice.GetPipelineTaskV2(int64(deliveryVersion.TaskID), deliveryVersion.WorkflowName, config.WorkflowType, ctx.Logger); err == nil {
			for _, subStage := range pipelineTask.Stages {
				if subStage.TaskType == config.TaskSecurity {
					subSecurityTaskMap := subStage.SubTasks
					for _, subTask := range subSecurityTaskMap {
						securityInfo, _ := base.ToSecurityTask(subTask)

						deliverySecurityStats := new(DeliverySecurityStats)
						deliverySecurityStats.ImageName = securityInfo.ImageName
						deliverySecurityStats.ImageID = securityInfo.ImageID
						deliverySecurityStatsMap, err := deliveryservice.FindDeliverySecurityStatistics(securityInfo.ImageID, ctx.Logger)
						if err != nil {
							ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
						}
						var transErr error
						b, err := json.Marshal(deliverySecurityStatsMap)
						if err != nil {
							transErr = fmt.Errorf("marshal task error: %v", err)
						}

						if err := json.Unmarshal(b, &deliverySecurityStats.DeliverySecurityStatsInfo); err != nil {
							transErr = fmt.Errorf("unmarshal task error: %v", err)
						}
						if transErr != nil {
							ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
						}

						deliverySecurityStatss = append(deliverySecurityStatss, deliverySecurityStats)
					}
					break
				}
			}
			releaseInfo.SecurityInfo = deliverySecurityStatss
		}

		//distributeInfo
		deliveryDistributeArgs := new(commonrepo.DeliveryDistributeArgs)
		deliveryDistributeArgs.ReleaseID = deliveryVersion.ID.Hex()
		deliveryDistributes, _ := deliveryservice.FindDeliveryDistribute(deliveryDistributeArgs, ctx.Logger)

		releaseInfo.DistributeInfo = deliveryDistributes

		releaseInfos = append(releaseInfos, releaseInfo)
	}
	ctx.Err = err
	ctx.Resp = releaseInfos
}

func getFileName(fileName string) string {
	names := strings.Split(fileName, "-")
	if len(names) > 0 {
		return names[0]
	}
	return ""
}
func getFileVersion(fileName string) string {
	fileNameArray := strings.Split(fileName, "-")
	if len(fileNameArray) > 3 {
		return fmt.Sprintf("%s-%s-%s", fileNameArray[1], fileNameArray[2], fileNameArray[3])
	}
	return ""
}

type DeliveryFileDetail struct {
	FileVersion     string `json:"fileVersion"`
	DeliveryVersion string `json:"deliveryVersion"`
	DeliveryID      string `json:"deliveryId"`
}

type DeliveryFileInfo struct {
	FileName           string               `json:"fileName"`
	DeliveryFileDetail []DeliveryFileDetail `json:"versionInfo"`
}

func ListPackagesVersion(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	//params validate
	orgIDStr := c.Query("orgId")
	orgID, err := strconv.Atoi(orgIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("orgId can't be empty!")
		return
	}
	version := &commonrepo.DeliveryVersionArgs{
		OrgID:       orgID,
		ProductName: c.Query("productName"),
	}
	deliveryVersions, err := deliveryservice.FindDeliveryVersion(version, ctx.Logger)
	if err != nil {
		ctx.Err = e.NewHTTPError(500, err.Error())
	}

	fileMap := map[string][]DeliveryFileDetail{}

	for _, deliveryVersion := range deliveryVersions {
		//delivery := deliveryVersion
		args := &commonrepo.DeliveryDistributeArgs{
			ReleaseID:      deliveryVersion.ID.Hex(),
			DistributeType: config.File,
		}
		distributeVersion, err := deliveryservice.FindDeliveryDistribute(args, ctx.Logger)
		if err != nil {
			ctx.Err = e.NewHTTPError(500, err.Error())
		}

		for _, distribute := range distributeVersion {
			packageFile := distribute.PackageFile

			fileMap[getFileName(packageFile)] = append(
				fileMap[getFileName(packageFile)], DeliveryFileDetail{
					FileVersion:     getFileVersion(packageFile),
					DeliveryVersion: deliveryVersion.Version,
					DeliveryID:      deliveryVersion.ID.Hex(),
				})
		}
	}

	fileInfoList := make([]*DeliveryFileInfo, 0)
	for fileName, versionList := range fileMap {
		info := DeliveryFileInfo{
			FileName:           fileName,
			DeliveryFileDetail: versionList,
		}
		fileInfoList = append(fileInfoList, &info)
	}

	ctx.Err = err
	ctx.Resp = fileInfoList
}

func DeleteDeliveryVersion(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.Username, c.GetString("productName"), "删除", "版本交付", fmt.Sprintf("主键ID:%s", c.Param("id")), "", ctx.Logger)

	//params validate
	ID := c.Param("id")
	if ID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("ID can't be empty!")
		return
	}
	version := new(commonrepo.DeliveryVersionArgs)
	version.ID = ID
	ctx.Err = deliveryservice.DeleteDeliveryVersion(version, ctx.Logger)

	errs := make([]string, 0)
	err := deliveryservice.DeleteDeliveryBuild(&commonrepo.DeliveryBuildArgs{ReleaseID: ID}, ctx.Logger)
	if err != nil {
		errs = append(errs, err.Error())
	}
	err = deliveryservice.DeleteDeliveryDeploy(&commonrepo.DeliveryDeployArgs{ReleaseID: ID}, ctx.Logger)
	if err != nil {
		errs = append(errs, err.Error())
	}
	err = deliveryservice.DeleteDeliveryTest(&commonrepo.DeliveryTestArgs{ReleaseID: ID}, ctx.Logger)
	if err != nil {
		errs = append(errs, err.Error())
	}
	err = deliveryservice.DeleteDeliveryDistribute(&commonrepo.DeliveryDistributeArgs{ReleaseID: ID}, ctx.Logger)
	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		ctx.Err = e.NewHTTPError(500, strings.Join(errs, ","))
	}
}

func ListDeliveryServiceNames(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Query("productName")
	orgIDStr := c.Query("orgId")
	orgID, err := strconv.Atoi(orgIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("orgId can't be empty!")
		return
	}

	ctx.Resp, ctx.Err = deliveryservice.ListDeliveryServiceNames(orgID, productName, ctx.Logger)
}
