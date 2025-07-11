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

package service

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

type DeployableEnv struct {
	EnvName           string                          `json:"env_name"`
	Alias             string                          `json:"alias"`
	Namespace         string                          `json:"namespace"`
	ClusterID         string                          `json:"cluster_id"`
	Services          []*types.ServiceWithVariable    `json:"services"`
	GlobalVariableKVs []*commontypes.GlobalVariableKV `json:"global_variable_kvs"`
}

type DeployableEnvResp struct {
	Envs []*DeployableEnv `json:"envs"`
}

// GetDeployableEnvs The service can be deployed only in the following situations:
//  1. All general environments are deployable.
//  2. All base environments are deployable.
//  3. If the service has been deployed in the baseline environment, all sub-environments of the baseline environment
//     can deploy the service.
//     Otherwise, all sub-environments of the baseline environment cannot deploy the service.
func GetDeployableEnvs(svcName, projectName string, production bool) (*DeployableEnvResp, error) {

	product, err := templatemodels.NewProductColl().Find(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to find template product %s, err: %s ", projectName, err)
	}

	resp := &DeployableEnvResp{Envs: make([]*DeployableEnv, 0)}
	// 1. Get all general environments.
	envs0, err := getAllGeneralEnvs(product, production)
	if err != nil {
		return nil, err
	}

	// 2. Get all deployable environments in the context of environment sharing..
	envs1, err := getDeployableShareEnvs(svcName, product, production)
	if err != nil {
		return nil, err
	}

	resp.Envs = envs0
	resp.Envs = append(resp.Envs, envs1...)

	return resp, nil
}

type GetKubeWorkloadsResp struct {
	WorkloadsMap map[string][]string `json:"workloads_map"`
}

func GetKubeWorkloads(namespace, clusterID string, log *zap.SugaredLogger) (*GetKubeWorkloadsResp, error) {
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("cluster is not connected [%s] err:%s", clusterID, err)
		return nil, err
	}
	cliSet, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return nil, err
	}

	deployments, err := getter.ListDeployments(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("GetKubeWorkloads ListDeployments error, error msg:%s", err)
		return nil, err
	}
	workloadsMap := make(map[string][]string)
	var deployNames []string
	for _, deployment := range deployments {
		deployNames = append(deployNames, deployment.Name)
	}
	workloadsMap["deployment"] = deployNames
	configMaps, err := getter.ListConfigMaps(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("GetKubeWorkloads ListConfigMaps error, error msg:%s", err)
		return nil, err
	}
	var configMapNames []string
	for _, configmap := range configMaps {
		configMapNames = append(configMapNames, configmap.Name)
	}
	workloadsMap["configmap"] = configMapNames
	services, err := getter.ListServices(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("GetKubeWorkloads ListServices error, error msg:%s", err)
		return nil, err
	}
	var serviceNames []string
	for _, service := range services {
		serviceNames = append(serviceNames, service.Name)
	}
	workloadsMap["service"] = serviceNames

	version, err := cliSet.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("Failed to get server version info for cluster: %s, the error is: %s", cliSet, err)
		return nil, err
	}
	ingresses, err := getter.ListIngresses(namespace, kubeClient, kubeclient.VersionLessThan122(version))
	if err != nil {
		log.Errorf("GetKubeWorkloads ListIngresses error, error msg:%s", err)
		return nil, err
	}

	var ingressNames []string
	for _, ingress := range ingresses.Items {
		ingressNames = append(ingressNames, ingress.GetName())
	}
	workloadsMap["ingress"] = ingressNames
	secrets, err := getter.ListSecrets(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("GetKubeWorkloads ListSecrets error, error msg:%s", err)
		return nil, err
	}
	var secretNames []string
	for _, secret := range secrets {
		secretNames = append(secretNames, secret.Name)
	}
	workloadsMap["secret"] = secretNames
	statefulsets, err := getter.ListStatefulSets(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("GetKubeWorkloads ListStatefulSets error, error msg:%s", err)
		return nil, err
	}
	var statefulsetNames []string
	for _, statefulset := range statefulsets {
		statefulsetNames = append(statefulsetNames, statefulset.Name)
	}
	workloadsMap["statefulset"] = statefulsetNames
	pvcs, err := getter.ListPvcs(namespace, nil, kubeClient)
	if err != nil {
		log.Errorf("GetKubeWorkloads ListPvcs error, error msg:%s", err)
		return nil, err
	}
	var pvcNames []string
	for _, pvc := range pvcs {
		pvcNames = append(pvcNames, pvc.Name)
	}
	workloadsMap["pvc"] = pvcNames
	return &GetKubeWorkloadsResp{
		WorkloadsMap: workloadsMap,
	}, nil
}

