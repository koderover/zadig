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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	deliveryservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/delivery/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func GetProductNameByDelivery(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	id := c.Param("id")
	if id == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id can't be empty!")
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
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	//params validate
	ID := c.Param("id")
	if ID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}
	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

	permit := false
	if ctx.Resources.IsSystemAdmin {
		permit = true
	} else {
		if ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
			permit = true
		}

		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
			if ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin ||
				ctx.Resources.ProjectAuthInfo[projectKey].Version.View {
				permit = true
			}
		}
	}

	if !permit {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	version := new(commonrepo.DeliveryVersionArgs)
	version.ID = ID
	ctx.Resp, ctx.RespErr = deliveryservice.GetDetailReleaseData(version, ctx.Logger)
}

func ListDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(deliveryservice.ListDeliveryVersionArgs)
	err = c.BindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	projectKey := args.ProjectName
	if !ctx.Resources.IsSystemAdmin {
		if projectKey == "" {
			if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
				ctx.UnAuthorized = true
				return
			}

			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Version.View {
				ctx.UnAuthorized = true
				return
			}
		}
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

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	var total int
	ctx.Resp, total, ctx.RespErr = deliveryservice.ListDeliveryVersion(args, ctx.Logger)
	c.Writer.Header().Add("X-Total", strconv.Itoa(total))
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

// @Summary Create K8S Delivery Version
// @Description Create K8S Delivery Version
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	body 		body 		deliveryservice.CreateK8SDeliveryVersionArgs 	true 	"body"
// @Success 200
// @Router /api/aslan/delivery/releases/k8s [post]
func CreateK8SDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(deliveryservice.CreateK8SDeliveryVersionArgs)
	err = c.ShouldBindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	args.CreateBy = ctx.UserName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Version.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新建", "版本交付", fmt.Sprintf("%s-%s", args.EnvName, args.Version), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = deliveryservice.CreateK8SDeliveryVersion(args, ctx.Logger)
}

func CreateHelmDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(deliveryservice.CreateHelmDeliveryVersionArgs)
	err = c.ShouldBindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	args.CreateBy = ctx.UserName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Version.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新建", "版本交付", fmt.Sprintf("%s-%s", args.EnvName, args.Version), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = deliveryservice.CreateHelmDeliveryVersion(args, ctx.Logger)
}

func DeleteDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("productName")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "删除", "版本交付", c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Version.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	//params validate
	ID := c.Param("id")
	if ID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("ID can't be empty!")
		return
	}
	version := new(commonrepo.DeliveryVersionArgs)
	version.ID = ID
	ctx.RespErr = deliveryservice.DeleteDeliveryVersion(version, ctx.Logger)
	if ctx.RespErr != nil {
		ctx.RespErr = fmt.Errorf("failed to delete delivery version, ID: %s, error: %v", ID, ctx.RespErr)
	}

	errs := make([]string, 0)
	err = deliveryservice.DeleteDeliveryBuild(&commonrepo.DeliveryBuildArgs{ReleaseID: ID}, ctx.Logger)
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
		ctx.RespErr = e.NewHTTPError(500, strings.Join(errs, ","))
	}
}

func DownloadDeliveryChart(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
			ctx.UnAuthorized = true
			return
		}
	}

	versionName := c.Query("version")
	chartName := c.Query("chartName")
	projectName := c.Query("projectName")

	fileBytes, fileName, err := deliveryservice.DownloadDeliveryChart(projectName, versionName, chartName, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Data(http.StatusOK, "application/octet-stream", fileBytes)
}

func GetChartVersionFromRepo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authorization leak
	// authorization checks
	//if !ctx.Resources.IsSystemAdmin {
	//	if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
	//		ctx.UnAuthorized = true
	//		return
	//	}
	//}

	chartName := c.Query("chartName")
	chartRepoName := c.Query("chartRepoName")

	ctx.Resp, ctx.RespErr = deliveryservice.GetChartVersion(chartName, chartRepoName)
}

func PreviewGetDeliveryChart(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
			ctx.UnAuthorized = true
			return
		}
	}

	versionName := c.Query("version")
	chartName := c.Query("chartName")
	projectName := c.Query("projectName")

	ctx.Resp, ctx.RespErr = deliveryservice.PreviewDeliveryChart(projectName, versionName, chartName, ctx.Logger)
}

func GetDeliveryChartFilePath(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(deliveryservice.DeliveryChartFilePathArgs)
	err = c.BindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = deliveryservice.GetDeliveryChartFilePath(args, ctx.Logger)
}

func GetDeliveryChartFileContent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(deliveryservice.DeliveryChartFileContentArgs)
	err = c.BindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = deliveryservice.GetDeliveryChartFileContent(args, ctx.Logger)
}

func ApplyDeliveryGlobalVariables(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(deliveryservice.DeliveryVariablesApplyArgs)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = deliveryservice.ApplyDeliveryGlobalVariables(args, ctx.Logger)
}

// @Summary Check Delivery Version
// @Description Check Delivery Version
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	version			query		string							true	"version"
// @Success 200
// @Router /api/aslan/delivery/releases/check [get]
func CheckDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	versionName := c.Query("version")
	projectName := c.Query("projectName")

	ctx.RespErr = deliveryservice.CheckDeliveryVersion(projectName, versionName)
}
