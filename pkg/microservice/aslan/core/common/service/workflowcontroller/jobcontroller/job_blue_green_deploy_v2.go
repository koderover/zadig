/*
Copyright 2023 The KodeRover Authors.

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

package jobcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
)

type BlueGreenDeployV2JobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	namespace   string
	jobTaskSpec *commonmodels.JobTaskBlueGreenDeployV2Spec
	ack         func()
}

func NewBlueGreenDeployV2JobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *BlueGreenDeployV2JobCtl {
	jobTaskSpec := &commonmodels.JobTaskBlueGreenDeployV2Spec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &BlueGreenDeployV2JobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *BlueGreenDeployV2JobCtl) Clean(ctx context.Context) {}

func (c *BlueGreenDeployV2JobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	if err := c.run(ctx); err != nil {
		return
	}
	c.wait(ctx)
}

func (c *BlueGreenDeployV2JobCtl) run(ctx context.Context) error {
	var err error

	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		msg := fmt.Sprintf("find project error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	c.namespace = env.Namespace
	clusterID := env.ClusterID

	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	// get raw green
	greenDeployment, found, err := getter.GetDeployment(c.namespace, c.jobTaskSpec.Service.GreenDeploymentName, c.kubeClient)
	if err != nil || !found {
		msg := fmt.Sprintf("get green deployment: %s error: %v", c.jobTaskSpec.Service.GreenDeploymentName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	greenService, found, err := getter.GetService(c.namespace, c.jobTaskSpec.Service.GreenServiceName, c.kubeClient)
	if err != nil || !found {
		msg := fmt.Sprintf("get green service: %s error: %v", c.jobTaskSpec.Service.GreenServiceName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	if greenService.Spec.Selector == nil {
		msg := fmt.Sprintf("blue service %s selector is nil", c.jobTaskSpec.Service.GreenServiceName)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	// raw pods list add original version label
	pods, err := getter.ListPods(c.namespace, labels.Set(greenDeployment.Spec.Selector.MatchLabels).AsSelector(), c.kubeClient)
	if err != nil {
		msg := fmt.Sprintf("list green deployment %s pods error: %v", c.jobTaskSpec.Service.GreenDeploymentName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	for _, pod := range pods {
		addlabelPatch := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, config.BlueGreenVerionLabelName, config.OriginVersion)
		if err := updater.PatchPod(c.namespace, pod.Name, []byte(addlabelPatch), c.kubeClient); err != nil {
			msg := fmt.Sprintf("add origin label to pod error: %v", err)
			logError(c.job, msg, c.logger)
			c.jobTaskSpec.Events.Error(msg)
			return errors.New(msg)
		}
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("add origin label to %d pods", len(pods)))
	c.ack()

	// green service selector add original version label
	greenService.Spec.Selector[config.BlueGreenVerionLabelName] = config.OriginVersion
	if err := updater.CreateOrPatchService(greenService, c.kubeClient); err != nil {
		msg := fmt.Sprintf("add origin label selector to green serivce: %s error: %v", greenService.Name, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("add origin label selector to service: %s", c.jobTaskSpec.Service.ServiceName))
	c.ack()

	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()

	// create blue service
	service := &corev1.Service{}
	err = runtime.DecodeInto(decoder, []byte(c.jobTaskSpec.Service.BlueServiceYaml), service)
	if err != nil {
		return errors.Errorf("failed to decode %s k8s service yaml, err: %s", c.jobTaskSpec.Service.ServiceName, err)
	}
	service.Namespace = c.namespace
	if err := c.kubeClient.Create(ctx, service); err != nil {
		msg := fmt.Sprintf("create blue serivce: %s error: %v", c.jobTaskSpec.Service.ServiceName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("create blue serivce: %s success", service.Name))

	// create blue deployment
	deployment := &v1.Deployment{}
	err = runtime.DecodeInto(decoder, []byte(c.jobTaskSpec.Service.BlueDeploymentYaml), deployment)
	if err != nil {
		return errors.Errorf("failed to decode %s k8s deployment yaml, err: %s", c.jobTaskSpec.Service.ServiceName, err)
	}
	deployment.Namespace = c.namespace
	if err := c.kubeClient.Create(ctx, deployment); err != nil {
		msg := fmt.Sprintf("create blue deployment: %s error: %v", c.jobTaskSpec.Service.ServiceName, err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}
	c.jobTaskSpec.Events.Info(fmt.Sprintf("create blue deployment: %s success", deployment.Name))

	return nil
}

func (c *BlueGreenDeployV2JobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return

		case <-timeout:
			c.job.Status = config.StatusTimeout
			msg := fmt.Sprintf("timeout waiting for the blue deployment: %s to run", c.jobTaskSpec.Service.BlueDeploymentName)
			c.jobTaskSpec.Events.Info(msg)
			return

		default:
			time.Sleep(time.Second * 2)
			d, found, err := getter.GetDeployment(c.namespace, c.jobTaskSpec.Service.BlueDeploymentName, c.kubeClient)
			if err != nil || !found {
				c.logger.Errorf(
					"failed to check deployment ready status %s/%s - %v",
					c.namespace,
					c.jobTaskSpec.Service.BlueDeploymentName,
					err,
				)
			} else {
				if wrapper.Deployment(d).Ready() {
					c.job.Status = config.StatusPassed
					msg := fmt.Sprintf("blue-green deployment: %s create successfully", c.jobTaskSpec.Service.BlueDeploymentName)
					c.jobTaskSpec.Events.Info(msg)
					return
				}
			}
		}
	}
}

func (c *BlueGreenDeployV2JobCtl) timeout() int {
	if c.jobTaskSpec.DeployTimeout == 0 {
		c.jobTaskSpec.DeployTimeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.DeployTimeout
}

func (c *BlueGreenDeployV2JobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}
