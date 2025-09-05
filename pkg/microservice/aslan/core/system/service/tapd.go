/*
 * Copyright 2025 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/tapd"
)

func ListTapdProjects(id string) ([]*tapd.Workspace, error) {
	spec, err := mongodb.NewProjectManagementColl().GetTapdSpec(id)
	if err != nil {
		return nil, err
	}

	client, err := tapd.NewClient(spec.TapdAddress, spec.TapdClientID, spec.TapdClientSecret, spec.TapdCompanyID)
	if err != nil {
		return nil, err
	}

	projectList, err := client.ListProjects()
	if err != nil {
		return nil, err
	}

	return projectList, nil
}

func ListTapdIterations(id, projectID, status string) ([]*tapd.Iteration, error) {
	spec, err := mongodb.NewProjectManagementColl().GetTapdSpec(id)
	if err != nil {
		return nil, err
	}

	client, err := tapd.NewClient(spec.TapdAddress, spec.TapdClientID, spec.TapdClientSecret, spec.TapdCompanyID)
	if err != nil {
		return nil, err
	}

	iterations, err := client.ListIterations(projectID, status)
	if err != nil {
		return nil, err
	}

	return iterations, nil
}
