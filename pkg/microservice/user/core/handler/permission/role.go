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

package permission

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	userhandler "github.com/koderover/zadig/v2/pkg/microservice/user/core/handler/user"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func checkLicense(actions []string) error {
	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !((licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional ||
		licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise) &&
		licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
		actionSet := sets.NewString(actions...)
		if actionSet.Has(permission.VerbCreateReleasePlan) || actionSet.Has(permission.VerbDeleteReleasePlan) ||
			actionSet.Has(permission.VerbEditReleasePlanMetadata) || actionSet.Has(permission.VerbEditReleasePlanApproval) ||
			actionSet.Has(permission.VerbEditReleasePlanSubtasks) || actionSet.Has(permission.VerbGetReleasePlan) ||
			actionSet.Has(permission.VerbEditConfigReleasePlan) ||
			actionSet.Has(permission.VerbEditDataCenterInsightConfig) ||
			actionSet.Has(permission.VerbGetProductionService) || actionSet.Has(permission.VerbGetProductionService) ||
			actionSet.Has(permission.VerbGetProductionService) || actionSet.Has(permission.VerbGetProductionService) ||
			actionSet.Has(permission.VerbGetProductionEnv) || actionSet.Has(permission.VerbCreateProductionEnv) ||
			actionSet.Has(permission.VerbConfigProductionEnv) || actionSet.Has(permission.VerbEditProductionEnv) ||
			actionSet.Has(permission.VerbDeleteProductionEnv) || actionSet.Has(permission.VerbDebugProductionEnvPod) ||
			actionSet.Has(permission.VerbGetDelivery) || actionSet.Has(permission.VerbCreateDelivery) || actionSet.Has(permission.VerbDeleteDelivery) {
			return e.ErrLicenseInvalid.AddDesc("")
		}
	}
	return nil
}

func OpenAPICreateRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	CreateRoleImpl(c, ctx)
}

func CreateRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	CreateRoleImpl(c, ctx)
}

func CreateRoleTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateRole c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &permission.CreateRoleReq{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	detail := "角色名称：" + args.Name
	detailEn := "Role Name: " + args.Name
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, "*", setting.OperationSceneProject, "创建", "全局角色", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	err = checkLicense(args.Actions)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = permission.CreateRoleTemplate(args, ctx.Logger)
}

func CreateRoleImpl(c *gin.Context, ctx *internalhandler.Context) {
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateRole c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &permission.CreateRoleReq{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	err = userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}

	detail := "角色名称：" + args.Name
	detailEn := "Role Name: " + args.Name
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "创建", "角色", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	err = checkLicense(args.Actions)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = permission.CreateRole(projectName, args, ctx.Logger)
}

func OpenAPIUpdateRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	UpdateRoleImpl(c, ctx)
}

func UpdateRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	UpdateRoleImpl(c, ctx)
}

func UpdateRoleImpl(c *gin.Context, ctx *internalhandler.Context) {

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateRole c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &permission.CreateRoleReq{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	err = userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}
	name := c.Param("name")
	args.Name = name

	detail := "角色名称：" + args.Name
	detailEn := "Role Name: " + args.Name
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "更新", "角色", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	//licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	//if err != nil {
	//	ctx.RespErr = fmt.Errorf("failed to validate zadig license status, error: %s", err)
	//	return
	//}
	//if !((licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional ||
	//	licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise) &&
	//	licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
	//	actionSet := sets.NewString(args.Actions...)
	//	if actionSet.Has(permission.VerbCreateReleasePlan) || actionSet.Has(permission.VerbDeleteReleasePlan) ||
	//		actionSet.Has(permission.VerbEditReleasePlan) || actionSet.Has(permission.VerbGetReleasePlan) ||
	//		actionSet.Has(permission.VerbEditDataCenterInsightConfig) ||
	//		actionSet.Has(permission.VerbGetProductionService) || actionSet.Has(permission.VerbCreateProductionService) ||
	//		actionSet.Has(permission.VerbEditProductionService) || actionSet.Has(permission.VerbDeleteProductionService) ||
	//		actionSet.Has(permission.VerbGetProductionEnv) || actionSet.Has(permission.VerbCreateProductionEnv) ||
	//		actionSet.Has(permission.VerbConfigProductionEnv) || actionSet.Has(permission.VerbEditProductionEnv) ||
	//		actionSet.Has(permission.VerbDeleteProductionEnv) || actionSet.Has(permission.VerbDebugProductionEnvPod) ||
	//		actionSet.Has(permission.VerbGetDelivery) || actionSet.Has(permission.VerbCreateDelivery) || actionSet.Has(permission.VerbDeleteDelivery) {
	//		ctx.RespErr = e.ErrLicenseInvalid.AddDesc("")
	//		return
	//	}
	//}

	ctx.RespErr = permission.UpdateRole(projectName, args, ctx.Logger)
}

func UpdateRoleTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateRole c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &permission.CreateRoleReq{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	name := c.Param("name")
	args.Name = name

	detail := fmt.Sprintf("角色名称：%s", args.Name)
	detailEn := fmt.Sprintf("Role Name: %s", args.Name)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, "*", setting.OperationSceneProject, "更新", "全局角色", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	err = checkLicense(args.Actions)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = permission.UpdateRoleTemplate(args, ctx.Logger)
}

func OpenAPIListRoles(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	err := userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}

		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	uid := c.Query("uid")
	if uid == "" {
		ctx.Resp, ctx.RespErr = permission.ListRolesByNamespace(projectName, ctx.Logger)
	} else {
		ctx.Resp, ctx.RespErr = permission.ListRolesByNamespaceAndUserID(projectName, uid, ctx.Logger)
	}
}

func ListRoles(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}
	uid := c.Query("uid")
	if uid == "" {
		ctx.Resp, ctx.RespErr = permission.ListRolesByNamespace(projectName, ctx.Logger)
	} else {
		ctx.Resp, ctx.RespErr = permission.ListRolesByNamespaceAndUserID(projectName, uid, ctx.Logger)
	}
}

func ListRoleTemplates(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = permission.ListRoleTemplates(ctx.Logger)
}

func OpenAPIGetRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	err := userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}

		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = permission.GetRole(projectName, c.Param("name"), ctx.Logger)
}

func GetRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}

	ctx.Resp, ctx.RespErr = permission.GetRole(projectName, c.Param("name"), ctx.Logger)
}

func GetRoleTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = permission.GetRoleTemplate(c.Param("name"), ctx.Logger)
}

func OpenAPIDeleteRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	DeleteRoleImpl(c, ctx)
}

func DeleteRoleTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	name := c.Param("name")
	detail := fmt.Sprintf("角色名称：%s", name)
	detailEn := fmt.Sprintf("Role Name: %s", name)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, "*", setting.OperationSceneProject, "删除", "角色", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, name)

	ctx.RespErr = permission.DeleteRoleTemplate(c.Param("name"), ctx.Logger)
}

func DeleteRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	DeleteRoleImpl(c, ctx)
}

func DeleteRoleImpl(c *gin.Context, ctx *internalhandler.Context) {

	name := c.Param("name")
	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}

	err := userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	detail := fmt.Sprintf("角色名称：%s", name)
	detailEn := fmt.Sprintf("Role Name: %s", name)
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "删除", "角色", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, name)

	ctx.RespErr = permission.DeleteRole(name, projectName, ctx.Logger)
}
