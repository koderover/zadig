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
	"fmt"
	"time"

	"go.uber.org/zap"
	"istio.io/api/meta/v1alpha1"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

const (
	ZadigIstioCopySuffix     = "zadig-copy"
	ZadigIstioLabelOriginal  = "original"
	ZadigIstioLabelDuplicate = "duplicate"
)

// label definition
const (
	ZadigIstioIdentifierLabel = "zadig-istio-release-version"
	ZadigIstioOriginalVSLabel = "last-applied-virtual-service"
)

// naming conventions
const (
	VirtualServiceNameTemplate            = "%s-vs-zadig"
	ServiceDestinationRuleTemplate        = "%s-zadig"
	VirtualServiceHttpRoutingNameTemplate = "%s-routing-zadig"
)

type IstioReleaseJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	jobTaskSpec *commonmodels.JobIstioReleaseSpec
	ack         func()
}

func NewIstioReleaseJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *IstioReleaseJobCtl {
	jobTaskSpec := &commonmodels.JobIstioReleaseSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	if jobTaskSpec.Event == nil {
		jobTaskSpec.Event = make([]*commonmodels.Event, 0)
	}
	job.Spec = jobTaskSpec
	return &IstioReleaseJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *IstioReleaseJobCtl) Clean(ctx context.Context) {
}

