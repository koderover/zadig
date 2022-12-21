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

package jobcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"istio.io/api/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
)

type IstioRollbackJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobIstioRollbackSpec
	ack         func()
}

func NewIstioRollbackJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *IstioRollbackJobCtl {
	jobTaskSpec := &commonmodels.JobIstioRollbackSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}

	jobTaskSpec.Replicas = 100

	job.Spec = jobTaskSpec
	return &IstioRollbackJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *IstioRollbackJobCtl) Clean(ctx context.Context) {
}

func (c *IstioRollbackJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	var err error

	// initialize istio client
	// NOTE that the only supported version is v1alpha3 right now
	istioClient, err := kubeclient.GetIstioClientV1Alpha3Client(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to prepare istio client to do the resource update"), c.logger)
		return
	}

	cli, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to prepare istio client to do the resource update"), c.logger)
		return
	}

	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		logError(c.job, fmt.Sprintf("can't init k8s client: %v", err), c.logger)
		return
	}

	deployment, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.Targets.WorkloadName, c.kubeClient)
	if err != nil || !found {
		logError(c.job, fmt.Sprintf("deployment: %s not found: %v", c.jobTaskSpec.Targets.WorkloadName, err), c.logger)
		return
	}

	// first we need to delete the destination rule created by zadig
	newDestinationRuleName := fmt.Sprintf(ServiceDestinationRuleTemplate, c.jobTaskSpec.Targets.WorkloadName)
	c.logger.Infof("deleting zadig's destination rule: %s", newDestinationRuleName)

	err = istioClient.DestinationRules(c.jobTaskSpec.Namespace).Delete(context.TODO(), newDestinationRuleName, v1.DeleteOptions{})
	if err != nil {
		// since this is not a fatal error, we simply print an error message and move on
		c.logger.Errorf("failed to delete destination rule: %s, error is: %s", newDestinationRuleName, err)
	}

	// reverting the virtualservice is the second thing to do
	vsName := deployment.Annotations[ZadigIstioOriginalVSLabel]
	// if no vs is provided in the deployment stage, we simply delete the vs created by zadig
	if vsName == "" || vsName == "none" {
		vsName = fmt.Sprintf(VirtualServiceNameTemplate, c.jobTaskSpec.Targets.WorkloadName)
		c.logger.Infof("deleteing virtual service: %s created by zadig", vsName)

		err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Delete(context.TODO(), vsName, v1.DeleteOptions{})
		if err != nil {
			// still, not a fatal error, only error log will be printed
			c.logger.Errorf("failed to delete virtual service: %s, error is: %s", vsName, err)
		}
	} else {
		// otherwise we find the routing information from the annotation
		vs, err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Get(context.TODO(), vsName, v1.GetOptions{})
		if err != nil {
			logError(c.job, fmt.Sprintf("failed to get virtual service: %s, error is: %s", vsName, err), c.logger)
			// this is a fatal error, we will stop here
			return
		}

		routeInfoString := vs.Annotations[ZadigIstioVirtualServiceLastAppliedRoutes]
		route := make([]*v1alpha3.HTTPRouteDestination, 0)
		err = json.Unmarshal([]byte(routeInfoString), &route)
		if err != nil {
			logError(c.job, fmt.Sprintf("failed to unmarshal original route information, error: %s", err), c.logger)
			return
		}

		c.logger.Infof("rolling back virtual service: %s", vs.Name)
		vs.Spec.Http[0].Route = route
		_, err = istioClient.VirtualServices(c.jobTaskSpec.Namespace).Update(context.TODO(), vs, v1.UpdateOptions{})
		if err != nil {
			logError(c.job, fmt.Sprintf("update virtual service: %s failed, error: %s", vs.Name, err), c.logger)
			return
		}
	}

	// Finally we deal with the deployment
	// If there are certain annotations, then the previous release is completed, thus we do a deployment update
	if image, ok := deployment.Annotations[config.ZadigLastAppliedImage]; ok {
		if replicaString, ok := deployment.Annotations[config.ZadigLastAppliedReplicas]; ok {
			replica, err := strconv.Atoi(replicaString)
			if err != nil {
				logError(c.job, fmt.Sprintf("cannot convert replicas: [%s] into valid replica, error: %s", replicaString, err), c.logger)
				return
			}
			replicas := int32(replica)
			delete(deployment.Annotations, config.ZadigLastAppliedReplicas)
			delete(deployment.Annotations, config.ZadigLastAppliedImage)
			deployment.Spec.Replicas = &replicas
			containerList := make([]corev1.Container, 0)
			for _, container := range deployment.Spec.Template.Spec.Containers {
				newContainer := container.DeepCopy()
				if container.Name == c.jobTaskSpec.Targets.ContainerName {
					newContainer.Image = image
				}
				containerList = append(containerList, *newContainer)
			}
			deployment.Spec.Template.Spec.Containers = containerList
			c.logger.Infof("reverting deployment: %s", deployment.Name)
			if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
				logError(c.job, fmt.Sprintf("creating deployment copy: %s failed: %v", fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix), err), c.logger)
				return
			}
			c.logger.Infof("waiting for deployment: %s to start", deployment.Name)
			if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.Targets.WorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
				logError(c.job, fmt.Sprintf("Timout waiting for deployment: %s", c.jobTaskSpec.Targets.WorkloadName), c.logger)
				c.job.Status = status
				return
			}
		} else {
			// something wrong about this deployment, we will stop here
			logError(c.job, fmt.Sprintf("failed to find last applied replicas when the last applied image is found"), c.logger)
			return
		}
	}
	// we try to delete the temporary deployment no matter what the situation is
	newDeploymentName := fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix)
	c.logger.Infof("deleting the deployment created by zadig: %s", newDeploymentName)
	err = cli.AppsV1().Deployments(c.jobTaskSpec.Namespace).Delete(context.TODO(), newDeploymentName, v1.DeleteOptions{})
	if err != nil {
		// not fatal so we just log it
		c.logger.Errorf("failed to delete the deployment: %s created by zadig, error: %s", newDeploymentName, err)
	}

	c.job.Status = config.StatusPassed
}

func (c *IstioRollbackJobCtl) timeout() int64 {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.Timeout = c.jobTaskSpec.Timeout * 60
	}
	return c.jobTaskSpec.Timeout
}
