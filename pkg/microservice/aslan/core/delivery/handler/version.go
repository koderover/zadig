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
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	deliveryservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/delivery/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary Get Delivery Version
// @Description Get Delivery Version
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	id 		path 		string							true	"id"
// @Success 200     {object} 	models.DeliveryVersionV2
// @Router /api/aslan/delivery/releases/:id [get]
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

	version := new(commonrepo.DeliveryVersionV2Args)
	version.ID = ID
	ctx.Resp, ctx.RespErr = deliveryservice.GetDeliveryVersionDetailV2(version, ctx.Logger)
}

// @Summary List Delivery Version
// @Description List Delivery Version
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	projectName 		query 		string							true	"projectName"
// @Param 	label 				query 		string							true	"label"
// @Param 	page 				query 		int								true	"page"
// @Param 	perPage 			query 		int								true	"perPage"
// @Success 200     {array} 	models.DeliveryVersionV2
// @Router /api/aslan/delivery/releases [get]
func ListDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(deliveryservice.ListDeliveryVersionV2Args)
	err = c.BindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	if args.Label != "" {
		if decodedLabel, decodeErr := url.QueryUnescape(args.Label); decodeErr == nil && decodedLabel != "" {
			args.Label = decodedLabel
		} else {
			ctx.RespErr = e.ErrInvalidParam.AddErr(decodeErr)
			return
		}
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

	if len(args.Verbosity) == 0 {
		args.Verbosity = deliveryservice.VerbosityDetailed
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	var total int
	ctx.Resp, total, ctx.RespErr = deliveryservice.ListDeliveryVersionV2(args, ctx.Logger)
	c.Writer.Header().Add("X-Total", strconv.Itoa(total))
}

// @Summary List Delivery Version Labels
// @Description List Delivery Version Labels
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	projectName 		query 		string							true	"projectName"
// @Success 200     {array} 	string
// @Router /api/aslan/delivery/releases/labels [get]
func ListDeliveryVersionLabels(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
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

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = deliveryservice.ListDeliveryVersionV2Labels(projectKey)
}

// @Summary Create K8S Delivery Version
// @Description Create K8S Delivery Version
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	body 		body 		deliveryservice.CreateDeliveryVersionRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/delivery/releases/k8s [post]
func CreateK8SDeliveryVersionV2(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(deliveryservice.CreateDeliveryVersionRequest)
	err = c.ShouldBindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	args.CreateBy = ctx.UserName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Version.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新建", "版本交付", fmt.Sprintf("%s", args.Version), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = deliveryservice.CreateK8SDeliveryVersionV2(args, ctx.Logger)
}

// @Summary Create Helm Delivery Version
// @Description Create Helm Delivery Version
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	body 		body 		deliveryservice.CreateDeliveryVersionRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/delivery/releases/helm [post]
func CreateHelmDeliveryVersionV2(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(deliveryservice.CreateDeliveryVersionRequest)
	err = c.ShouldBindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	args.CreateBy = ctx.UserName

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Version.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新建", "版本交付", fmt.Sprintf("%s-%s", args.EnvName, args.Version), string(bs), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = deliveryservice.CreateHelmDeliveryVersionV2(args, ctx.Logger)
}

// @Summary Retry Delivery Version
// @Description Retry Delivery Version
// @Tags 	delivery
// @Accept 	json
// @Produce json
// @Param 	id				query		string							true	"id"
// @Success 200
// @Router /api/aslan/delivery/releases/retry [post]
func RetryDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

	id := c.Query("id")
	if id == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Version.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = deliveryservice.RetryDeliveryVersionV2(id, ctx.Logger)
}

func DeleteDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectName")

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
	version := new(commonrepo.DeliveryVersionV2Args)
	version.ID = ID
	ctx.RespErr = deliveryservice.DeleteDeliveryVersionV2(version, ctx.Logger)
	if ctx.RespErr != nil {
		ctx.RespErr = fmt.Errorf("failed to delete delivery version, ID: %s, error: %v", ID, ctx.RespErr)
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

// func PreviewGetDeliveryChart(c *gin.Context) {
// 	ctx, err := internalhandler.NewContextWithAuthorization(c)
// 	defer func() { internalhandler.JSONResponse(c, ctx) }()

// 	if err != nil {

// 		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
// 		ctx.UnAuthorized = true
// 		return
// 	}

// 	// authorization checks
// 	if !ctx.Resources.IsSystemAdmin {
// 		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
// 			ctx.UnAuthorized = true
// 			return
// 		}
// 	}

// 	versionName := c.Query("version")
// 	chartName := c.Query("chartName")
// 	projectName := c.Query("projectName")

// 	ctx.Resp, ctx.RespErr = deliveryservice.PreviewDeliveryChart(projectName, versionName, chartName, ctx.Logger)
// }

// func GetDeliveryChartFilePath(c *gin.Context) {
// 	ctx, err := internalhandler.NewContextWithAuthorization(c)
// 	defer func() { internalhandler.JSONResponse(c, ctx) }()

// 	if err != nil {

// 		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
// 		ctx.UnAuthorized = true
// 		return
// 	}

// 	// authorization checks
// 	if !ctx.Resources.IsSystemAdmin {
// 		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
// 			ctx.UnAuthorized = true
// 			return
// 		}
// 	}

// 	args := new(deliveryservice.DeliveryChartFilePathArgs)
// 	err = c.BindQuery(args)
// 	if err != nil {
// 		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
// 		return
// 	}

// 	ctx.Resp, ctx.RespErr = deliveryservice.GetDeliveryChartFilePath(args, ctx.Logger)
// }

// func GetDeliveryChartFileContent(c *gin.Context) {
// 	ctx, err := internalhandler.NewContextWithAuthorization(c)
// 	defer func() { internalhandler.JSONResponse(c, ctx) }()

// 	if err != nil {

// 		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
// 		ctx.UnAuthorized = true
// 		return
// 	}

// 	// authorization checks
// 	if !ctx.Resources.IsSystemAdmin {
// 		if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
// 			ctx.UnAuthorized = true
// 			return
// 		}
// 	}

// 	args := new(deliveryservice.DeliveryChartFileContentArgs)
// 	err = c.BindQuery(args)
// 	if err != nil {
// 		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
// 		return
// 	}
// 	ctx.Resp, ctx.RespErr = deliveryservice.GetDeliveryChartFileContent(args, ctx.Logger)
// }

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