type ServiceWorkloads struct {
	Name         string              `json:"name"`
	WorkloadsMap map[string][]string `json:"workloads_map"`
}

type LoadKubeWorkloadsYamlReq struct {
	ProductName string             `json:"product_name"`
	Visibility  string             `json:"visibility"`
	Type        string             `json:"type"`
	Namespace   string             `json:"namespace"`
	ClusterID   string             `json:"cluster_id"`
	Services    []ServiceWorkloads `json:"services"`
}

type ServiceYaml struct {
	Name string `json:"name"`
	Yaml string `json:"yaml"`
}

type GetKubeWorkloadsYamlResp struct {
	Services []ServiceYaml `json:"services"`
}

// LoadKubeWorkloadsYaml creates service from existing workloads in k8s namespace
func LoadKubeWorkloadsYaml(username string, params *LoadKubeWorkloadsYamlReq, force, proudction bool, log *zap.SugaredLogger) error {
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(params.ClusterID)
	if err != nil {
		log.Errorf("cluster is not connected [%s]", params.ClusterID)
		return e.ErrGetService.AddErr(err)
	}
	cliSet, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(params.ClusterID)
	if err != nil {
		return e.ErrGetService.AddErr(err)
	}
	for _, service := range params.Services {
		var yamls []string
		for workloadType, workloads := range service.WorkloadsMap {
			switch workloadType {
			case "configmap":
				for _, workload := range workloads {
					bs, _, err := getter.GetConfigMapYamlFormat(params.Namespace, workload, kubeClient)
					if len(bs) == 0 || err != nil {
						log.Errorf("not found yaml %v", err)
						return e.ErrGetService.AddDesc(fmt.Sprintf("get deploy/configmap failed err:%s", err))
					}

					yamls = append(yamls, string(bs))
				}
			case "deployment":
				for _, workload := range workloads {
					bs, _, err := getter.GetDeploymentYamlFormat(params.Namespace, workload, kubeClient)
					if len(bs) == 0 || err != nil {
						log.Errorf("not found yaml %v", err)
						return e.ErrGetService.AddDesc(fmt.Sprintf("get deploy/deployment failed err:%s", err))
					}

					yamls = append(yamls, string(bs))
				}
			case "service":
				for _, workload := range workloads {
					bs, _, err := getter.GetServiceYamlFormat(params.Namespace, workload, kubeClient)
					if len(bs) == 0 || err != nil {
						log.Errorf("not found yaml %v", err)
						return e.ErrGetService.AddDesc(fmt.Sprintf("get deploy/service failed err:%s", err))
					}

					yamls = append(yamls, string(bs))
				}
			case "secret":
				for _, workload := range workloads {
					bs, _, err := getter.GetSecretYamlFormat(params.Namespace, workload, kubeClient)
					if len(bs) == 0 || err != nil {
						log.Errorf("not found yaml %v", err)
						return e.ErrGetService.AddDesc(fmt.Sprintf("get deploy/secret failed err:%s", err))
					}

					yamls = append(yamls, string(bs))
				}
			case "ingress":
				version, err := cliSet.Discovery().ServerVersion()
				if err != nil {
					log.Errorf("Failed to get server version info for cluster: %s, the error is: %s", cliSet, err)
					return e.ErrGetService.AddErr(err)
				}
				for _, workload := range workloads {
					bs, _, err := getter.GetIngressYaml(params.Namespace, workload, kubeClient, kubeclient.VersionLessThan122(version))
					if len(bs) == 0 || err != nil {
						log.Errorf("not found yaml %v", err)
						return e.ErrGetService.AddDesc(fmt.Sprintf("get deploy/ingress failed err:%s", err))
					}
					yamls = append(yamls, string(bs))
				}
			case "statefulset":
				for _, workload := range workloads {
					bs, _, err := getter.GetStatefulSetYamlFormat(params.Namespace, workload, kubeClient)
					if len(bs) == 0 || err != nil {
						log.Errorf("not found yaml %v", err)
						return fmt.Errorf("get deploy/statefulset failed err:%s", err)
					}
					yamls = append(yamls, string(bs))
				}
			case "pvc":
				for _, workload := range workloads {
					bs, _, err := getter.GetPVCYamlFormat(params.Namespace, workload, kubeClient)
					if len(bs) == 0 || err != nil {
						log.Errorf("not found yaml %v", err)
						return fmt.Errorf("get deploy/pvc failed err:%s", err)
					}
					yamls = append(yamls, string(bs))
				}
			default:
				return fmt.Errorf("do not support workload kind:%s", workloadType)
			}
		}
		yaml := util.JoinYamls(yamls)
		serviceParam := &commonmodels.Service{
			ProductName: params.ProductName,
			ServiceName: service.Name,
			Visibility:  params.Visibility,
			Type:        params.Type,
			Yaml:        yaml,
			Source:      setting.SourceFromZadig,
		}
		_, err := CreateServiceTemplate(username, serviceParam, force, proudction, log)
		if err != nil {
			log.Errorf("CreateServiceTemplate error, msg:%s", err)
			return err
		}
	}

	return nil
}

