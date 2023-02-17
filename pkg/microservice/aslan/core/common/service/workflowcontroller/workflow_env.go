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

package workflowcontroller

import "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"

type ProductServiceDeployInfo struct {
	ProductName     string
	EnvName         string
	ServiceName     string
	Uninstall       bool
	ServiceRevision int
	VariableYaml    string
	Containers      []*models.Container
}

// UpdateProductServiceDeployInfo updates deploy info of service for some product
// Including: Deploy service / Update service / Uninstall services
func UpdateProductServiceDeployInfo(deployInfo *ProductServiceDeployInfo) error {
	return nil
}
