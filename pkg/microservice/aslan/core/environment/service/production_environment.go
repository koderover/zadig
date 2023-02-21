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

package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collaboration"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/util"
)

func ListProductionEnvs(projectName string, envNames []string, log *zap.SugaredLogger) ([]*EnvResp, error) {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:                projectName,
		InEnvs:              envNames,
		IsSortByProductName: true,
		Production:          util.GetBoolPointer(true),
	})
	if err != nil {
		log.Errorf("Failed to list production envs, err: %s", err)
		return nil, e.ErrListEnvs.AddDesc(err.Error())
	}

	var res []*EnvResp
	reg, _, err := commonservice.FindDefaultRegistry(false, log)
	if err != nil {
		log.Errorf("FindDefaultRegistry error: %v", err)
		return nil, e.ErrListEnvs.AddErr(err)
	}

	clusters, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{})
	if err != nil {
		log.Errorf("failed to list clusters, err: %s", err)
		return nil, e.ErrListEnvs.AddErr(err)
	}
	clusterMap := make(map[string]*models.K8SCluster)
	for _, cluster := range clusters {
		clusterMap[cluster.ID.Hex()] = cluster
	}
	getClusterName := func(clusterID string) string {
		cluster, ok := clusterMap[clusterID]
		if ok {
			return cluster.Name
		}
		return ""
	}

	envCMMap, err := collaboration.GetEnvCMMap([]string{projectName}, log)
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		if len(env.RegistryID) == 0 {
			env.RegistryID = reg.ID.Hex()
		}

		var baseRefs []string
		if cmSet, ok := envCMMap[collaboration.BuildEnvCMMapKey(env.ProductName, env.EnvName)]; ok {
			baseRefs = append(baseRefs, cmSet.List()...)
		}
		res = append(res, &EnvResp{
			ProjectName:     projectName,
			Name:            env.EnvName,
			IsPublic:        env.IsPublic,
			IsExisted:       env.IsExisted,
			ClusterName:     getClusterName(env.ClusterID),
			Source:          env.Source,
			Production:      env.Production,
			Status:          env.Status,
			Error:           env.Error,
			UpdateTime:      env.UpdateTime,
			UpdateBy:        env.UpdateBy,
			RegistryID:      env.RegistryID,
			ClusterID:       env.ClusterID,
			Namespace:       env.Namespace,
			Alias:           env.Alias,
			BaseRefs:        baseRefs,
			BaseName:        env.BaseName,
			ShareEnvEnable:  env.ShareEnv.Enable,
			ShareEnvIsBase:  env.ShareEnv.IsBase,
			ShareEnvBaseEnv: env.ShareEnv.BaseEnv,
		})
	}

	return res, nil
}
