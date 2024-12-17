/*
Copyright 2023 The KodeRover Authors.

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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/project/service"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

// @Summary Create Project CodeHost
// @Description Create Project CodeHost
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name			path		string					true	"project name"
// @Param 	body 			body 		models.CodeHost 		true 	"body"
// @Success 200
// @Router /api/aslan/project/integration/{name}/codehosts [post]
func CreateProjectCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	req := new(models.CodeHost)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	if err != nil {
		ctx.RespErr = e.ErrLicenseInvalid.AddDesc("")
		return
	}

	ctx.Resp, ctx.RespErr = service.CreateProjectCodeHost(projectKey, req, ctx.Logger)
}

// @Summary List Project CodeHost
// @Description List Project CodeHost
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name			path		string					true	"project name"
// @Param 	encryptedKey	query		string					true	"encrypted key"
// @Success 200  			{array} 	models.CodeHost
// @Router /api/aslan/project/integration/{name}/codehosts [get]
func ListProjectCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// TODO: Authorization leaks
	// authorization checks
	//if !ctx.Resources.IsSystemAdmin {
	//	if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
	//		ctx.UnAuthorized = true
	//		return
	//	}
	//	if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
	//		ctx.UnAuthorized = true
	//		return
	//	}
	//}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	ctx.Resp, ctx.RespErr = service.GetProjectCodehostList(projectKey, encryptedKey, c.Query("address"), c.Query("owner"), c.Query("source"), ctx.Logger)
}

// @Summary Delete Project CodeHost
// @Description Delete Project CodeHost
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name			path		string					true	"project name"
// @Param 	id				path		int						true	"code host id"
// @Success 200
// @Router /api/aslan/project/integration/{name}/codehosts/{id} [delete]
func DeleteCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.RespErr = service.DeleteProjectCodeHostByID(projectKey, id, ctx.Logger)
}

// @Summary Update Project CodeHost
// @Description Update Project CodeHost
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name			path		string					true	"project name"
// @Param 	id				path		int						true	"code host id"
// @Param 	body 			body 		models.CodeHost 		true 	"body"
// @Success 200
// @Router /api/aslan/project/integration/{name}/codehosts/{id} [patch]
func UpdateProjectCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// license checks
	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.RespErr = err
		return
	}
	req := &models.CodeHost{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}
	req.ID = id

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = e.ErrLicenseInvalid.AddDesc("")
		return
	}

	ctx.Resp, ctx.RespErr = service.UpdateProjectSystemCodeHost(projectKey, req, ctx.Logger)
}

// @Summary Get Project CodeHost
// @Description Get Project CodeHost
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name			path		string					true	"project name"
// @Param 	id				path		int						true	"code host id"
// @Success 200  			{object} 	models.CodeHost
// @Router /api/aslan/project/integration/{name}/codehosts/{id} [get]
func GetProjectCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ignoreDelete := false
	if len(c.Query("ignoreDelete")) > 0 {
		ignoreDelete, err = strconv.ParseBool(c.Query("ignoreDelete"))
		if err != nil {
			ctx.RespErr = fmt.Errorf("failed to parse param ignoreDelete, err: %s", err)
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetProjectCodeHost(id, projectKey, ignoreDelete, ctx.Logger)
}

// @Summary List Available CodeHost
// @Description List Available CodeHost
// @Tags 	project
// @Accept 	json
// @Produce json
// @Param 	name			path		string					true	"project name"
// @Param 	encryptedKey	query		string					true	"encrypted key"
// @Success 200  			{array} 	models.CodeHost
// @Router /api/aslan/project/integration/{name}/codehosts/available [get]
func ListAvailableCodeHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		authorizedProjectList, found, err := internalhandler.ListAuthorizedProjects(ctx.UserID)
		if err != nil {
			ctx.RespErr = e.ErrInternalError.AddDesc(err.Error())
			return
		}
		if !found {
			ctx.RespErr = e.ErrUnauthorized
			return
		}

		authorizedProjectSet := sets.NewString(authorizedProjectList...)
		if !authorizedProjectSet.Has(projectKey) {
			ctx.RespErr = e.ErrUnauthorized
			return
		}
	}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	ctx.Resp, ctx.RespErr = service.GetAvailableCodehostList(projectKey, encryptedKey, ctx.Logger)
}
