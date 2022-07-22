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
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	deliveryservice "github.com/koderover/zadig/pkg/microservice/aslan/core/delivery/service"
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
	ctx.Resp, ctx.Err = deliveryservice.GetDetailReleaseData(version, ctx.Logger)
}

func ListDeliveryVersion(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(deliveryservice.ListDeliveryVersionArgs)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if args.Page <= 0 {
		args.Page = 1
	}
	if args.PerPage <= 0 {
		args.PerPage = 20
	}
	if len(args.Verbosity) == 0 {
		args.Verbosity = deliveryservice.VerbosityDetailed
	}

	ctx.Resp, ctx.Err = deliveryservice.ListDeliveryVersion(args, ctx.Logger)
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

	version := &commonrepo.DeliveryVersionArgs{
		ProductName: c.Query("projectName"),
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

func CreateHelmDeliveryVersion(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(deliveryservice.CreateHelmDeliveryVersionArgs)
	err := c.ShouldBindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	args.CreateBy = ctx.UserName

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新建", "版本交付", fmt.Sprintf("%s-%s", args.EnvName, args.Version), string(bs), ctx.Logger)

	ctx.Err = deliveryservice.CreateHelmDeliveryVersion(args, ctx.Logger)
}

func DeleteDeliveryVersion(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, c.GetString("productName"), "删除", "版本交付", c.Param("id"), "", ctx.Logger)

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

	productName := c.Query("projectName")
	ctx.Resp, ctx.Err = deliveryservice.ListDeliveryServiceNames(productName, ctx.Logger)
}

func DownloadDeliveryChart(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	versionName := c.Query("version")
	chartName := c.Query("chartName")
	projectName := c.Query("projectName")

	fileBytes, fileName, err := deliveryservice.DownloadDeliveryChart(projectName, versionName, chartName, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Data(http.StatusOK, "application/octet-stream", fileBytes)
}

func GetChartVersionFromRepo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	chartName := c.Query("chartName")
	chartRepoName := c.Query("chartRepoName")

	ctx.Resp, ctx.Err = deliveryservice.GetChartVersion(chartName, chartRepoName)
}

func PreviewGetDeliveryChart(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	versionName := c.Query("version")
	chartName := c.Query("chartName")
	projectName := c.Query("projectName")

	ctx.Resp, ctx.Err = deliveryservice.PreviewDeliveryChart(projectName, versionName, chartName, ctx.Logger)
}

func GetDeliveryChartFilePath(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(deliveryservice.DeliveryChartFilePathArgs)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = deliveryservice.GetDeliveryChartFilePath(args, ctx.Logger)
}

func GetDeliveryChartFileContent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(deliveryservice.DeliveryChartFileContentArgs)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = deliveryservice.GetDeliveryChartFileContent(args, ctx.Logger)
}

func ApplyDeliveryGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(deliveryservice.DeliveryVariablesApplyArgs)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = deliveryservice.ApplyDeliveryGlobalVariables(args, ctx.Logger)
}
