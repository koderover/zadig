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

package kube

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
)

func FetchRelatedWorkloads(namespace, serviceName string,
	productInfo *commonmodels.Product, kubeclient crClient.Client) ([]*appsv1.Deployment, []*appsv1.StatefulSet, error) {

	var err error
	selector := labels.Set{setting.ProductLabel: productInfo.ProductName, setting.ServiceLabel: serviceName}.AsSelector()

	var deployments []*appsv1.Deployment
	deployments, err = getter.ListDeployments(namespace, selector, kubeclient)
	if err != nil {
		return nil, nil, err
	}

	var statefulSets []*appsv1.StatefulSet
	statefulSets, err = getter.ListStatefulSets(namespace, selector, kubeclient)
	if err != nil {
		return nil, nil, err
	}

	if len(deployments) > 0 && len(statefulSets) > 0 {
		return deployments, statefulSets, nil
	}

	productService := productInfo.GetServiceMap()[serviceName]
	if productService == nil {
		return nil, nil, nil
	}
	opt := &commonrepo.RenderSetFindOption{
		Name:        productInfo.Render.Name,
		Revision:    productInfo.Render.Revision,
		EnvName:     productInfo.EnvName,
		ProductTmpl: productInfo.ProductName,
	}
	renderset, exists, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil || !exists {
		log.Errorf("failed to find renderset for env: %s, err: %v", productInfo.EnvName, err)
		return nil, nil, fmt.Errorf("failed to find renderset for env: %s/%s, err: %v", productInfo.ProductName, productInfo.EnvName, err)
	}
	rederedYaml, err := RenderEnvService(productInfo, renderset, productService)
	if err != nil {
		log.Errorf("failed to render service yaml, err: %s", err)
		return nil, nil, fmt.Errorf("failed to render service yaml, err: %s", err)
	}

	deploys, stss := make([]*appsv1.Deployment, 0), make([]*appsv1.StatefulSet, 0)
	manifests := util.SplitManifests(rederedYaml)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Errorf("failed to convert yaml to Unstructured when check resources, manifest is\n%s\n, error: %v", item, err)
			continue
		}
		switch u.GetKind() {
		case setting.Deployment:
			deploy, deployExists, err := getter.GetDeployment(namespace, u.GetName(), kubeclient)
			if deployExists && err == nil {
				deploys = append(deploys, deploy)
			}
		case setting.StatefulSet:
			sts, stsExists, err := getter.GetStatefulSet(namespace, u.GetName(), kubeclient)
			if stsExists && err == nil {
				stss = append(stss, sts)
			}
		}
	}
	return deploys, stss, nil
}

func FetchSelectedWorkloads(namespace string, Resource []*WorkloadResource, kubeclient crClient.Client, clientSet *kubernetes.Clientset) ([]*appsv1.Deployment, []*appsv1.StatefulSet,
	[]*batchv1.CronJob, []*batchv1beta1.CronJob, error) {
	var deployments []*appsv1.Deployment
	var statefulSets []*appsv1.StatefulSet
	var cronJobs []*batchv1.CronJob
	var betaCronJobs []*batchv1beta1.CronJob

	k8sServerVersion, err := clientSet.Discovery().ServerVersion()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	for _, item := range Resource {
		switch item.Type {
		case setting.Deployment:
			deploy, deployExists, err := getter.GetDeployment(namespace, item.Name, kubeclient)
			if deployExists && err == nil {
				deployments = append(deployments, deploy)
			}
			if err != nil {
				log.Errorf("failed to fetch deployment %s, error: %v", item.Name, err)
			}
		case setting.StatefulSet:
			sts, stsExists, err := getter.GetStatefulSet(namespace, item.Name, kubeclient)
			if stsExists && err == nil {
				statefulSets = append(statefulSets, sts)
			}
			if err != nil {
				log.Errorf("failed to fetch statefulset %s, error: %v", item.Name, err)
			}
		case setting.CronJob:
			cronjob, cronjobBeta, cronjobExists, err := getter.GetCronJob(namespace, item.Name, kubeclient, client.VersionLessThan121(k8sServerVersion))
			if err != nil {
				log.Errorf("failed to fetch cronjob %s, error: %v", item.Name, err)
			}
			if cronjob != nil && cronjobExists {
				cronJobs = append(cronJobs, cronjob)
			}
			if cronjobBeta != nil && cronjobExists {
				betaCronJobs = append(betaCronJobs, cronjobBeta)
			}
		}
	}
	return deployments, statefulSets, cronJobs, betaCronJobs, nil
}
