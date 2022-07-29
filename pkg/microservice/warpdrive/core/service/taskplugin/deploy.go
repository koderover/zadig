/*
Copyright 2021 The KodeRover Authors.

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

package taskplugin

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

// InitializeDeployTaskPlugin to initiate deploy task plugin and return ref
func InitializeDeployTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &DeployTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		restConfig: krkubeclient.RESTConfig(),
		httpClient: httpclient.New(
			httpclient.SetHostURL(configbase.AslanServiceAddress()),
		),
	}
}

// DeployTaskPlugin Plugin name should be compatible with task type
type DeployTaskPlugin struct {
	Name         config.TaskType
	JobName      string
	kubeClient   client.Client
	restConfig   *rest.Config
	Task         *task.Deploy
	Log          *zap.SugaredLogger
	ReplaceImage string

	httpClient *httpclient.Client
}

func (p *DeployTaskPlugin) SetAckFunc(func()) {
}

const (
	imageUrlParseRegexString = `(?P<repo>.+/)?(?P<image>[^:]+){1}(:)?(?P<tag>.+)?`
)

var (
	imageParseRegex = regexp.MustCompile(imageUrlParseRegexString)
)

// Init ...
func (p *DeployTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *DeployTaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *DeployTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *DeployTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *DeployTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = setting.DeployTimeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}

	return p.Task.Timeout
}

type EnvArgs struct {
	EnvName     string `json:"env_name"`
	ProductName string `json:"product_name"`
}

type ResourceComponentSet interface {
	GetName() string
	GetAnnotations() map[string]string
	GetContainers() []*resource.ContainerImage
	GetKind() string
}

func (p *DeployTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, _ *task.PipelineCtx, _ string) {
	var (
		err      error
		replaced = false
	)

	defer func() {
		if err != nil {
			p.Log.Error(err)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			return
		}
	}()

	if pipelineTask.ConfigPayload.DeployClusterID != "" {
		p.restConfig, err = kubeclient.GetRESTConfig(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't get k8s rest config")
			return
		}

		p.kubeClient, err = kubeclient.GetKubeClient(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't init k8s client")
			return
		}
	}
	containerName := p.Task.ContainerName
	containerName = strings.TrimSuffix(containerName, "_"+p.Task.ServiceName)

	// get servcie info
	var (
		serviceInfo *types.ServiceTmpl
		selector    labels.Selector
	)
	serviceInfo, err = p.getService(ctx, p.Task.ServiceName, p.Task.ServiceType, p.Task.ProductName, 0)
	if err != nil {
		// Maybe it is a share service, the entity is not under the project
		serviceInfo, err = p.getService(ctx, p.Task.ServiceName, p.Task.ServiceType, "", 0)
		if err != nil {
			return
		}
	}
	if serviceInfo.WorkloadType == "" {
		selector = labels.Set{setting.ProductLabel: p.Task.ProductName, setting.ServiceLabel: p.Task.ServiceName}.AsSelector()

		var deployments []*appsv1.Deployment
		deployments, err = getter.ListDeployments(p.Task.Namespace, selector, p.kubeClient)
		if err != nil {
			return
		}

		var statefulSets []*appsv1.StatefulSet
		statefulSets, err = getter.ListStatefulSets(p.Task.Namespace, selector, p.kubeClient)
		if err != nil {
			return
		}

	L:
		for _, deploy := range deployments {
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, containerName, p.Task.Image, p.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/deployments/%s/%s",
							p.Task.Namespace, deploy.Name, container.Name)
						return
					}
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.Deployment,
						Container: container.Name,
						Origin:    container.Image,
						Name:      deploy.Name,
					})
					replaced = true
					break L
				}
			}
		}
	Loop:
		for _, sts := range statefulSets {
			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, containerName, p.Task.Image, p.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/statefulsets/%s/%s",
							p.Task.Namespace, sts.Name, container.Name)
						return
					}
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.StatefulSet,
						Container: container.Name,
						Origin:    container.Image,
						Name:      sts.Name,
					})
					replaced = true
					break Loop
				}
			}
		}
	} else {
		switch serviceInfo.WorkloadType {
		case setting.StatefulSet:
			var statefulSet *appsv1.StatefulSet
			statefulSet, _, err = getter.GetStatefulSet(p.Task.Namespace, p.Task.ServiceName, p.kubeClient)
			if err != nil {
				return
			}
			for _, container := range statefulSet.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					err = updater.UpdateStatefulSetImage(statefulSet.Namespace, statefulSet.Name, containerName, p.Task.Image, p.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/statefulsets/%s/%s",
							p.Task.Namespace, statefulSet.Name, container.Name)
						return
					}
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.StatefulSet,
						Container: container.Name,
						Origin:    container.Image,
						Name:      statefulSet.Name,
					})
					replaced = true
					break
				}
			}
		case setting.Deployment:
			var deployment *appsv1.Deployment
			deployment, _, err = getter.GetDeployment(p.Task.Namespace, p.Task.ServiceName, p.kubeClient)
			if err != nil {
				return
			}
			for _, container := range deployment.Spec.Template.Spec.Containers {
				if container.Name == containerName {
					err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, containerName, p.Task.Image, p.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/deployments/%s/%s",
							p.Task.Namespace, deployment.Name, container.Name)
						return
					}
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.Deployment,
						Container: container.Name,
						Origin:    container.Image,
						Name:      deployment.Name,
					})
					replaced = true
					break
				}
			}
		}
	}
	if !replaced {
		err = errors.Errorf(
			"container %s is not found in resources with label %s", containerName, selector)
		return
	}
}

func (p *DeployTaskPlugin) getService(ctx context.Context, name, serviceType, productName string, revision int64) (*types.ServiceTmpl, error) {
	url := fmt.Sprintf("/api/service/services/%s/%s", name, serviceType)

	s := &types.ServiceTmpl{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(s), httpclient.SetQueryParams(map[string]string{
		"projectName": productName,
		"revision":    fmt.Sprintf("%d", revision),
	}))
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Wait ...
func (p *DeployTaskPlugin) Wait(ctx context.Context) {
	// skip waiting for reset image task
	if p.Task.SkipWaiting {
		p.Task.TaskStatus = config.StatusPassed
		return
	}

	timeout := time.After(time.Duration(p.TaskTimeout()) * time.Second)

	selector := labels.Set{setting.ProductLabel: p.Task.ProductName, setting.ServiceLabel: p.Task.ServiceName}.AsSelector()

	for {
		select {
		case <-ctx.Done():
			p.Task.TaskStatus = config.StatusCancelled
			return

		case <-timeout:
			p.Task.TaskStatus = config.StatusTimeout

			pods, err := getter.ListPods(p.Task.Namespace, selector, p.kubeClient)
			if err != nil {
				p.Task.Error = err.Error()
				return
			}

			var msg []string
			for _, pod := range pods {
				podResource := wrapper.Pod(pod).Resource()
				if podResource.Status != setting.StatusRunning && podResource.Status != setting.StatusSucceeded {
					for _, cs := range podResource.ContainerStatuses {
						// message为空不认为是错误状态，有可能还在waiting
						if cs.Message != "" {
							msg = append(msg, fmt.Sprintf("Status: %s, Reason: %s, Message: %s", cs.Status, cs.Reason, cs.Message))
						}
					}
				}
			}

			if len(msg) != 0 {
				p.Task.Error = strings.Join(msg, "\n")
			}

			return

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
		L:
			for _, resource := range p.Task.ReplaceResources {
				switch resource.Kind {
				case setting.Deployment:
					d, found, e := getter.GetDeployment(p.Task.Namespace, resource.Name, p.kubeClient)
					if e != nil {
						err = e
					}
					if e != nil || !found {
						p.Log.Errorf(
							"failed to check deployment ready status %s/%s/%s - %v",
							p.Task.Namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = wrapper.Deployment(d).Ready()
					}

					if !ready {
						break L
					}
				case setting.StatefulSet:
					s, found, e := getter.GetStatefulSet(p.Task.Namespace, resource.Name, p.kubeClient)
					if e != nil {
						err = e
					}
					if err != nil || !found {
						p.Log.Errorf(
							"failed to check statefulSet ready status %s/%s/%s",
							p.Task.Namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = wrapper.StatefulSet(s).Ready()
					}

					if !ready {
						break L
					}
				}
			}

			if ready {
				p.Task.TaskStatus = config.StatusPassed
			}

			if p.IsTaskDone() {
				return
			}
		}
	}
}

func (p *DeployTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

func (p *DeployTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToDeployTask(t)
	if err != nil {
		return err
	}
	p.Task = task

	return nil
}

// GetTask ...
func (p *DeployTaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *DeployTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *DeployTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *DeployTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *DeployTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *DeployTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *DeployTaskPlugin) ResetError() {
	p.Task.Error = ""
}
