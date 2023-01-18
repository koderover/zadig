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

package service

import (
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
)

type Overview struct {
	ProjectCount  int `json:"project_count"`
	ClusterCount  int `json:"cluster_count"`
	ServiceCount  int `json:"service_count"`
	WorkflowCount int `json:"workflow_count"`
	EnvCount      int `json:"env_count"`
	ArtifactCount int `json:"artifact_count"`
}

func GetOverviewStat(log *zap.SugaredLogger) (*Overview, error) {
	overview := new(Overview)
	projectCount, err := templaterepo.NewProductColl().Count()
	if err != nil {
		log.Errorf("Failed to get project count err:%s", err)
		return nil, err
	}
	overview.ProjectCount = int(projectCount)

	clusterCount, err := commonrepo.NewK8SClusterColl().Count()
	if err != nil {
		log.Errorf("Failed to get cluster count err:%s", err)
		return nil, err
	}
	overview.ClusterCount = int(clusterCount)

	workflowCount, err := commonrepo.NewWorkflowColl().Count()
	if err != nil {
		log.Errorf("Failed to get workflow count err:%s", err)
		return nil, err
	}
	workflowV4Count, err := commonrepo.NewWorkflowV4Coll().Count()
	if err != nil {
		log.Errorf("Failed to get workflow v4 count err:%s", err)
		return nil, err
	}
	overview.WorkflowCount = int(workflowCount + workflowV4Count)

	envCount, err := commonrepo.NewProductColl().EnvCount()
	if err != nil {
		log.Errorf("Failed to get env count err:%s", err)
		return nil, err
	}
	overview.EnvCount = int(envCount)

	_, artifactCount, err := commonrepo.NewDeliveryArtifactColl().List(&commonrepo.DeliveryArtifactArgs{OnlyCount: true})
	if err != nil {
		log.Errorf("Failed to get artifact count err:%s", err)
		return nil, err
	}
	overview.ArtifactCount = artifactCount

	serviceCount, err := commonrepo.NewServiceColl().Count("")
	if err != nil {
		log.Errorf("Failed to get service count err:%s", err)
		return nil, err
	}
	overview.ServiceCount = int(serviceCount)

	return overview, nil
}