func getServiceVariables(templateProduct *template.Product, product *commonmodels.Product) []*types.ServiceWithVariable {
	ret := make([]*types.ServiceWithVariable, 0)
	for _, svc := range product.GetServiceMap() {
		ret = append(ret, &types.ServiceWithVariable{
			ServiceName: svc.ServiceName,
		})
	}

	if !templateProduct.IsK8sYamlProduct() {
		return ret
	}

	args, _, err := commonservice.GetK8sSvcRenderArgs(product.ProductName, product.EnvName, "", product.Production, log.SugaredLogger())
	if err != nil {
		log.Errorf("failed to get k8s service render args, err: %s", err)
	}
	ret = make([]*types.ServiceWithVariable, 0)
	for _, arg := range args {
		ret = append(ret, &types.ServiceWithVariable{
			ServiceName:  arg.ServiceName,
			VariableYaml: arg.LatestVariableYaml,
			VariableKVs:  arg.LatestVariableKVs,
		})
	}

	return ret
}

func getAllGeneralEnvs(templateProduct *template.Product, production bool) ([]*DeployableEnv, error) {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:                 templateProduct.ProductName,
		ShareEnvEnable:       util.GetBoolPointer(false),
		IstioGrayscaleEnable: util.GetBoolPointer(false),
		Production:           util.GetBoolPointer(production),
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*DeployableEnv, len(envs))

	envNames := make([]string, len(envs))
	for i, env := range envs {

		envNames[i] = env.EnvName
		ret[i] = &DeployableEnv{
			EnvName:           env.EnvName,
			Alias:             env.Alias,
			Namespace:         env.Namespace,
			ClusterID:         env.ClusterID,
			GlobalVariableKVs: env.GlobalVariables,
			Services:          getServiceVariables(templateProduct, env),
		}
	}

	return ret, nil
}

