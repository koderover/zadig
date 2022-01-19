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

package service

import (
	"fmt"
	"strconv"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	labeldb "github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	"github.com/koderover/zadig/pkg/tool/log"
)

type resourceSpec struct {
	ResourceID  string                 `json:"resourceID"`
	ProjectName string                 `json:"projectName"`
	Spec        map[string]interface{} `json:"spec"`
}

func GetBundleResources() ([]*resourceSpec, error) {
	var res []*resourceSpec

	envs, err := mongodb.NewProductColl().List(nil)
	if err != nil {
		log.Errorf("Failed to list envs, err: %s", err)
		return nil, err
	}

	clusterMap := make(map[string]*models.K8SCluster)
	clusters, err := mongodb.NewK8SClusterColl().List(nil)
	if err != nil {
		log.Errorf("Failed to list clusters in db, err: %s", err)
		return nil, err
	}

	for _, cls := range clusters {
		clusterMap[cls.ID.Hex()] = cls
	}

	// get labels by workflow resources ids
	var resources []labeldb.Resource
	for _, env := range envs {
		resource := labeldb.Resource{
			Name:        env.EnvName,
			ProjectName: env.ProductName,
			Type:        string(config.ResourceTypeProduct),
		}
		resources = append(resources, resource)
	}
	labelsResp, err := service.ListLabelsByResources(resources, nil)
	if err != nil {
		return nil, err
	}

	for _, env := range envs {
		clusterID := env.ClusterID
		production := false
		cluster, ok := clusterMap[clusterID]
		if ok {
			production = cluster.Production
		}

		resourceKey := fmt.Sprintf("%s-%s-%s", config.ResourceTypeProduct, env.ProductName, env.EnvName)
		resourceSpec := &resourceSpec{
			ResourceID:  env.EnvName,
			ProjectName: env.ProductName,
			Spec: map[string]interface{}{
				"production": strconv.FormatBool(production),
			},
		}
		if labels, ok := labelsResp.Labels[resourceKey]; ok {
			for _, v := range labels {
				resourceSpec.Spec[v.Key] = v.Value
			}
		}
		res = append(res, resourceSpec)
	}

	return res, nil
}
