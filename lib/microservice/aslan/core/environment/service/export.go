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

package service

import (
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// ExportYaml 查询使用到服务模板的服务组模板
func ExportYaml(envName, productName, serviceName string, log *xlog.Logger) []string {
	var yamls [][]byte
	res := make([]string, 0)

	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{EnvName: envName, Name: productName})
	if err != nil {
		log.Errorf("failed to find env [%s][%s] %v", envName, productName, err)
		return res
	}

	namespace := env.Namespace
	kubeClient, err := kube.GetKubeClient(env.ClusterId)
	if err != nil {
		log.Errorf("cluster is not connected [%s][%s][%s]", env.EnvName, env.ProductName, env.ClusterId)
		return res
	}

	selector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceName}.AsSelector()
	yamls = append(yamls, getConfigMapYaml(kubeClient, namespace, selector, log)...)
	yamls = append(yamls, getIngressYaml(kubeClient, namespace, selector, log)...)
	yamls = append(yamls, getServiceYaml(kubeClient, namespace, selector, log)...)
	yamls = append(yamls, getDeploymentYaml(kubeClient, namespace, selector, log)...)
	yamls = append(yamls, getStatefulSetYaml(kubeClient, namespace, selector, log)...)

	for _, y := range yamls {
		res = append(res, string(y))
	}
	return res
}

func getConfigMapYaml(kubeClient client.Client, namespace string, selector labels.Selector, log *xlog.Logger) [][]byte {
	resources, err := getter.ListConfigMapsYaml(namespace, selector, kubeClient)
	if err != nil {
		log.Errorf("ListConfigMaps error: %v", err)
		return nil
	}

	return resources
}

func getIngressYaml(kubeClient client.Client, namespace string, selector labels.Selector, log *xlog.Logger) [][]byte {
	resources, err := getter.ListIngressesYaml(namespace, selector, kubeClient)
	if err != nil {
		log.Errorf("ListIngresses error: %v", err)
		return nil
	}

	return resources
}

func getServiceYaml(kubeClient client.Client, namespace string, selector labels.Selector, log *xlog.Logger) [][]byte {
	resources, err := getter.ListServicesYaml(namespace, selector, kubeClient)
	if err != nil {
		log.Errorf("ListServices error: %v", err)
		return nil
	}

	return resources
}

func getDeploymentYaml(kubeClient client.Client, namespace string, selector labels.Selector, log *xlog.Logger) [][]byte {
	resources, err := getter.ListDeploymentsYaml(namespace, selector, kubeClient)
	if err != nil {
		log.Errorf("ListDeployments error: %v", err)
		return nil
	}

	return resources
}

func getStatefulSetYaml(kubeClient client.Client, namespace string, selector labels.Selector, log *xlog.Logger) [][]byte {
	resources, err := getter.ListStatefulSetsYaml(namespace, selector, kubeClient)
	if err != nil {
		log.Errorf("ListStatefulSets error: %v", err)
		return nil
	}

	return resources
}

//func ExportBuildYaml(pipelineName string, log *xlog.Logger) (string, error) {
//
//	resp, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: pipelineName})
//	if err != nil {
//		log.Error(err)
//		return "", err
//	}
//
//	for _, sub := range resp.SubTasks {
//		pre, err := sub.ToPreview()
//		if err != nil {
//			return "", errors.New(e.InterfaceToTaskErrMsg)
//		}
//
//		if pre.TaskType == task.TaskBuild {
//			build, err := sub.ToBuildTask()
//			if err != nil {
//				return "", err
//			}
//			fmtBuildsTask(build, log)
//
//			return build.ToYaml()
//		}
//	}
//
//	return "", errors.New("no build task found")
//}
