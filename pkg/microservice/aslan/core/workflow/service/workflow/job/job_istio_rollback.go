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

type IstioRollBackJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.IstioRollBackJobSpec
}

func (j *IstioRollBackJob) Instantiate() error {
	j.spec = &commonmodels.IstioRollBackJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *IstioRollBackJob) SetPreset() error {
	j.spec = &commonmodels.IstioRollBackJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), j.spec.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client, err: %v", err)
	}
	newTargets := make([]*commonmodels.IstioJobTarget, 0)
	for _, target := range j.spec.Targets {
		deployment, found, err := getter.GetDeployment(j.spec.Namespace, target.WorkloadName, kubeClient)
		if err != nil || !found {
			log.Errorf("deployment %s not found in namespace: %s", target.WorkloadName, j.spec.Namespace)
			continue
		}
		zadigNewDeploymentName := fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix)
		_, zadigFound, err := getter.GetDeployment(j.spec.Namespace, zadigNewDeploymentName, kubeClient)
		if err != nil {
			log.Errorf("deployment %s not found in namespace: %s", zadigNewDeploymentName, j.spec.Namespace)
			continue
		}

		if !zadigFound {
			if _, ok := deployment.Annotations[config.ZadigLastAppliedImage]; !ok {
				if _, ok := deployment.Annotations[config.ZadigLastAppliedReplicas]; !ok {
					// if no annotation and no new deployment was found, it cannot be selected
					continue
				}
			}
			target.Image = deployment.Annotations[config.ZadigLastAppliedImage]
			replicas, err := strconv.Atoi(deployment.Annotations[config.ZadigLastAppliedReplicas])
			if err != nil {
				log.Errorf("failed to get the replicas from annotation")
			}
			target.TargetReplica = replicas
			newTargets = append(newTargets, target)
		} else {
			target.TargetReplica = int(*deployment.Spec.Replicas)
			for _, container := range deployment.Spec.Template.Spec.Containers {
				if container.Name == target.ContainerName {
					target.Image = container.Image
				}
			}
			newTargets = append(newTargets, target)
		}

	}
	j.spec.Targets = newTargets
	j.job.Spec = j.spec
	return nil
}

func (j *IstioRollBackJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.IstioRollBackJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.IstioRollBackJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.job.Spec = j.spec
	}
	return nil
}

func (j *IstioRollBackJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.IstioRollBackJobSpec{}

	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	cluster, err := commonrepo.NewK8SClusterColl().Get(j.spec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("cluster id: %s not found", j.spec.ClusterID)
	}

	for _, target := range j.spec.Targets {
		jobTask := &commonmodels.JobTask{
			Name:    jobNameFormat(j.job.Name + "-" + target.WorkloadName),
			JobType: string(config.JobIstioRollback),
			Spec: &commonmodels.JobIstioRollbackSpec{
				Namespace:   j.spec.Namespace,
				ClusterID:   j.spec.ClusterID,
				ClusterName: cluster.Name,
				Image:       target.Image,
				Targets:     target,
				Timeout:     j.spec.Timeout,
			},
		}
		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j *IstioRollBackJob) LintJob() error {
	return nil
}
