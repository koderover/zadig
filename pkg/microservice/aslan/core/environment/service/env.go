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
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
)

type envHandle interface {
	createGroup(username string, product *commonmodels.Product, group []*commonmodels.ProductService, renderSet *commonmodels.RenderSet, inf informers.SharedInformerFactory, kubeClient client.Client) error
	listGroupServices(allServices []*commonmodels.ProductService, envName, productName string, informer informers.SharedInformerFactory, productInfo *commonmodels.Product) []*commonservice.ServiceResp
	updateService(args *SvcOptArgs) error
	//queryServiceStatus( serviceTmpl *commonmodels.Service, kubeClient informers.SharedInformerFactory) (string, string, []string)
}

func envHandleFunc(projectType string, log *zap.SugaredLogger) envHandle {
	switch projectType {
	case setting.K8SDeployType:
		return &K8sService{
			log: log,
		}
	case setting.PMDeployType:
		return &PMService{
			log: log,
		}
	}
	return nil
}

func UpdateService(args *SvcOptArgs, log *zap.SugaredLogger) error {
	projectType := getProjectType(args.ProductName)
	return envHandleFunc(projectType, log).updateService(args)
}
