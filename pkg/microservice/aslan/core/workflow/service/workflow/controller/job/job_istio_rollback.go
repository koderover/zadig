/*
Copyright 2025 The KodeRover Authors.
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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"k8s.io/apimachinery/pkg/labels"
)

type IstioRollbackJobController struct {
	*BasicInfo

	jobSpec *commonmodels.IstioRollBackJobSpec
}

func CreateIstioRollbackJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.IstioRollBackJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return IstioRollbackJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j IstioRollbackJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j IstioRollbackJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j IstioRollbackJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j IstioRollbackJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.IstioRollBackJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ClusterID = currJobSpec.ClusterID
	j.jobSpec.ClusterSource = currJobSpec.ClusterSource
	j.jobSpec.Namespace = currJobSpec.Namespace
	j.jobSpec.Timeout = currJobSpec.Timeout

	if !useUserInput {
		j.jobSpec.Targets = currJobSpec.Targets
	}
	return nil
}

func (j IstioRollbackJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	targets := make([]*commonmodels.IstioJobTarget, 0)

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(j.jobSpec.ClusterID)
	if err != nil {
		return err
	}

	deployments, err := getter.ListDeployments(j.jobSpec.Namespace, labels.Everything(), kubeClient)
	if err != nil {
		return err
	}
	for _, deployment := range deployments {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			target := &commonmodels.IstioJobTarget{
				WorkloadName:  deployment.Name,
				ContainerName: container.Name,
			}

			zadigNewDeploymentName := fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix)
			_, zadigFound, err := getter.GetDeployment(j.jobSpec.Namespace, zadigNewDeploymentName, kubeClient)
			if err != nil {
				log.Warnf("deployment %s not found in namespace: %s", zadigNewDeploymentName, j.jobSpec.Namespace)
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
				targets = append(targets, target)
			} else {
				target.TargetReplica = int(*deployment.Spec.Replicas)
				for _, container := range deployment.Spec.Template.Spec.Containers {
					if container.Name == target.ContainerName {
						target.Image = container.Image
					}
				}
				targets = append(targets, target)
			}
		}
	}

	j.jobSpec.TargetOptions = targets

	return nil
}

func (j IstioRollbackJobController) ClearOptions() {
	return
}

func (j IstioRollbackJobController) ClearSelection() {
	return
}

func (j IstioRollbackJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)
	cluster, err := commonrepo.NewK8SClusterColl().Get(j.jobSpec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("cluster id: %s not found", j.jobSpec.ClusterID)
	}

	for _, target := range j.jobSpec.Targets {
		jobTask := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, 0),
			Key:         genJobKey(j.name, target.WorkloadName),
			DisplayName: genJobDisplayName(j.name, target.WorkloadName),
			OriginName:  j.name,
			JobType:     string(config.JobIstioRollback),
			JobInfo: map[string]string{
				JobNameKey:      j.name,
				"workload_name": target.WorkloadName,
			},
			Spec: &commonmodels.JobIstioRollbackSpec{
				Namespace:   j.jobSpec.Namespace,
				ClusterID:   j.jobSpec.ClusterID,
				ClusterName: cluster.Name,
				Image:       target.Image,
				Targets:     target,
				Timeout:     j.jobSpec.Timeout,
			},
			ErrorPolicy: j.errorPolicy,
		}
		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j IstioRollbackJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j IstioRollbackJobController) SetRepoCommitInfo() error {
	return nil
}

func (j IstioRollbackJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j IstioRollbackJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j IstioRollbackJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}
