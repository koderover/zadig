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

package job

import (
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/releaseutil"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/types"
)

type MseGrayReleaseJobController struct {
	*BasicInfo

	jobSpec *commonmodels.MseGrayReleaseJobSpec
}

func CreateMseGrayReleaseJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.MseGrayReleaseJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return MseGrayReleaseJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j MseGrayReleaseJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j MseGrayReleaseJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j MseGrayReleaseJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigEnterpriseLicense(); err != nil {
		return err
	}

	if j.jobSpec.GrayTag == types.ZadigReleaseVersionOriginal {
		return fmt.Errorf("gray tag must not be 'original'")
	}
	if j.jobSpec.GrayTag == "" {
		return fmt.Errorf("gray tag must not be empty")
	}

	return nil
}

func (j MseGrayReleaseJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.MseGrayReleaseJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.Production = currJobSpec.Production
	j.jobSpec.BaseEnv = currJobSpec.BaseEnv
	if currJobSpec.GrayEnvSource == "fixed" {
		j.jobSpec.GrayEnv = currJobSpec.GrayEnv
	}
	j.jobSpec.GrayEnvSource = currJobSpec.GrayEnvSource
	j.jobSpec.DockerRegistryID = currJobSpec.DockerRegistryID
	j.jobSpec.SkipCheckRunStatus = currJobSpec.SkipCheckRunStatus
	return nil
}

func (j MseGrayReleaseJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j MseGrayReleaseJobController) ClearOptions() {
	return
}

func (j MseGrayReleaseJobController) ClearSelection() {
	j.jobSpec.GrayServices = make([]*commonmodels.MseGrayReleaseService, 0)
}

func (j MseGrayReleaseJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)
	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	for _, service := range j.jobSpec.GrayServices {
		resources := make([]*unstructured.Unstructured, 0)
		manifests := releaseutil.SplitManifests(service.YamlContent)
		for _, item := range manifests {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
			if err != nil {
				return nil, fmt.Errorf("failed to decode service %s yaml to unstructured: %v", service.ServiceName, err)
			}
			resources = append(resources, u)
		}
		deploymentNum := 0
		for _, resource := range resources {
			switch resource.GetKind() {
			case setting.Deployment:
				if deploymentNum > 0 {
					return nil, fmt.Errorf("service-%s: only one deployment is allowed in each service", service.ServiceName)
				}
				deploymentNum++

				deploymentObj := &v1.Deployment{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deploymentObj)
				if err != nil {
					return nil, fmt.Errorf("failed to convert service %s deployment to deployment object: %v", service.ServiceName, err)
				}
				if deploymentObj.Spec.Selector == nil || deploymentObj.Spec.Selector.MatchLabels == nil {
					return nil, fmt.Errorf("service %s deployment selector is nil", service.ServiceName)
				}
				if exist, key := checkMapKeyExist(deploymentObj.Spec.Selector.MatchLabels,
					types.ZadigReleaseVersionLabelKey, types.ZadigReleaseServiceNameLabelKey,
					types.ZadigReleaseMSEGrayTagLabelKey, types.ZadigReleaseTypeLabelKey); !exist {
					return nil, fmt.Errorf("service %s deployment label selector must contain %s", service.ServiceName, key)
				}
				if exist, key := checkMapKeyExist(deploymentObj.Spec.Template.Labels,
					types.ZadigReleaseVersionLabelKey, types.ZadigReleaseServiceNameLabelKey,
					types.ZadigReleaseMSEGrayTagLabelKey, types.ZadigReleaseTypeLabelKey); !exist {
					return nil, fmt.Errorf("service %s deployment template label must contain %s", service.ServiceName, key)
				}
			case setting.ConfigMap, setting.Secret, setting.Service, setting.Ingress:
			default:
				return nil, fmt.Errorf("service %s resource type %s not allowed", service.ServiceName, resource.GetKind())
			}
		}
		if deploymentNum == 0 {
			return nil, fmt.Errorf("service-%s: each service must contain one deployment", service.ServiceName)
		}
		resp = append(resp, &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, 0),
			Key:         genJobKey(j.name, service.ServiceName),
			DisplayName: genJobDisplayName(j.name, service.ServiceName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:     j.name,
				"service_name": service.ServiceName,
			},
			JobType: string(config.JobMseGrayRelease),
			Spec: commonmodels.JobTaskMseGrayReleaseSpec{
				GrayTag:            j.jobSpec.GrayTag,
				BaseEnv:            j.jobSpec.BaseEnv,
				GrayEnv:            j.jobSpec.GrayEnv,
				SkipCheckRunStatus: j.jobSpec.SkipCheckRunStatus,
				GrayService:        *service,
				Timeout:            timeout,
				Production:         j.jobSpec.Production,
			},
			ErrorPolicy:   j.errorPolicy,
			ExecutePolicy: j.executePolicy,
		})
	}

	return resp, nil
}

func (j MseGrayReleaseJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j MseGrayReleaseJobController) SetRepoCommitInfo() error {
	return nil
}

func (j MseGrayReleaseJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}
	return resp, nil
}

func (j MseGrayReleaseJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j MseGrayReleaseJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j MseGrayReleaseJobController) IsServiceTypeJob() bool {
	return false
}

func checkMapKeyExist(m map[string]string, keys ...string) (bool, string) {
	if m == nil {
		return false, ""
	}
	for _, key := range keys {
		_, ok := m[key]
		if !ok {
			return false, key
		}
	}
	return true, ""
}
