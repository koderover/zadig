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
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/releaseutil"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
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

	resources := make([]*unstructured.Unstructured, 0)
	for _, service := range j.spec.GrayServices {
		manifests := releaseutil.SplitManifests(service.YamlContent)
		for _, item := range manifests {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
			if err != nil {
				return resp, errors.Errorf("failed to decode service %s yaml to unstructured: %v", service.ServiceName, err)
			}
			resources = append(resources, u)
		}
	}
	deploymentNum := 0
	for i, resource := range resources {
		switch resource.GetKind() {
		case setting.Deployment:
			if deploymentNum > 0 {
				return resp, errors.Errorf("only one deployment is allowed in service")
			}
			deploymentNum++
			    deploymentObj := &v1.Deployment{}
			resource.
		default:
			return resp, errors.Errorf("%s not allowed", resource.GetKind())
		}
	}

	return []*commonmodels.JobTask{jobTask}, nil
}

func (j *MseGrayReleaseJob) LintJob() error {
	j.spec = &commonmodels.MseGrayReleaseJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec

	return nil
}
