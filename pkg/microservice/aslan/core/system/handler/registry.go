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

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

// @Summary List Registries
// @Description List Registries
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName	query		string										true	"project name"
// @Success 200 		{array} 	commonmodels.RegistryNamespace
// @Router /api/aslan/system/registry/project [get]
func ListRegistries(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if projectName != "" {
			if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if !ctx.Resources.SystemActions.RegistryManagement.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.ListRegistriesByProject(projectName, ctx.Logger)
}

func GetDefaultRegistryNamespace(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	reg, err := commonservice.FindDefaultRegistry(true, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to validate zadig license status, error: %s", err)
		return
	}
	if reg.RegType == config.RegistryProviderACREnterprise ||
		reg.RegType == config.RegistryProviderTCREnterprise ||
		reg.RegType == config.RegistryProviderJFrog {
		if !((licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional ||
			licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise) &&
			licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
			ctx.RespErr = e.ErrLicenseInvalid.AddDesc("")
			return
		}
	}

	// FIXME: a new feature in 1.11 added a tls certificate field for registry, but it is not added in this API temporarily
	//        since it is for Kodespace ONLY
	ctx.Resp = &Registry{
		ID:        reg.ID.Hex(),
		RegAddr:   reg.RegAddr,
		IsDefault: reg.IsDefault,
		Namespace: reg.Namespace,
		AccessKey: reg.AccessKey,
		SecretKey: reg.SecretKey,
	}
}

func GetRegistryNamespace(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.RegistryManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	reg, err := commonservice.FindRegistryById(c.Param("id"), false, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	resp := &Registry{
		ID:        reg.ID.Hex(),
		RegAddr:   reg.RegAddr,
		IsDefault: reg.IsDefault,
		Namespace: reg.Namespace,
	}

	if reg.AdvancedSetting != nil {
		resp.AdvancedSetting = &AdvancedRegistrySetting{
			Modified:   reg.AdvancedSetting.Modified,
			TLSEnabled: reg.AdvancedSetting.TLSEnabled,
			TLSCert:    reg.AdvancedSetting.TLSCert,
		}
	}

	ctx.Resp = resp
}

func ListRegistryNamespaces(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.RegistryManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}
	ctx.Resp, ctx.RespErr = commonservice.ListRegistryNamespaces(encryptedKey, false, ctx.Logger)
}

func CreateRegistryNamespace(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.RegistryNamespace)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateRegistryNamespace c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateRegistryNamespace json.Unmarshal err : %v", err)
	}

	detail := fmt.Sprintf("提供商:%s,Namespace:%s", args.RegProvider, args.Namespace)
	detailEn := fmt.Sprintf("Provider: %s, Namespace: %s", args.RegProvider, args.Namespace)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "资源配置-镜像仓库", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.RegistryManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err := args.LicenseValidate(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.CreateRegistryNamespace(ctx.UserName, args, ctx.Logger)
}

func UpdateRegistryNamespace(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.RegistryNamespace)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateRegistryNamespace c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateRegistryNamespace json.Unmarshal err : %v", err)
	}

	detail := fmt.Sprintf("提供商:%s,Namespace:%s", args.RegProvider, args.Namespace)
	detailEn := fmt.Sprintf("Provider: %s, Namespace: %s", args.RegProvider, args.Namespace)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "资源配置-镜像仓库", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.RegistryManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err := args.LicenseValidate(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateRegistryNamespace(ctx.UserName, c.Param("id"), args, ctx.Logger)
}

// @Summary 验证镜像仓库连接
// @Description
// @Tags 	registry
// @Accept 	json
// @Produce json
// @Param 	registry		body		commonmodels.RegistryNamespace					true	"镜像仓库信息"
// @Success 200
// @Router /api/aslan/system/registry/validate [post]
func ValidateRegistryNamespace(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.RegistryManagement.Create && !ctx.Resources.SystemActions.RegistryManagement.Edit && !ctx.Resources.SystemActions.RegistryManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(commonmodels.RegistryNamespace)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if err := args.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.RespErr = service.ValidateRegistryNamespace(args, ctx.Logger)
}

func DeleteRegistryNamespace(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	detail := fmt.Sprintf("registry ID:%s", c.Param("id"))
	detailEn := fmt.Sprintf("Registry ID: %s", c.Param("id"))
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "资源配置-镜像仓库", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.RegistryManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = service.DeleteRegistryNamespace(c.Param("id"), ctx.Logger)
}

// @Summary Get Registry References
// @Description Get the list of environments that are using the specified registry
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id		path		string										true	"registry id"
// @Success 200 	{object} 	service.RegistryReferencesResp
// @Router /api/aslan/system/registry/namespaces/{id}/references [get]
func GetRegistryReferences(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.RegistryManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetRegistryReferences(c.Param("id"), ctx.Logger)
}

func ListAllRepos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListAllRepos(ctx.Logger)
}

type ListImagesOption struct {
	Names []string `json:"names"`
}

func ListImages(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty")
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
	}

	//判断当前registryId是否为空
	registryID := c.Query("registryId")
	var registryInfo *commonmodels.RegistryNamespace
	if registryID != "" {
		registryInfo, err = commonservice.FindRegistryById(registryID, false, ctx.Logger)
	} else {
		registryInfo, err = commonservice.FindDefaultRegistry(false, ctx.Logger)
	}
	if err != nil {
		ctx.Logger.Errorf("can't find candidate registry err :%v", err)
		ctx.Resp = make([]*service.RepoImgResp, 0)
		return
	}

	authProjectSet := sets.NewString(registryInfo.Projects...)
	if !authProjectSet.Has(setting.AllProjects) && !authProjectSet.Has(projectName) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("project %s is not in the registry %s/%s's project list", projectName, registryInfo.RegAddr, registryInfo.Namespace))
		return
	}

	args := new(ListImagesOption)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	names := args.Names
	images, err := service.ListReposTags(registryInfo, names, ctx.Logger)
	ctx.Resp, ctx.RespErr = images, err
}

func ListRepoImages(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	registryInfo, err := commonservice.FindDefaultRegistry(false, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	name := c.Param("name")

	resp, err := service.GetRepoTags(registryInfo, name, ctx.Logger)
	ctx.Resp, ctx.RespErr = resp, err
}
