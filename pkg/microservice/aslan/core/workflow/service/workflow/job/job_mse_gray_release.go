/*
 * Copyright 2023 The KodeRover Authors.
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

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/releaseutil"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/types"
)

type MseGrayReleaseJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.MseGrayReleaseJobSpec
}

func (j *MseGrayReleaseJob) Instantiate() error {
	j.spec = &commonmodels.MseGrayReleaseJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *MseGrayReleaseJob) SetPreset() error {
	j.spec = &commonmodels.MseGrayReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	j.job.Spec = j.spec
	return nil
}

func (j *MseGrayReleaseJob) MergeArgs(args *commonmodels.Job) error {
	j.spec = &commonmodels.MseGrayReleaseJobSpec{}
	if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *MseGrayReleaseJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.MseGrayReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec
	if j.spec.GrayTag == types.ZadigReleaseVersionOriginal {
		return nil, errors.Errorf("gray tag must not be 'original'")
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	resources := make([]*unstructured.Unstructured, 0)
	for _, service := range j.spec.GrayServices {
		manifests := releaseutil.SplitManifests(service.YamlContent)
		for _, item := range manifests {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
			if err != nil {
				return nil, errors.Errorf("failed to decode service %s yaml to unstructured: %v", service.ServiceName, err)
			}
			resources = append(resources, u)
		}
		deploymentNum := 0
		for _, resource := range resources {
			switch resource.GetKind() {
			case setting.Deployment:
				if deploymentNum > 0 {
					return nil, errors.Errorf("service-%s: only one deployment is allowed in each service", service.ServiceName)
				}
				deploymentNum++

				deploymentObj := &v1.Deployment{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deploymentObj)
				if err != nil {
					return nil, errors.Errorf("failed to convert service %s deployment to deployment object: %v", service.ServiceName, err)
				}
				if deploymentObj.Spec.Selector == nil || deploymentObj.Spec.Selector.MatchLabels == nil {
					return nil, errors.Errorf("service %s deployment selector is nil", service.ServiceName)
				}
				if !checkMapKeyExist(deploymentObj.Spec.Selector.MatchLabels, types.ZadigReleaseVersionLabelKey) {
					return nil, errors.Errorf("service %s deployment label selector must contain %s", service.ServiceName, types.ZadigReleaseVersionLabelKey)
				}
				if !checkMapKeyExist(deploymentObj.Spec.Template.Labels, types.ZadigReleaseVersionLabelKey) {
					return nil, errors.Errorf("service %s deployment template label must contain %s", service.ServiceName, types.ZadigReleaseVersionLabelKey)
				}
			case setting.ConfigMap, setting.Secret, setting.Service:
			default:
				return nil, errors.Errorf("service %s resource type %s not allowed", service.ServiceName, resource.GetKind())
			}
		}
		if deploymentNum == 0 {
			return nil, errors.Errorf("service-%s: each service must contain one deployment", service.ServiceName)
		}
		resp = append(resp, &commonmodels.JobTask{
			Name: jobNameFormat(service.ServiceName + "-" + j.job.Name),
			Key:  strings.Join([]string{j.job.Name, service.ServiceName}, "."),
			JobInfo: map[string]string{
				JobNameKey:     j.job.Name,
				"service_name": service.ServiceName,
			},
			JobType: string(config.JobMseGrayRelease),
			Spec: commonmodels.JobTaskMseGrayReleaseSpec{
				GrayTag:            j.spec.GrayTag,
				BaseEnv:            j.spec.BaseEnv,
				GrayEnv:            j.spec.GrayEnv,
				SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
				GrayService:        *service,
				Timeout:            timeout,
			},
		})
	}

	return resp, nil
}

func (j *MseGrayReleaseJob) LintJob() error {
	j.spec = &commonmodels.MseGrayReleaseJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec

	return nil
}

func checkMapKeyExist(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}
