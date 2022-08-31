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

package stepcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/types/step"
)

type customDeployCtl struct {
	step             *commonmodels.StepTask
	customDeploySpec *step.StepCustomDeploySpec
	workflowCtx      *commonmodels.WorkflowTaskCtx
	log              *zap.SugaredLogger
	kubeClient       client.Client
}

func NewCustomDeployCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*customDeployCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal custom deploy spec error: %v", err)
	}
	customDeploySpec := &step.StepCustomDeploySpec{}
	if err := yaml.Unmarshal(yamlString, &customDeploySpec); err != nil {
		return nil, fmt.Errorf("unmarshal custom deploy spec error: %v", err)
	}
	stepTask.Spec = customDeploySpec
	return &customDeployCtl{customDeploySpec: customDeploySpec, workflowCtx: workflowCtx, log: log, step: stepTask}, nil
}

func (s *customDeployCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *customDeployCtl) Run(ctx context.Context) (config.Status, error) {
	if err := s.run(ctx); err != nil {
		return config.StatusFailed, err
	}
	if s.customDeploySpec.SkipCheckRunStatus {
		return config.StatusPassed, nil
	}
	return s.wait(ctx)
}

func (s *customDeployCtl) AfterRun(ctx context.Context) error {
	return nil
}

func (s *customDeployCtl) run(ctx context.Context) error {
	var err error
	if s.customDeploySpec.ClusterID != "" {
		s.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), s.customDeploySpec.ClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't init k8s client")
			return err
		}
	} else {
		s.kubeClient = krkubeclient.Client()
	}
	replaced := false

	switch s.customDeploySpec.WorkloadType {
	case setting.Deployment:
		deployment, _, err := getter.GetDeployment(s.customDeploySpec.Namespace, s.customDeploySpec.WorkloadName, s.kubeClient)
		if err != nil {
			return err
		}
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == s.customDeploySpec.ContainerName {
				err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, container.Name, s.customDeploySpec.Image, s.kubeClient)
				if err != nil {
					err = errors.WithMessagef(
						err,
						"failed to update container image in %s/deployments/%s/%s",
						deployment.Namespace, deployment.Name, container.Name)
					return err
				}
				s.customDeploySpec.ReplaceResources = append(s.customDeploySpec.ReplaceResources, step.Resource{
					Kind:      setting.Deployment,
					Container: container.Name,
					Origin:    container.Image,
					Name:      deployment.Name,
				})
				replaced = true
				break
			}
		}
	case setting.StatefulSet:
		statefulSet, _, err := getter.GetStatefulSet(s.customDeploySpec.Namespace, s.customDeploySpec.WorkloadName, s.kubeClient)
		if err != nil {
			return err
		}
		for _, container := range statefulSet.Spec.Template.Spec.Containers {
			if container.Name == s.customDeploySpec.ContainerName {
				err = updater.UpdateDeploymentImage(statefulSet.Namespace, statefulSet.Name, container.Name, s.customDeploySpec.Image, s.kubeClient)
				if err != nil {
					err = errors.WithMessagef(
						err,
						"failed to update container image in %s/statefulset/%s/%s",
						statefulSet.Namespace, statefulSet.Name, container.Name)
					return err
				}
				s.customDeploySpec.ReplaceResources = append(s.customDeploySpec.ReplaceResources, step.Resource{
					Kind:      setting.StatefulSet,
					Container: container.Name,
					Origin:    container.Image,
					Name:      statefulSet.Name,
				})
				replaced = true
				break
			}
		}
	default:
		return fmt.Errorf("workfload type: %s not supported", s.customDeploySpec.WorkloadType)
	}
	if !replaced {
		return errors.Errorf("workload type: %s,name: %s, container %s is not found in namespace %s", s.customDeploySpec.WorkloadType, s.customDeploySpec.WorkloadName, s.customDeploySpec.ContainerName, s.customDeploySpec.Namespace)
	}
	s.step.Spec = s.customDeploySpec
	return nil
}

func (s *customDeployCtl) wait(ctx context.Context) (config.Status, error) {
	timeout := time.After(time.Duration(s.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled, nil

		case <-timeout:
			return config.StatusTimeout, errors.New("deploy timeout")

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
			switch s.customDeploySpec.WorkloadType {
			case setting.Deployment:
				d, found, e := getter.GetDeployment(s.customDeploySpec.Namespace, s.customDeploySpec.WorkloadName, s.kubeClient)
				if e != nil {
					err = e
				}
				if e != nil || !found {
					s.log.Errorf(
						"failed to check deployment ready status %s/%s/%s - %v",
						s.customDeploySpec.Namespace,
						s.customDeploySpec.WorkloadType,
						s.customDeploySpec.WorkloadName,
						e,
					)
					ready = false
				} else {
					ready = wrapper.Deployment(d).Ready()
				}
			case setting.StatefulSet:
				st, found, e := getter.GetStatefulSet(s.customDeploySpec.Namespace, s.customDeploySpec.WorkloadName, s.kubeClient)
				if e != nil {
					err = e
				}
				if err != nil || !found {
					s.log.Errorf(
						"failed to check statefulSet ready status %s/%s/%s",
						s.customDeploySpec.Namespace,
						s.customDeploySpec.WorkloadType,
						s.customDeploySpec.WorkloadName,
						e,
					)
					ready = false
				} else {
					ready = wrapper.StatefulSet(st).Ready()
				}
			default:
				return config.StatusFailed, fmt.Errorf("workfload type: %s not supported", s.customDeploySpec.WorkloadType)
			}
			if ready {
				return config.StatusPassed, nil
			}
		}
	}
}

func (s *customDeployCtl) timeout() int64 {
	if s.customDeploySpec.Timeout == 0 {
		s.customDeploySpec.Timeout = setting.DeployTimeout
	}
	return s.customDeploySpec.Timeout
}
