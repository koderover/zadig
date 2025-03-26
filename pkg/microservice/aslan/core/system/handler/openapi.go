/*
Copyright 2022 The KodeRover Authors.

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
	"time"

	"github.com/gin-gonic/gin"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	clusterservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/multicluster/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPICreateRegistry(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPICreateRegistryReq)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("OpenAPICreateRegistry c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateRegistryNamespace json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "新增", "系统设置-Registry", fmt.Sprintf("提供商:%s,Namespace:%s", args.Provider, args.Namespace), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPICreateRegistry(ctx.UserName, args, ctx.Logger)
}

func OpenAPIListRegistry(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	registry, err := service.ListRegistries(ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	resp := make([]*service.OpenAPIRegistry, 0)
	for _, reg := range registry {
		resp = append(resp, &service.OpenAPIRegistry{
			ID:        reg.ID.Hex(),
			Address:   reg.RegAddr,
			Provider:  config.RegistryProvider(reg.RegProvider),
			Region:    reg.Region,
			IsDefault: reg.IsDefault,
			Namespace: reg.Namespace,
		})
	}
	ctx.Resp = resp
}

func OpenAPIGetRegistry(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	registry, err := commonservice.FindRegistryById(c.Param("id"), true, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ret := &service.OpenAPIRegistry{
		ID:        registry.ID.Hex(),
		Address:   registry.RegAddr,
		Provider:  config.RegistryProvider(registry.RegProvider),
		Region:    registry.Region,
		IsDefault: registry.IsDefault,
		Namespace: registry.Namespace,
	}
	ctx.Resp = ret
}

func OpenAPIUpdateRegistry(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	registryInfo, err := commonservice.FindRegistryById(c.Param("id"), true, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	args := new(service.OpenAPIRegistry)

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	registryInfo.RegAddr = args.Address
	registryInfo.Namespace = args.Namespace
	registryInfo.RegProvider = string(args.Provider)
	registryInfo.Region = args.Region
	registryInfo.IsDefault = args.IsDefault

	if err := registryInfo.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err := registryInfo.LicenseValidate(); err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "更新", "资源配置-镜像仓库", c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.UpdateRegistryNamespace(ctx.UserName, c.Param("id"), registryInfo, ctx.Logger)
}

func OpenAPIListCluster(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.OpenAPIListCluster(c.Query("projectName"), ctx.Logger)
}

// @Summary OpenAPI Create Cluster
// @Description OpenAPI Create Cluster
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	body 		body 		service.OpenAPICreateClusterRequest 	true 	"body"
// @Success 200 		{object} 	service.OpenAPICreateClusterResponse
// @Router /openapi/system/cluster [post]
func OpenAPICreateCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPICreateClusterRequest)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "创建", "资源配置-集群", req.Name, "", types.RequestBodyTypeJSON, ctx.Logger)

	clusterAccessYaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: koderover-agent-admin
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - '*'
`

	AdvancedConfig := &clusterservice.AdvancedConfig{
		ClusterAccessYaml: clusterAccessYaml,
		ProjectNames:      req.ProjectNames,
		ScheduleWorkflow:  true,
		ScheduleStrategy: []*clusterservice.ScheduleStrategy{
			{
				NodeLabels:   []string{},
				StrategyName: "normal",
				Strategy:     "normal",
				Default:      true,
			},
		},
	}
	dindCfg := &models.DindCfg{
		Replicas: 1,
		Resources: &models.Resources{
			Limits: &models.Limits{
				CPU:    4000,
				Memory: 8192,
			},
		},
		Storage: &models.DindStorage{
			StorageSizeInGiB: 10,
			Type:             models.DindStorageRootfs,
		},
	}
	shareStorage := types.ShareStorage{
		MediumType: types.NFSMedium,
		NFSProperties: types.NFSProperties{
			PVC:              "cache-cfs-10",
			StorageClass:     "cfs",
			StorageSizeInGiB: 10,
		},
	}

	cluster := &clusterservice.K8SCluster{
		Name:           req.Name,
		Type:           req.Type,
		Provider:       req.Provider,
		KubeConfig:     req.KubeConfig,
		Description:    req.Description,
		Production:     req.Production,
		AdvancedConfig: AdvancedConfig,
		DindCfg:        dindCfg,
		ShareStorage:   shareStorage,
		CreatedAt:      time.Now().Unix(),
		CreatedBy:      ctx.UserName + "(openAPI)",
	}

	if err := cluster.Clean(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	err = cluster.Validate()
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to validate cluster: %v", err)
		return
	}

	clusterResp, err := clusterservice.CreateCluster(cluster, ctx.Logger)
	if err != nil {
		ctx.RespErr = fmt.Errorf("Failed to create cluster: %v", err)
		return
	}

	for _, projectName := range req.ProjectNames {
		err = commonrepo.NewProjectClusterRelationColl().Create(&models.ProjectClusterRelation{
			ProjectName: projectName,
			ClusterID:   clusterResp.ID.Hex(),
			CreatedBy:   ctx.UserName + "(openAPI)",
		})
		if err != nil {
			log.Errorf("Failed to create projectClusterRelation err:%s", err)
		}
	}

	agentCmd := fmt.Sprintf(`kubectl apply -f "%s/api/aslan/cluster/agent/%s/agent.yaml?type=deploy"`, configbase.SystemAddress(), clusterResp.ID.Hex())

	resp := service.OpenAPICreateClusterResponse{
		Cluster: &service.OpenAPICluster{
			ID:           clusterResp.ID.Hex(),
			Name:         clusterResp.Name,
			Type:         clusterResp.Type,
			ProviderName: service.ClusterProviderValueNames[clusterResp.Provider],
			Production:   clusterResp.Production,
			Description:  clusterResp.Description,
			ProjectNames: clusterservice.GetProjectNames(clusterResp.ID.Hex(), log.SugaredLogger()),
			Local:        clusterResp.Local,
			Status:       string(clusterResp.Status),
			CreatedBy:    clusterResp.CreatedBy,
			CreatedTime:  clusterResp.CreatedAt,
		},
		AgentCmd: agentCmd,
	}

	ctx.Resp = resp
}

func OpenAPIUpdateCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPICluster)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		log.Errorf("Failed to bind data: %s", err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "更新", "资源配置-集群", c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = service.OpenAPIUpdateCluster(ctx.UserName, c.Param("id"), args, ctx.Logger)
}

func OpenAPIDeleteCluster(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "删除", "资源配置-集群", c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.OpenAPIDeleteCluster(ctx.UserName, c.Param("id"), ctx.Logger)
}
