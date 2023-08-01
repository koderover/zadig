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
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

type BlueGreenDeployV2Job struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.BlueGreenDeployV2JobSpec
}

func (j *BlueGreenDeployV2Job) Instantiate() error {
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BlueGreenDeployV2Job) SetPreset() error {
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BlueGreenDeployV2Job) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.BlueGreenDeployV2JobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.Services = argsSpec.Services
		j.job.Spec = j.spec
	}
	return nil
}

func (j *BlueGreenDeployV2Job) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	for _, target := range j.spec.Services {
		if target.KubernetesServiceYaml != "" {
			svc := &corev1.Service{}
			decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
			err := runtime.DecodeInto(decoder, []byte(target.KubernetesServiceYaml), svc)
			if err != nil {
				return resp, errors.Errorf("failed to decode %s k8s service yaml, err: %s", target.ServiceName, err)
			}
		}

		yamlContent, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
			ProductName: j.workflow.Project,
			EnvName:     j.spec.Env,
			ServiceName: target.ServiceName,
		})
		if err != nil {
			return resp, errors.Errorf("failed to fetch %s current applied yaml, err: %s", target.ServiceName, err)
		}
		var yamls []string
		resources := make([]*unstructured.Unstructured, 0)
		manifests := releaseutil.SplitManifests(yamlContent)
		for _, item := range manifests {
			u, err := runtime.Serializer.Decode().ToUnstructured(&item)
			if err != nil {
				return resp, errors.Errorf("failed to decode service %s yaml to unstructured: %v", target.ServiceName, err)
			}
			resources = append(resources, u)
		}

		task := &commonmodels.JobTask{
			Name: jobNameFormat(j.job.Name + "-" + target.ServiceName),
			Key:  strings.Join([]string{j.job.Name, target.K8sServiceName}, "."),
			JobInfo: map[string]string{
				JobNameKey:         j.job.Name,
				"k8s_service_name": target.K8sServiceName,
			},
			JobType: string(config.JobK8sBlueGreenDeploy),
			Spec: &commonmodels.JobTaskBlueGreenDeployV2Spec{
				Namespace:          j.spec.Namespace,
				ClusterID:          j.spec.ClusterID,
				DockerRegistryID:   j.spec.DockerRegistryID,
				DeployTimeout:      target.DeployTimeout,
				K8sServiceName:     target.K8sServiceName,
				BlueK8sServiceName: target.BlueK8sServiceName,
				WorkloadType:       setting.Deployment,
				WorkloadName:       deployment.Name,
				BlueWorkloadName:   target.BlueWorkloadName,
				ContainerName:      target.ContainerName,
				Image:              target.Image,
				Version:            version,
			},
		}
		resp = append(resp, task)
	}

	j.job.Spec = j.spec
	return resp, nil
}

func (j *BlueGreenDeployV2Job) LintJob() error {
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	quoteJobs := []*commonmodels.Job{}
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobK8sBlueGreenRelease {
				continue
			}
			releaseJobSpec := &commonmodels.BlueGreenReleaseJobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, releaseJobSpec); err != nil {
				return err
			}
			if releaseJobSpec.FromJob == j.job.Name {
				quoteJobs = append(quoteJobs, job)
			}
		}
	}
	if len(quoteJobs) == 0 {
		return errors.Errorf("no blue-green release job quote blue-green deploy job %s", j.job.Name)
	}
	if len(quoteJobs) > 1 {
		return errors.Errorf("more than one blue-green release job quote blue-green deploy job %s", j.job.Name)
	}
	jobRankmap := getJobRankMap(j.workflow.Stages)
	if jobRankmap[j.job.Name] >= jobRankmap[quoteJobs[0].Name] {
		return errors.Errorf("blue-green release job %s should run before blue-green deploy job %s", quoteJobs[0].Name, j.job.Name)
	}
	return nil
}

func getBlueWorkloadName(name, version string) string {
	reg, _ := regexp.Compile("v[0-9]{10}$")
	blueWorkfloadName := reg.ReplaceAllString(name, version)
	if blueWorkfloadName == name {
		blueWorkfloadName = name + "-" + version
	}
	return blueWorkfloadName
}
