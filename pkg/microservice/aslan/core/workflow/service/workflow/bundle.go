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

package workflow

import (
	"go.uber.org/zap"

	commonConfig "github.com/koderover/zadig/pkg/config"
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

func GetBundleResources(logger *zap.SugaredLogger) ([]*resourceSpec, error) {
	var res []*resourceSpec
	workflows, err := mongodb.NewWorkflowColl().List(&mongodb.ListWorkflowOption{})
	if err != nil {
		log.Error("Failed to list workflows , err:%s", err)
		return nil, err
	}

	// get labels by workflow resources ids
	var resources []labeldb.Resource
	for _, workflow := range workflows {
		resource := labeldb.Resource{
			Name:        workflow.Name,
			ProjectName: workflow.ProductTmplName,
			Type:        string(config.ResourceTypeWorkflow),
		}
		resources = append(resources, resource)
	}
	labelsResp, err := service.ListLabelsByResources(resources, logger)
	if err != nil {
		return nil, err
	}

	for _, workflow := range workflows {
		resourceKey := commonConfig.BuildResourceKey(string(config.ResourceTypeWorkflow), workflow.ProductTmplName, workflow.Name)
		resourceSpec := &resourceSpec{
			ResourceID:  workflow.Name,
			ProjectName: workflow.ProductTmplName,
			Spec:        make(map[string]interface{}),
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
