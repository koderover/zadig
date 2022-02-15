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
	"go.uber.org/zap"

	config2 "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	labeldb "github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
)

type resourceSpec struct {
	ResourceID  string                 `json:"resourceID"`
	ProjectName string                 `json:"projectName"`
	Spec        map[string]interface{} `json:"spec"`
}

func GetBundleResources(logger *zap.SugaredLogger) ([]*resourceSpec, error) {
	var res []*resourceSpec

	envs, err := mongodb.NewProductColl().List(nil)
	if err != nil {
		logger.Errorf("Failed to list envs, err: %s", err)
		return nil, err
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
	labelsResp, err := service.ListLabelsByResources(resources, logger)
	if err != nil {
		logger.Errorf("ListLabelsByResources err:%s", err)
		return nil, err
	}

	for _, env := range envs {
		resourceKey := config2.BuildResourceKey(string(config.ResourceTypeProduct), env.ProductName, env.EnvName)
		resourceSpec := &resourceSpec{
			ResourceID:  env.EnvName,
			ProjectName: env.ProductName,
			Spec:        map[string]interface{}{},
		}
		if labels, ok := labelsResp.Labels[resourceKey]; ok {
			for _, v := range labels {
				resourceSpec.Spec[v.Key] = v.Value
			}
		} else {
			logger.Warnf("can not find resource key :%s", resourceKey)
		}
		res = append(res, resourceSpec)
	}

	return res, nil
}
