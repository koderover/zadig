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

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
)

type BlueGreenReleaseV2JobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	namespace   string
	jobTaskSpec *commonmodels.JobTaskBlueGreenReleaseV2Spec
	ack         func()
}

func NewBlueGreenReleaseV2JobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *BlueGreenReleaseV2JobCtl {
	jobTaskSpec := &commonmodels.JobTaskBlueGreenReleaseV2Spec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Events == nil {
		jobTaskSpec.Events = &commonmodels.Events{}
	}
	job.Spec = jobTaskSpec
	return &BlueGreenReleaseV2JobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger.With("ctl", "BlueGreenReleaseV2"),
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *BlueGreenReleaseV2JobCtl) Clean(ctx context.Context) {
	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		c.logger.Errorf("find project error: %v", err)
		return
	}
	c.namespace = env.Namespace
	clusterID := env.ClusterID

	c.kubeClient, err = clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		c.logger.Errorf("can't init k8s client: %v", err)
		return
	}

	// ensure delete blue deployment and service
	err = updater.DeleteDeploymentAndWait(c.namespace, c.jobTaskSpec.Service.BlueDeploymentName, c.kubeClient)
	if err != nil {
		c.logger.Warnf("can't delete blue deployment %s, err: %v", c.jobTaskSpec.Service.BlueDeploymentName, err)
	}
	err = updater.DeleteService(c.namespace, c.jobTaskSpec.Service.BlueServiceName, c.kubeClient)
	if err != nil {
		c.logger.Warnf("can't delete blue service %s, err: %v", c.jobTaskSpec.Service.BlueServiceName, err)
	}

	// ensure green service and pods not contain release label
	greenDeployment, found, err := getter.GetDeployment(c.namespace, c.jobTaskSpec.Service.GreenDeploymentName, c.kubeClient)
	if err != nil || !found {
		c.logger.Errorf("get green deployment: %s error: %v", c.jobTaskSpec.Service.GreenDeploymentName, err)
		return
	}
	greenService, found, err := getter.GetService(c.namespace, c.jobTaskSpec.Service.GreenServiceName, c.kubeClient)
	if err != nil || !found {
		c.logger.Errorf("get green service: %s error: %v", c.jobTaskSpec.Service.GreenServiceName, err)
		return
	}
	if greenService.Spec.Selector == nil {
		c.logger.Errorf("blue service %s selector is nil", c.jobTaskSpec.Service.GreenServiceName)
		return
	}
	// must remove service selector before remove pods labels
	if _, ok := greenService.Spec.Selector[config.BlueGreenVersionLabelName]; ok {
		delete(greenService.Spec.Selector, config.BlueGreenVersionLabelName)
		if err := updater.CreateOrPatchService(greenService, c.kubeClient); err != nil {
			c.logger.Errorf("delete origin label for service error: %v", err)
			return
		}
	}
	pods, err := getter.ListPods(c.namespace, labels.Set(greenDeployment.Spec.Selector.MatchLabels).AsSelector(), c.kubeClient)
	if err != nil {
		c.logger.Errorf("list green deployment %s pods error: %v", c.jobTaskSpec.Service.GreenDeploymentName, err)
		return
	}
	for _, pod := range pods {
		if pod.Labels == nil {
			continue
		}
		if _, ok := pod.Labels[config.BlueGreenVersionLabelName]; ok {
			removeLabelPatch := fmt.Sprintf(`{"metadata":{"labels":{"%s":null}}}`, config.BlueGreenVersionLabelName)
			if err := updater.PatchPod(c.namespace, pod.Name, []byte(removeLabelPatch), c.kubeClient); err != nil {
				c.logger.Errorf("remove origin label to pod error: %v", err)
				continue
			}
		}
	}

	return
}

func (c *BlueGreenReleaseV2JobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	if err := c.run(ctx); err != nil {
		return
	}
	c.wait(ctx)
}

