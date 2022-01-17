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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
)

type resourceSpec struct {
	ResourceID  string                 `json:"resourceID"`
	ProjectName string                 `json:"projectName"`
	Spec        map[string]interface{} `json:"spec"`
}

func GetBundleResources() ([]*resourceSpec, error) {
	var res []*resourceSpec
	workflows, err := mongodb.NewWorkflowColl().List(nil)
	if err != nil {
		log.Error("Failed to list workflows , err:%s", err)
		return nil, err
	}
	// get labels by workflow resources ids
	// TODO - mouuii
	for _, workflow := range workflows {
		res = append(res, &resourceSpec{
			ResourceID:  workflow.ID.Hex(),
			ProjectName: workflow.ProductTmplName,
			Spec:        map[string]interface{}{},
		})
	}
	return res, nil
}
