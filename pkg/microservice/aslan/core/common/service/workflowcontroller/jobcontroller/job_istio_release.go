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
	"time"

	"go.uber.org/zap"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	ZadigIstioLabelOriginal  = "original"
	ZadigIstioLabelDuplicate = "duplicate"
)

// label definition
const (
	ZadigIstioIdentifierLabel                 = "zadig-istio-release-version"
	ZadigIstioOriginalVSLabel                 = "last-applied-virtual-service"
	ZadigIstioVirtualServiceLastAppliedRoutes = "last-applied-routes"
	WorkloadCreator                           = "workload-creator"
)

// naming conventions
const (
	VirtualServiceNameTemplate     = "%s-vs-zadig"
	ServiceDestinationRuleTemplate = "%s-zadig"
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
	c.job.Status = config.StatusRunning
	c.ack()

	var err error
	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.Errorf("can't init k8s client: %v", err)
		return
	}

	cli, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to prepare istio client to do the resource update"), c.logger)
		return
	}
	// initialize istio client
	// NOTE that the only supported version is v1alpha3 right now
	istioClient, err := kubeclient.GetIstioClientV1Alpha3Client(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		c.Errorf("failed to prepare istio client to do the resource update")
		return
	}

	deployment, found, err := getter.GetDeployment(c.jobTaskSpec.Namespace, c.jobTaskSpec.Targets.WorkloadName, c.kubeClient)
	if err != nil || !found {
		c.Errorf("deployment: %s not found: %v", c.jobTaskSpec.Targets.WorkloadName, err)
		return
	}

	// ==================================================================
	//                     Deployment modification
	// ==================================================================

	// if an original VS is provided, put a label into the label, so we could find it during the rollback
	if c.jobTaskSpec.Targets.VirtualServiceName != "" {
		deployment.Annotations[ZadigIstioOriginalVSLabel] = c.jobTaskSpec.Targets.VirtualServiceName
	} else {
		deployment.Annotations[ZadigIstioOriginalVSLabel] = "none"
	}

	if value, ok := deployment.Spec.Template.Labels[ZadigIstioIdentifierLabel]; ok {
		if value != ZadigIstioLabelOriginal {
			// the given label has to have value: original for the thing to work
			c.Errorf("the deployment %s's label: %s need to have the value: %s to proceed.", deployment.Name, ZadigIstioIdentifierLabel, ZadigIstioLabelOriginal)
			return
		}
	} else {
		// if the specified key does not exist, we simply return error
		c.Errorf("the deployment %s need to have label: %s on its spec.template.labels", deployment.Name, ZadigIstioIdentifierLabel)
		return
	}

	// The logic below will only be applied when this is the first deployment job
	if c.jobTaskSpec.FirstJob {
		c.Infof("Adding annotation to original deployment: %s", c.jobTaskSpec.Targets.WorkloadName)
		c.ack()
		if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
			c.Errorf("add annotations to origin deployment: %s failed: %v", c.jobTaskSpec.Targets.WorkloadName, err)
			return
		}

		originalLabels := make(map[string]string)
		for k, v := range deployment.Spec.Template.Labels {
			originalLabels[k] = v
		}

		// create a new deployment called <deployment-name>-zadig-copy
		newDeployment := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:        fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix),
				Labels:      deployment.ObjectMeta.Labels,
				Annotations: deployment.ObjectMeta.Annotations,
				Namespace:   c.jobTaskSpec.Namespace,
			},
			Spec: deployment.Spec,
		}

		// Add an annotation for filter purpose
		newDeployment.Annotations[WorkloadCreator] = "zadig-istio-release"

		// edit the label of the new deployment so we could find it
		newDeployment.Labels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
		newDeployment.Spec.Selector.MatchLabels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
		newDeployment.Spec.Template.Labels[ZadigIstioIdentifierLabel] = ZadigIstioLabelDuplicate
		// edit the image of the new deployment
		containerList := make([]corev1.Container, 0)
		for _, container := range newDeployment.Spec.Template.Spec.Containers {
			if container.Name == c.jobTaskSpec.Targets.ContainerName {
				newContainer := container.DeepCopy()
				if container.Name == c.jobTaskSpec.Targets.ContainerName {
					newContainer.Image = c.jobTaskSpec.Targets.Image
				}
				containerList = append(containerList, *newContainer)
			}
		}
		newDeployment.Spec.Template.Spec.Containers = containerList
		targetReplica := int32(c.jobTaskSpec.Replicas)
		newDeployment.Spec.Replicas = &targetReplica

		c.Infof("Creating deployment copy for deployment: %s", c.jobTaskSpec.Targets.WorkloadName)
		c.ack()
		if err := updater.CreateOrPatchDeployment(newDeployment, c.kubeClient); err != nil {
			c.Errorf("creating deployment copy: %s failed: %v", fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix), err)
			return
		}

		// waiting for original deployment to run
		c.Infof("Waiting for deployment: %s to start", c.jobTaskSpec.Targets.WorkloadName)
		c.ack()
		if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.Targets.WorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
			c.Errorf("Timout waiting for deployment: %s", c.jobTaskSpec.Targets.WorkloadName)
			c.job.Status = status
			return
		}

		// waiting for the deployment copy to run
		c.Infof("Waiting for the duplicate deployment: %s to start", c.jobTaskSpec.Targets.WorkloadName)
		c.ack()
		if status, err := waitDeploymentReady(ctx, fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix), c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
			c.Errorf("Timout waiting for deployment: %s", fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix))
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
			Labels: originalLabels,
		})

		// appending the subset for duplicate deployment
		subsetList = append(subsetList, &networkingv1alpha3.Subset{
			Name:   ZadigIstioLabelDuplicate,
			Labels: newDeployment.Spec.Template.Labels,
		})

		newDestinationRuleName := fmt.Sprintf(ServiceDestinationRuleTemplate, c.jobTaskSpec.Targets.WorkloadName)
		targetDestinationRule := &v1alpha3.DestinationRule{
			ObjectMeta: v1.ObjectMeta{
				Name: newDestinationRuleName,
			},
			Spec: networkingv1alpha3.DestinationRule{
				Host:    c.jobTaskSpec.Targets.Host,
				Subsets: subsetList,
			},
		}

		c.Infof("Creating new Destination Rule: %s", newDestinationRuleName)
		c.ack()
		_, err = istioClient.DestinationRules(c.jobTaskSpec.Namespace).Create(context.TODO(), targetDestinationRule, v1.CreateOptions{})
		if err != nil {
			c.Errorf("failed to create new destination rule: %s, error: %s", newDestinationRuleName, err)
			return
		}

		// if a virtual service is provided, we simply get it and take its host information
		if c.jobTaskSpec.Targets.VirtualServiceName != "" {
			vs, err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Get(context.TODO(), c.jobTaskSpec.Targets.VirtualServiceName, v1.GetOptions{})
			if err != nil {
				c.Errorf("failed to find virtual service of name: %s, error is: %s", c.jobTaskSpec.Targets.VirtualServiceName, err)
				return
			}

			found := false
			for _, host := range vs.Spec.Hosts {
				if host == c.jobTaskSpec.Targets.Host {
					found = true
				}
				break
			}
			if !found {
				vs.Spec.Hosts = append(vs.Spec.Hosts, c.jobTaskSpec.Targets.Host)
			}

			newHTTPRoutingRules := make([]*networkingv1alpha3.HTTPRouteDestination, 0)
			newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
				Destination: &networkingv1alpha3.Destination{
					Host:   c.jobTaskSpec.Targets.Host,
					Subset: ZadigIstioLabelOriginal,
					Port:   vs.Spec.Http[0].Route[0].Destination.Port,
				},
				Weight: 100 - int32(c.jobTaskSpec.Weight),
			})
			newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
				Destination: &networkingv1alpha3.Destination{
					Host:   c.jobTaskSpec.Targets.Host,
					Subset: ZadigIstioLabelDuplicate,
					Port:   vs.Spec.Http[0].Route[0].Destination.Port,
				},
				Weight: int32(c.jobTaskSpec.Weight),
			})
			routeByte, err := json.Marshal(vs.Spec.Http[0].Route)
			if err != nil {
				c.Errorf("failed to parse route information in virtual service, error: %s", err)
				return
			}
			vs.Annotations[ZadigIstioVirtualServiceLastAppliedRoutes] = string(routeByte)

			vs.Spec.Http[0].Route = newHTTPRoutingRules
			c.Infof("Modifying Virtual Service: %s", c.jobTaskSpec.Targets.VirtualServiceName)
			c.ack()
			_, err = istioClient.VirtualServices(c.jobTaskSpec.Namespace).Update(context.TODO(), vs, v1.UpdateOptions{})
			if err != nil {
				c.Errorf("update virtual service: %s failed, error: %s", c.jobTaskSpec.Targets.VirtualServiceName, err)
				return
			}
		} else {
			vsName := fmt.Sprintf(VirtualServiceNameTemplate, c.jobTaskSpec.Targets.WorkloadName)
			// prepare the route destination
			newHTTPRoutingRules := make([]*networkingv1alpha3.HTTPRouteDestination, 0)
			newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
				Destination: &networkingv1alpha3.Destination{
					Host:   c.jobTaskSpec.Targets.Host,
					Subset: ZadigIstioLabelOriginal,
				},
				Weight: 100 - int32(c.jobTaskSpec.Weight),
			})
			newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
				Destination: &networkingv1alpha3.Destination{
					Host:   c.jobTaskSpec.Targets.Host,
					Subset: ZadigIstioLabelDuplicate,
				},
				Weight: int32(c.jobTaskSpec.Weight),
			})

			// prepare exactly 2 httpRouteDestination for the vs
			httpRoutes := make([]*networkingv1alpha3.HTTPRoute, 0)

			httpRoutes = append(httpRoutes, &networkingv1alpha3.HTTPRoute{
				Route: newHTTPRoutingRules,
			})

			// create zadig's own virtual service
			zadigVirtualService := &v1alpha3.VirtualService{
				ObjectMeta: v1.ObjectMeta{
					Name: vsName,
				},
				Spec: networkingv1alpha3.VirtualService{
					Hosts: []string{c.jobTaskSpec.Targets.Host},
					Http:  httpRoutes,
				},
			}

			c.Infof("Creating virtual service: %s")
			c.ack()

			_, err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Create(context.TODO(), zadigVirtualService, v1.CreateOptions{})
			if err != nil {
				c.Errorf("failed to create virtual service: %s, err: %", vsName, err)
				return
			}
		}
	} else {
		// Otherwise there are 2 cases, either this is a finishing move, or not.
		// When it is NOT a finishing move, simply modify the weight of the vs destination rule, and we are done
		var newVSName string
		if c.jobTaskSpec.Targets.VirtualServiceName == "" {
			newVSName = fmt.Sprintf(VirtualServiceNameTemplate, c.jobTaskSpec.Targets.WorkloadName)
		} else {
			newVSName = c.jobTaskSpec.Targets.VirtualServiceName
		}

		vs, err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Get(context.TODO(), newVSName, v1.GetOptions{})
		if err != nil {
			c.Errorf("failed to find virtual service of name: %s, error is: %s", newVSName, err)
			return
		}

		newHTTPRoutingRules := make([]*networkingv1alpha3.HTTPRouteDestination, 0)
		newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host:   c.jobTaskSpec.Targets.Host,
				Subset: ZadigIstioLabelOriginal,
				Port:   vs.Spec.Http[0].Route[0].Destination.Port,
			},
			Weight: 100 - int32(c.jobTaskSpec.Weight),
		})
		newHTTPRoutingRules = append(newHTTPRoutingRules, &networkingv1alpha3.HTTPRouteDestination{
			Destination: &networkingv1alpha3.Destination{
				Host:   c.jobTaskSpec.Targets.Host,
				Subset: ZadigIstioLabelDuplicate,
				Port:   vs.Spec.Http[0].Route[0].Destination.Port,
			},
			Weight: int32(c.jobTaskSpec.Weight),
		})
		vs.Spec.Http[0].Route = newHTTPRoutingRules
		c.Infof("Modifying Virtual Service: %s", c.jobTaskSpec.Targets.VirtualServiceName)
		c.ack()
		_, err = istioClient.VirtualServices(c.jobTaskSpec.Namespace).Update(context.TODO(), vs, v1.UpdateOptions{})
		if err != nil {
			c.Errorf("update virtual service: %s failed, error: %s", c.jobTaskSpec.Targets.VirtualServiceName, err)
			return
		}

		// If this is a finishing move, following additional steps will have to be done
		// 1. edit the old deployment
		//   a. change the image to the new one
		//   b. adding the old image info to the annotation
		// 2. wait for the old deployment to be ready.
		// 3. revert the virtual service back to normal, so all the queries goes back to the original workload
		// 4. delete the destination rule created by zadig.

		oldImage := ""
		if c.jobTaskSpec.Weight == 100 {
			containerList := make([]corev1.Container, 0)
			for _, container := range deployment.Spec.Template.Spec.Containers {
				newContainer := container.DeepCopy()
				if container.Name == c.jobTaskSpec.Targets.ContainerName {
					oldImage = container.Image
					newContainer.Image = c.jobTaskSpec.Targets.Image
				}
				containerList = append(containerList, *newContainer)
			}

			oldReplicas := strconv.Itoa(int(*deployment.Spec.Replicas))
			deployment.Annotations[config.ZadigLastAppliedReplicas] = oldReplicas

			deployment.Annotations[config.ZadigLastAppliedImage] = oldImage
			deployment.Spec.Template.Spec.Containers = containerList
			targetReplica := int32(c.jobTaskSpec.Replicas)
			deployment.Spec.Replicas = &targetReplica

			c.Infof("updating the original workload %s with the new image: %s", deployment.Name, c.jobTaskSpec.Targets.Image)
			c.ack()

			if err := updater.CreateOrPatchDeployment(deployment, c.kubeClient); err != nil {
				c.Errorf("update origin deployment: %s failed: %v", deployment.Name, err)
				return
			}

			// waiting for original deployment to run
			c.Infof("Waiting for deployment: %s to start", c.jobTaskSpec.Targets.WorkloadName)
			c.ack()
			if status, err := waitDeploymentReady(ctx, c.jobTaskSpec.Targets.WorkloadName, c.jobTaskSpec.Namespace, c.timeout(), c.kubeClient, c.logger); err != nil {
				c.Errorf("Timout waiting for deployment: %s", c.jobTaskSpec.Targets.WorkloadName)
				c.job.Status = status
				return
			}

			modifiedVS, err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Get(context.TODO(), newVSName, v1.GetOptions{})
			if err != nil {
				c.Errorf("failed to find virtual service of name: %s, error is: %s", newVSName, err)
				return
			}

			// restore the vs to before
			if c.jobTaskSpec.Targets.VirtualServiceName != "" {
				// if there was a configuration before, then we roll it back
				lastAppliedRouteInfo := modifiedVS.Annotations[ZadigIstioVirtualServiceLastAppliedRoutes]
				route := make([]*networkingv1alpha3.HTTPRouteDestination, 0)
				err := json.Unmarshal([]byte(lastAppliedRouteInfo), &route)
				if err != nil {
					c.Errorf("failed to get the last applied virtualservice info, error: %s", err)
					return
				}
				modifiedVS.Spec.Http[0].Route = route
				c.Infof("switching the queries back to the original workload on virtual service: %s", vs.Name)
				c.ack()
				_, err = istioClient.VirtualServices(c.jobTaskSpec.Namespace).Update(context.TODO(), modifiedVS, v1.UpdateOptions{})
				if err != nil {
					c.Errorf("virtual service update failed, error: %s", err)
					return
				}
			} else {
				c.Infof("deleting the virtual service created by zadig: %s", vs.Name)
				c.ack()
				// else we simply delete
				err := istioClient.VirtualServices(c.jobTaskSpec.Namespace).Delete(context.TODO(), modifiedVS.Name, v1.DeleteOptions{})
				if err != nil {
					c.Errorf("virtual service deletion failed, error: %s", err)
					return
				}
			}

			newDestinationRuleName := fmt.Sprintf(ServiceDestinationRuleTemplate, c.jobTaskSpec.Targets.WorkloadName)
			// delete the destination rule created by zadig
			c.Infof("deleteing the destination rule created by zadig: %s", newDestinationRuleName)
			c.ack()

			err = istioClient.DestinationRules(c.jobTaskSpec.Namespace).Delete(context.TODO(), newDestinationRuleName, v1.DeleteOptions{})
			if err != nil {
				c.Errorf("destination rule deletion failed, error: %s", err)
				return
			}

			// finally delete the new deployment created by us
			newDeploymentName := fmt.Sprintf("%s-%s", deployment.Name, config.ZadigIstioCopySuffix)
			c.Infof("Deleting the temporary deployment created by zadig: %s", newDeploymentName)
			err = cli.AppsV1().Deployments(c.jobTaskSpec.Namespace).Delete(context.TODO(), newDeploymentName, v1.DeleteOptions{})
			if err != nil {
				c.Errorf("failed to delete deployment: %s, error: %s", newDeploymentName, err)
				return
			}
		}
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