func getDeployableShareEnvs(svcName string, templateProduct *template.Product, production bool) ([]*DeployableEnv, error) {
	projectName := templateProduct.ProjectName
	if !production {
		baseEnvs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:           projectName,
			ShareEnvEnable: util.GetBoolPointer(true),
			ShareEnvIsBase: util.GetBoolPointer(true),
			Production:     util.GetBoolPointer(false),
		})
		if err != nil {
			return nil, err
		}

		ret := make([]*DeployableEnv, 0)
		for _, baseEnv := range baseEnvs {

			ret = append(ret, &DeployableEnv{
				EnvName:           baseEnv.EnvName,
				Alias:             baseEnv.Alias,
				Namespace:         baseEnv.Namespace,
				ClusterID:         baseEnv.ClusterID,
				GlobalVariableKVs: baseEnv.GlobalVariables,
				Services:          getServiceVariables(templateProduct, baseEnv),
			})

			if !hasSvcInEnv(svcName, baseEnv) {
				continue
			}

			subEnvs, err := getSubEnvs(baseEnv.EnvName, templateProduct)
			if err != nil {
				return nil, err
			}

			ret = append(ret, subEnvs...)
		}

		return ret, nil
	} else {
		baseEnvs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:                 projectName,
			IstioGrayscaleEnable: util.GetBoolPointer(true),
			IstioGrayscaleIsBase: util.GetBoolPointer(true),
			Production:           util.GetBoolPointer(true),
		})
		if err != nil {
			return nil, err
		}

		ret := make([]*DeployableEnv, 0)
		for _, baseEnv := range baseEnvs {

			ret = append(ret, &DeployableEnv{
				EnvName:           baseEnv.EnvName,
				Alias:             baseEnv.Alias,
				Namespace:         baseEnv.Namespace,
				ClusterID:         baseEnv.ClusterID,
				GlobalVariableKVs: baseEnv.GlobalVariables,
				Services:          getServiceVariables(templateProduct, baseEnv),
			})

			if !hasSvcInEnv(svcName, baseEnv) {
				continue
			}

			grayEnvs, err := getGrayEnvs(baseEnv.EnvName, baseEnv.ClusterID, templateProduct)
			if err != nil {
				return nil, err
			}

			ret = append(ret, grayEnvs...)
		}

		return ret, nil
	}
}

func getSubEnvs(baseEnvName string, templateProduct *template.Product) ([]*DeployableEnv, error) {
	projectName := templateProduct.ProjectName
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:            projectName,
		ShareEnvEnable:  util.GetBoolPointer(true),
		ShareEnvIsBase:  util.GetBoolPointer(false),
		ShareEnvBaseEnv: util.GetStrPointer(baseEnvName),
		Production:      util.GetBoolPointer(false),
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*DeployableEnv, len(envs))
	for i, env := range envs {
		ret[i] = &DeployableEnv{
			EnvName:           env.EnvName,
			Alias:             env.Alias,
			Namespace:         env.Namespace,
			ClusterID:         env.ClusterID,
			GlobalVariableKVs: env.GlobalVariables,
			Services:          getServiceVariables(templateProduct, env),
		}
	}

	return ret, nil
}

func getGrayEnvs(baseEnvName, clusterID string, templateProduct *template.Product) ([]*DeployableEnv, error) {
	projectName := templateProduct.ProjectName
	envs, err := commonutil.FetchGrayEnvs(context.TODO(), projectName, clusterID, baseEnvName)
	if err != nil {
		return nil, err
	}

	ret := make([]*DeployableEnv, len(envs))
	for i, env := range envs {
		ret[i] = &DeployableEnv{
			EnvName:           env.EnvName,
			Alias:             env.Alias,
			Namespace:         env.Namespace,
			ClusterID:         env.ClusterID,
			GlobalVariableKVs: env.GlobalVariables,
			Services:          getServiceVariables(templateProduct, env),
		}
	}

	return ret, nil
}

func hasSvcInEnv(svcName string, env *commonmodels.Product) bool {
	for _, svcGroup := range env.Services {
		for _, svc := range svcGroup {
			if svc.ServiceName == svcName {
				return true
			}
		}
	}

	return false
}
