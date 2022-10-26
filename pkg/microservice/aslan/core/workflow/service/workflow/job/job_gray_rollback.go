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

package job

import (
	"fmt"
	"strconv"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/log"
)

type GrayRollbackJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.GrayRollbackJobSpec
}

func (j *GrayRollbackJob) Instantiate() error {
	j.spec = &commonmodels.GrayRollbackJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *GrayRollbackJob) SetPreset() error {
	j.spec = &commonmodels.GrayRollbackJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), j.spec.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client, err: %v", err)
	}
	newTargets := []*commonmodels.GrayRollbackTarget{}
	for _, target := range j.spec.Targets {
		deployment, found, err := getter.GetDeployment(j.spec.Namespace, target.WorkloadName, kubeClient)
		if err != nil || !found {
			log.Errorf("deployment %s not found in namespace: %s", target.WorkloadName, j.spec.Namespace)
			continue
		}
		rollbackInfo, err := getGrayRollbackInfoFromAnnotations(deployment.GetAnnotations())
		if err != nil {
			log.Errorf("deployment %s get gray rollback info failed: %v", target.WorkloadName, err)
			continue
		}
		target.OriginImage = rollbackInfo.image
		target.OriginReplica = rollbackInfo.replica
		newTargets = append(newTargets, target)
	}
	j.spec.Targets = newTargets
	j.job.Spec = j.spec
	return nil
}

func (j *GrayRollbackJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.GrayRollbackJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.GrayRollbackJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.Targets = argsSpec.Targets
		j.job.Spec = j.spec
	}
	return nil
}

func (j *GrayRollbackJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	// logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.GrayRollbackJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	cluster, err := commonrepo.NewK8SClusterColl().Get(j.spec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("cluster id: %s not found", j.spec.ClusterID)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), j.spec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("Failed to get kube client, err: %v", err)
	}
	for _, target := range j.spec.Targets {
		deployment, found, err := getter.GetDeployment(j.spec.Namespace, target.WorkloadName, kubeClient)
		if err != nil || !found {
			return resp, fmt.Errorf("deployment %s not found in namespace: %s", target.WorkloadName, j.spec.Namespace)
		}
		rollbackInfo, err := getGrayRollbackInfoFromAnnotations(deployment.GetAnnotations())
		if err != nil {
			return resp, fmt.Errorf("deployment %s get gray rollback info failed: %v", target.WorkloadName, err)
		}
		jobTask := &commonmodels.JobTask{
			Name:    j.job.Name,
			JobType: string(config.JobK8sGrayRollback),
			Spec: &commonmodels.JobTaskGrayRollbackSpec{
				ClusterID:        j.spec.ClusterID,
				ClusterName:      cluster.Name,
				Namespace:        j.spec.Namespace,
				WorkloadType:     target.WorkloadType,
				WorkloadName:     target.WorkloadName,
				ContainerName:    rollbackInfo.containerName,
				GrayWorkloadName: target.WorkloadName + config.GrayDeploymentSuffix,
				Image:            rollbackInfo.image,
				RollbackTimeout:  j.spec.RollbackTimeout,
				TotalReplica:     rollbackInfo.replica,
			},
		}
		resp = append(resp, jobTask)
	}
	j.job.Spec = j.spec
	return resp, nil
}

type grayRollbackInfo struct {
	image         string
	replica       int
	containerName string
}

func getGrayRollbackInfoFromAnnotations(annotations map[string]string) (*grayRollbackInfo, error) {
	image, ok := annotations[config.GrayImageAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("deployment annotations has no zadig gray image info")
	}
	containerName, ok := annotations[config.GrayContainerAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("deployment annotations has no zadig gray container info")
	}
	replicaStr, ok := annotations[config.GrayReplicaAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("deployment annotations has no zadig gray replica info")
	}
	replica, err := strconv.Atoi(replicaStr)
	if err != nil {
		return nil, fmt.Errorf("annotation replica: %s is not a number", replicaStr)
	}

	return &grayRollbackInfo{
		image:         image,
		replica:       replica,
		containerName: containerName,
	}, nil
}

func (j *GrayRollbackJob) LintJob() error {
	// TODO
	return nil
}