func (c *BlueGreenReleaseV2JobCtl) run(ctx context.Context) error {
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
	c.jobTaskSpec.Namespace = env.Namespace
	clusterID := env.ClusterID

	c.kubeClient, err = clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		c.jobTaskSpec.Events.Error(msg)
		return errors.New(msg)
	}

	// update green deployment image to new version
	for _, v := range c.jobTaskSpec.Service.ServiceAndImage {
		err := updater.UpdateDeploymentImage(c.namespace, c.jobTaskSpec.Service.GreenDeploymentName, v.ServiceModule, v.Image, c.kubeClient)
		if err != nil {
			msg := fmt.Sprintf("can't update deployment %s container %s image %s, err: %v",
				c.jobTaskSpec.Service.GreenDeploymentName, v.ServiceModule, v.Image, err)
			logError(c.job, msg, c.logger)
			c.jobTaskSpec.Events.Error(msg)
			return errors.New(msg)
		}
		if err := commonutil.UpdateProductImage(c.jobTaskSpec.Env, c.workflowCtx.ProjectName, c.jobTaskSpec.Service.ServiceName, map[string]string{v.ServiceModule: v.Image}, "", c.workflowCtx.WorkflowTaskCreatorUsername, c.logger); err != nil {
			msg := fmt.Sprintf("update product image service %s service module %s image %s error: %v", c.jobTaskSpec.Service.ServiceName, v.ServiceModule, v.Image, err)
			logError(c.job, msg, c.logger)
			c.jobTaskSpec.Events.Error(msg)
			return errors.New(msg)
		}
	}

	timeout := time.After(time.Duration(3) * time.Minute)
	err = func() error {
		c.logger.Infof("waiting green deploy update")
		defer c.logger.Infof("green deploy wait update finished")
		for {
			select {
			case <-timeout:
				return errors.New("wait timeout")
			default:
				time.Sleep(time.Second * 3)
				d, found, e := getter.GetDeployment(c.namespace, c.jobTaskSpec.Service.GreenDeploymentName, c.kubeClient)
				if e != nil {
					c.logger.Errorf("failed to find green deoloy: %s, err: %s", c.jobTaskSpec.Service.GreenDeploymentName, e)
					break
				}
				if !found {
					c.logger.Infof("green deploy: %s not found", c.jobTaskSpec.Service.GreenDeploymentName)
					return fmt.Errorf("green deploy: %s not found", c.jobTaskSpec.Service.GreenDeploymentName)
				}
				ready := wrapper.Deployment(d).Ready()
				if !ready {
					break
				}
				// we need to add
				if ready {
					pods, err := getter.ListPods(c.namespace, labels.Set(d.Spec.Selector.MatchLabels).AsSelector(), c.kubeClient)
					if err != nil {
						c.logger.Errorf("list green deployment %s pods error: %v", c.jobTaskSpec.Service.GreenDeploymentName, err)
						return fmt.Errorf("list green deployment %s pods error: %v", c.jobTaskSpec.Service.GreenDeploymentName, err)
					}
					for _, pod := range pods {
						if pod.Labels == nil {
							pod.Labels = make(map[string]string)
							continue
						}
						if _, ok := pod.Labels[config.BlueGreenVersionLabelName]; !ok {
							addLabelPatch := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, config.BlueGreenVersionLabelName, config.OriginVersion)
							if err := updater.PatchPod(c.namespace, pod.Name, []byte(addLabelPatch), c.kubeClient); err != nil {
								c.logger.Errorf("remove origin label to pod error: %v", err)
								continue
							}
						}
					}
				}
				return nil
			}
		}
	}()

	if err != nil {
		c.jobTaskSpec.Events.Info(fmt.Sprintf("failed to wait green deploy: %s ready, err: %s", c.jobTaskSpec.Service.GreenDeploymentName, err))
		c.ack()
		return errors.New(fmt.Sprintf("failed to wait green deploy: %s ready, err: %s", c.jobTaskSpec.Service.GreenDeploymentName, err))
	}

	c.jobTaskSpec.Events.Info(fmt.Sprintf("update deployment %s image success", c.jobTaskSpec.Service.GreenDeploymentName))
	c.ack()

	return nil
}

func (c *BlueGreenReleaseV2JobCtl) wait(ctx context.Context) {
	c.jobTaskSpec.Events.Info(fmt.Sprintf("wait for deployment %s ready", c.jobTaskSpec.Service.GreenDeploymentName))

	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return

		case <-timeout:
			c.job.Status = config.StatusTimeout
			msg := fmt.Sprintf("timeout waiting for deployment %s ready", c.jobTaskSpec.Service.GreenDeploymentName)
			c.jobTaskSpec.Events.Info(msg)
			return

		default:
			time.Sleep(time.Second * 2)
			d, found, err := getter.GetDeployment(c.namespace, c.jobTaskSpec.Service.GreenDeploymentName, c.kubeClient)
			if err != nil || !found {
				c.logger.Errorf(
					"failed to check deployment ready status %s/%s - %v",
					c.namespace,
					c.jobTaskSpec.Service.GreenDeploymentName,
					err,
				)
			} else {
				if wrapper.Deployment(d).Ready() {
					c.job.Status = config.StatusPassed
					msg := fmt.Sprintf("blue-green deployment: %s release successfully", c.jobTaskSpec.Service.GreenDeploymentName)
					c.jobTaskSpec.Events.Info(msg)
					return
				}
			}
		}
	}
}

func (c *BlueGreenReleaseV2JobCtl) timeout() int {
	if c.jobTaskSpec.DeployTimeout == 0 {
		c.jobTaskSpec.DeployTimeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.DeployTimeout
}

func (c *BlueGreenReleaseV2JobCtl) SaveInfo(ctx context.Context) error {
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