func (c *IstioReleaseJobCtl) Run(ctx context.Context) {
	var err error
	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.Errorf("can't init k8s client: %v", err)
		return
	}
	// initialize istio client
	// NOTE that the only supported version is v1alpha3 right now
	istioClient, err := kubeclient.GetIstioClientV1Alpha3Client(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.Errorf("failed to prepare istio client to do the resource update")
		return
	}

	deployment, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.Service.WorkloadName, c.kubeClient)
	if err != nil || !found {
		c.Errorf("deployment: %s not found: %v", c.jobTaskSpec.Service.WorkloadName, err)
		return
	}

	// ==================================================================
	//                     Deployment modification
	// ==================================================================

	// if an original VS is provided, put a label into the label, so we could find it during the rollback
	if c.jobTaskSpec.Service.VirtualServiceName != "" {
		deployment.Labels[ZadigIstioOriginalVSLabel] = c.jobTaskSpec.Service.VirtualServiceName
	}

	if value, ok := deployment.Spec.Template.Labels[ZadigIstioIdentifierLabel]; ok {
		if value != ZadigIstioLabelOriginal {
			// the given label has to have value: original for the thing to work
			c.Errorf("the deployment %s's label: %s need to have the value: %s to proceed.", deployment.Name, ZadigIstioIdentifierLabel, ZadigIstioLabelOriginal)
			return
		}
	} else {
		// if the specified key does not exist, we simply return error
		c.Errorf("the deployment %s need to have label: %s on its metadata", deployment.Name, ZadigIstioIdentifierLabel)
		return
	}

	c.Infof("Adding annotation to original deployment: %s", c.jobTaskSpec.Service.WorkloadName)
	c.ack()
	if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
		c.Errorf("add annotations to origin deployment: %s failed: %v", c.jobTaskSpec.Service.WorkloadName, err)
		return
	}

	// create a new deployment called <deployment-name>-zadig-copy
	newDeployment := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix),
			Labels:      deployment.ObjectMeta.Labels,
			Annotations: deployment.ObjectMeta.Annotations,
		},
		Spec: deployment.Spec,
	}

	// edit the label of the new deployment so we could find it
	newDeployment.Labels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
	newDeployment.Spec.Selector.MatchLabels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
	newDeployment.Spec.Template.Labels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
	// edit the image of the new deployment
	for _, container := range newDeployment.Spec.Template.Spec.Containers {
		if container.Name == c.jobTaskSpec.Service.ContainerName {
			container.Image = "docker.io/istio/examples-bookinfo-reviews-v3:1.17.0"
		}
	}

	c.Infof("Creating deployment copy for deployment: %s", c.jobTaskSpec.Service.WorkloadName)
	c.ack()
	if err := updater.CreateOrPatchDeployment(newDeployment, c.kubeClient); err != nil {
		c.Errorf("creating deployment copy: %s failed: %v", fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix), err)
		return
	}

	// waiting for original deployment to run
	c.Infof("Waiting for deployment: %s to start", c.jobTaskSpec.Service.WorkloadName)
	c.ack()
	if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.Service.WorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
		c.Errorf("Timout waiting for deployment: %s", c.jobTaskSpec.Service.WorkloadName)
		c.job.Status = status
		return
	}

	// waiting for the deployment copy to run
	c.Infof("Waiting for the duplicate deployment: %s to start", c.jobTaskSpec.Service.WorkloadName)
	c.ack()
	if status, err := waitDeploymentReady(ctx, fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix), c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
		c.Errorf("Timout waiting for deployment: %s", fmt.Sprintf("%s-%s", deployment.Name, ZadigIstioCopySuffix))
		c.job.Status = status
		return
	}

	// =====================================================================================================
	//                     Istio Virtual service & Destination Rule modification
	// =====================================================================================================

	// creating a destination rule that covers both deployment pods with different subset
	subsetList := make([]*networkingv1alpha3.Subset, 0)

	// appending the subset for original deployment
	subsetList = append(subsetList, &networkingv1alpha3.Subset{
		Name:   ZadigIstioLabelOriginal,
		Labels: deployment.Spec.Template.Labels,
	})

	// appending the subset for duplicate deployment
	subsetList = append(subsetList, &networkingv1alpha3.Subset{
		Name:   ZadigIstioLabelDuplicate,
		Labels: newDeployment.Spec.Template.Labels,
	})

	newDestinationRuleName := fmt.Sprintf(ServiceDestinationRuleTemplate, c.jobTaskSpec.Service.WorkloadName)
	targetDestinationRule := &v1alpha3.DestinationRule{
		ObjectMeta: v1.ObjectMeta{
			Name: newDestinationRuleName,
		},
		Spec: networkingv1alpha3.DestinationRule{
			Host:    c.jobTaskSpec.Service.Host,
			Subsets: subsetList,
		},
	}

	c.Infof("Creating new Destination Rule: %s", newDestinationRuleName)
	c.ack()
	_, err = istioClient.DestinationRules(c.jobTaskSpec.Namespace).Create(context.TODO(), targetDestinationRule, v1.CreateOptions{})
	if err != nil {
		c.Errorf("failed to create new destination rule: %s", newDestinationRuleName)
		return
	}

	// if a virtual service is provided, we simply get it and take its host information
	if c.jobTaskSpec.Service.VirtualServiceName != "" {
		vs, err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Get(context.TODO(), c.jobTaskSpec.Service.VirtualServiceName, v1.GetOptions{})
		if err != nil {
			c.Errorf("failed to find virtual service of name: %s, error is: %s", c.jobTaskSpec.Service.VirtualServiceName, err)
			return
		}

		found := false
		for _, host := range vs.Spec.Hosts {
			if host == c.jobTaskSpec.Service.Host {
				found = true
			}
			break
		}
		if !found {
			vs.Spec.Hosts = append(vs.Spec.Hosts, c.jobTaskSpec.Service.Host)
		}

		newHTTPRoutingRules := make([]*networkingv1alpha3.HTTPRouteDestination, 0)
		newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host:   c.jobTaskSpec.Service.Host,
				Subset: ZadigIstioLabelOriginal,
				Port:   vs.Spec.Http[0].Route[0].Destination.Port,
			},
			Weight: 100 - int32(c.jobTaskSpec.Weight),
		})
		newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host:   c.jobTaskSpec.Service.Host,
				Subset: ZadigIstioLabelOriginal,
				Port:   vs.Spec.Http[0].Route[0].Destination.Port,
			},
			Weight: int32(c.jobTaskSpec.Weight),
		})
		vs.Spec.Http[0].Route = newHTTPRoutingRules
		c.Infof("Modifying Virtual Service: %s", c.jobTaskSpec.Service.VirtualServiceName)
		c.ack()
		_, err = istioClient.VirtualServices(c.jobTaskSpec.Namespace).Update(context.TODO(), vs, v1.UpdateOptions{})
		if err != nil {
			c.Errorf("update virtual service: %s failed, error: %s", c.jobTaskSpec.Service.VirtualServiceName, err)
			return
		}
	} else {
		vsName := fmt.Sprintf(VirtualServiceNameTemplate, c.jobTaskSpec.Service.WorkloadName)
		// create zadig's own virtual service
		_ = &v1alpha3.VirtualService{
			ObjectMeta: v1.ObjectMeta{
				Name: vsName,
			},
			Spec: networkingv1alpha3.VirtualService{
				Hosts:    []string{"my-host"},
				Http:     nil,
				Tls:      nil,
				Tcp:      nil,
				ExportTo: nil,
			},
			Status: v1alpha1.IstioStatus{},
		}

		c.Infof("Creating virtual service: %s")
		c.ack()
	}

	c.job.Status = config.StatusPassed
}

func (c *IstioReleaseJobCtl) Errorf(format string, a ...any) {
	errMsg := fmt.Sprintf(format, a...)
	logError(c.job, errMsg, c.logger)
	c.jobTaskSpec.Event = append(c.jobTaskSpec.Event, &commonmodels.Event{
		EventType: "error",
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Message:   errMsg,
	})
}

func (c *IstioReleaseJobCtl) Infof(format string, a ...any) {
	InfoMsg := fmt.Sprintf(format, a...)
	c.jobTaskSpec.Event = append(c.jobTaskSpec.Event, &commonmodels.Event{
		EventType: "info",
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Message:   InfoMsg,
	})
}

func (c *IstioReleaseJobCtl) timeout() int64 {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	} else {
		c.jobTaskSpec.Timeout = c.jobTaskSpec.Timeout * 60
	}
	return c.jobTaskSpec.Timeout
}
