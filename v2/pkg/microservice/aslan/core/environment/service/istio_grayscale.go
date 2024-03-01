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

package service

import (
	"context"
	"fmt"

	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util/boolptr"
)

func EnableIstioGrayscale(ctx context.Context, envName, productName string) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err)
	}
	if prod.IsSleeping() {
		return fmt.Errorf("Environment is sleeping")
	}

	ns := prod.Namespace
	clusterID := prod.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrEnableIstioGrayscale.AddErr(fmt.Errorf("failed to get kube client: %s", err))
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrEnableIstioGrayscale.AddErr(fmt.Errorf("failed to get rest config: %s", err))
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrEnableIstioGrayscale.AddErr(fmt.Errorf("failed to new istio client: %s", err))
	}

	// 1. Ensure `istio-injection=enabled` label on the namespace.
	err = ensureIstioLabel(ctx, kclient, ns)
	if err != nil {
		return e.ErrEnableIstioGrayscale.AddErr(fmt.Errorf("failed to ensure istio label on namespace `%s`: %s", ns, err))
	}

	// 2. Ensure Pods that are not injected with `istio-proxy`.
	err = ensurePodsWithIsitoProxy(ctx, kclient, ns)
	if err != nil {
		return e.ErrEnableIstioGrayscale.AddErr(fmt.Errorf("failed to ensure pods with istio-proxy in namespace `%s`: %s", ns, err))
	}

	// 3. Ensure `EnvoyFilter` in istio namespace.
	err = ensureEnvoyFilter(ctx, istioClient, clusterID, istioNamespace, zadigEnvoyFilter, nil)
	if err != nil {
		return e.ErrEnableIstioGrayscale.AddErr(fmt.Errorf("failed to ensure EnvoyFilter in namespace `%s`: %s", istioNamespace, err))
	}

	// 4. Update the environment configuration.
	err = commonutil.EnsureIstioGrayConfig(ctx, prod)
	if err != nil {
		return e.ErrEnableIstioGrayscale.AddErr(fmt.Errorf("failed to ensure istio gray config: %s", err))
	}

	return nil
}

func DisableIstioGrayscale(ctx context.Context, envName, productName string) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err))
	}
	if prod.IsSleeping() {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("Environment is sleeping"))
	}

	ns := prod.Namespace
	clusterID := prod.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to get kube client: %s", err))
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to get rest config: %s", err))
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to new istio client: %s", err))
	}

	// 1. Delete all associated gray environments.
	err = ensureDeleteGrayscaleAssociatedEnvs(ctx, prod, kclient, istioClient)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to delete associated gray environments of base ns `%s`: %s", ns, err))
	}

	// 2. Delete EnvoyFilter in the namespace of Istio installation.
	err = ensureDeleteEnvoyFilter(ctx, prod, istioClient)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to delete EnvoyFilter: %s", err))
	}

	// 3. Delete Gateway delivered by the Zadig.
	err = deleteGateways(ctx, kclient, istioClient, ns)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to delete EnvoyFilter: %s", err))
	}

	// 4. Delete all VirtualServices delivered by the Zadig.
	err = deleteVirtualServices(ctx, kclient, istioClient, ns)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to delete VirtualServices that Zadig created in ns `%s`: %s", ns, err))
	}

	// 5. Remove the `istio-injection=enabled` label of the namespace.
	err = removeIstioLabel(ctx, kclient, ns)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to remove istio label on ns `%s`: %s", ns, err))
	}

	// 6. Restart the istio-Proxy injected Pods.
	err = removePodsIstioProxy(ctx, kclient, ns)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to remove istio-proxy from pods in ns `%s`: %s", ns, err))
	}

	// 7. Update the environment configuration.
	err = ensureDisableGrayscaleEnvConfig(ctx, prod)
	if err != nil {
		return e.ErrDisableIstioGrayscale.AddErr(fmt.Errorf("failed to ensure disable istio gray config: %s", err))
	}

	return nil
}

func CheckIstioGrayscaleReady(ctx context.Context, envName, op, productName string) (*IstioGrayscaleReady, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, e.ErrCheckIstioGrayscale.AddErr(fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err))
	}

	ns := prod.Namespace
	clusterID := prod.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return nil, e.ErrCheckIstioGrayscale.AddErr(fmt.Errorf("failed to get kube client: %s", err))
	}

	shareEnvOp := ShareEnvOp(op)

	// 1. Check whether namespace has labeled `istio-injection=enabled`.
	isNamespaceHasIstioLabel, err := checkIstioLabel(ctx, kclient, ns)
	if err != nil {
		return nil, e.ErrCheckIstioGrayscale.AddErr(fmt.Errorf("failed to check whether namespace `%s` has labeled `istio-injection=enabled`: %s", ns, err))
	}

	// 2. Check whether all workloads have K8s Service.
	workloadsHaveNoK8sService, err := checkWorkloadsHaveK8sService(ctx, kclient, ns)
	if err != nil {
		return nil, e.ErrCheckIstioGrayscale.AddErr(fmt.Errorf("failed to check whether all workloads in ns `%s` have K8s Service: %s", ns, err))
	}

	isWorkloadsHaveNoK8sService := true
	if len(workloadsHaveNoK8sService) > 0 {
		isWorkloadsHaveNoK8sService = false
	}

	// 3. Check whether all Pods have istio-proxy and are ready.
	allHaveIstioProxy, allPodsReady, err := checkPodsWithIstioProxyAndReady(ctx, kclient, ns)
	if err != nil {
		return nil, e.ErrCheckIstioGrayscale.AddErr(fmt.Errorf("failed to check whether all pods in ns `%s` have istio-proxy and are ready: %s", ns, err))
	}

	res := &IstioGrayscaleReady{
		Checks: IstioGrayscaleChecks{
			NamespaceHasIstioLabel:  isNamespaceHasIstioLabel,
			WorkloadsHaveK8sService: isWorkloadsHaveNoK8sService,
			PodsHaveIstioProxy:      allHaveIstioProxy,
			WorkloadsReady:          allPodsReady,
		},
	}
	res.CheckAndSetReady(shareEnvOp)

	return res, nil
}

func GetIstioGrayscaleConfig(ctx context.Context, envName, productName string) (commonmodels.IstioGrayscale, error) {
	resp := commonmodels.IstioGrayscale{}
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: boolptr.True()}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return resp, e.ErrGetIstioGrayscaleConfig.AddErr(fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err))
	}
	if !prod.IstioGrayscale.IsBase {
		return resp, e.ErrGetIstioGrayscaleConfig.AddErr(fmt.Errorf("cannot get istio grayscale config from gray environment %s/%s", productName, envName))
	}

	return prod.IstioGrayscale, nil
}

type SetIstioGrayscaleConfigRequest struct {
	GrayscaleStrategy  commonmodels.GrayscaleStrategyType    `bson:"grayscale_strategy" json:"grayscale_strategy"`
	WeightConfigs      []commonmodels.IstioWeightConfig      `bson:"weight_configs" json:"weight_configs"`
	HeaderMatchConfigs []commonmodels.IstioHeaderMatchConfig `bson:"header_match_configs" json:"header_match_configs"`
}

func SetIstioGrayscaleConfig(ctx context.Context, envName, productName string, req SetIstioGrayscaleConfigRequest) error {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: boolptr.True()}
	baseEnv, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("failed to query env `%s` in project `%s`: %s", envName, productName, err))
	}
	if !baseEnv.IstioGrayscale.IsBase {
		return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("cannot set istio grayscale config for gray environment"))
	}
	if baseEnv.IsSleeping() {
		return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("Environment %s is sleeping", baseEnv.EnvName))
	}

	grayEnvs, err := commonutil.FetchGrayEnvs(ctx, baseEnv.ProductName, baseEnv.ClusterID, baseEnv.EnvName)
	if err != nil {
		return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("failed to list gray environments of %s/%s: %s", baseEnv.ProductName, baseEnv.EnvName, err))
	}

	envMap := map[string]*commonmodels.Product{
		baseEnv.EnvName: baseEnv,
	}
	for _, env := range grayEnvs {
		if env.IsSleeping() {
			return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("Environment %s is sleeping", baseEnv.EnvName))
		}
		envMap[env.EnvName] = env
	}

	if req.GrayscaleStrategy == commonmodels.GrayscaleStrategyWeight {
		err = kube.SetIstioGrayscaleWeight(context.TODO(), envMap, req.WeightConfigs)
		if err != nil {
			return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("failed to set istio grayscale weight, err: %w", err))
		}
	} else if req.GrayscaleStrategy == commonmodels.GrayscaleStrategyHeaderMatch {
		err = kube.SetIstioGrayscaleHeaderMatch(context.TODO(), envMap, req.HeaderMatchConfigs)
		if err != nil {
			return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("failed to set istio grayscale weight, err: %w", err))
		}

		headerKeys := []string{}
		for _, headerMatchConfig := range req.HeaderMatchConfigs {
			for _, headerMatch := range headerMatchConfig.HeaderMatchs {
				headerKeys = append(headerKeys, headerMatch.Key)
			}
		}

		err = reGenerateEnvoyFilter(ctx, baseEnv.ClusterID, headerKeys)
		if err != nil {
			return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("failed to re-generate envoy filter, err: %w", err))
		}
	} else {
		return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("unsupported grayscale strategy type: %s", req.GrayscaleStrategy))
	}

	baseEnv.IstioGrayscale.GrayscaleStrategy = req.GrayscaleStrategy
	baseEnv.IstioGrayscale.WeightConfigs = req.WeightConfigs
	baseEnv.IstioGrayscale.HeaderMatchConfigs = req.HeaderMatchConfigs

	err = commonrepo.NewProductColl().UpdateIstioGrayscale(baseEnv.EnvName, baseEnv.ProductName, baseEnv.IstioGrayscale)
	if err != nil {
		return e.ErrSetIstioGrayscaleConfig.AddErr(fmt.Errorf("failed to update istio grayscale config of %s/%s environment: %s", baseEnv.ProductName, baseEnv.EnvName, err))
	}

	return nil
}

func GetIstioGrayscalePortalService(ctx context.Context, productName, envName, serviceName string) (GetPortalServiceResponse, error) {
	resp := GetPortalServiceResponse{}
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return resp, e.ErrGetIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to query env %s of product %s: %s", envName, productName, err))
	}
	if !env.IstioGrayscale.Enable && !env.IstioGrayscale.IsBase {
		return resp, e.ErrGetIstioGrayscalePortalService.AddDesc("%s doesn't enable share environment or is not base environment")
	}

	ns := env.Namespace
	clusterID := env.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return resp, e.ErrGetIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to get kube client: %s", err))
	}
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return resp, e.ErrGetIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to get rest config: %s", err))
	}
	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return resp, e.ErrGetIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to new istio client: %s", err))
	}

	gatewayName := commonutil.GenIstioGatewayName(serviceName)
	resp.DefaultGatewayAddress, err = getDefaultIstioIngressGatewayAddress(ctx, serviceName, gatewayName, err, kclient)
	if err != nil {
		return resp, e.ErrGetIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to get default istio ingress gateway address: %s", err))
	}

	resp.Servers, err = getIstioGatewayConfig(ctx, istioClient, ns, gatewayName)
	if err != nil {
		return resp, e.ErrGetIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to get istio gateway config: %s", err))
	}

	return resp, nil
}

func SetupIstioGrayscalePortalService(ctx context.Context, productName, envName, serviceName string, servers []SetupPortalServiceRequest) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to query env %s of product %s: %s", envName, productName, err))
	}
	if !env.IstioGrayscale.Enable && !env.IstioGrayscale.IsBase {
		return e.ErrSetupIstioGrayscalePortalService.AddDesc("%s doesn't enable share environment or is not base environment")
	}

	templateProd, err := template.NewProductColl().Find(productName)
	if err != nil {
		return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to find template product %s, err: %w", productName, err))
	}

	ns := env.Namespace
	clusterID := env.ClusterID

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to get kube client: %s", err))
	}
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to get rest config: %s", err))
	}
	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to new istio client: %s", err))
	}
	gatewayName := commonutil.GenIstioGatewayName(serviceName)

	if len(servers) == 0 {
		// delete operation
		err = cleanIstioIngressGatewayService(ctx, err, istioClient, ns, gatewayName, kclient)
		if err != nil {
			return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to clean istio ingress gateway service, err: %w", err))
		}
	} else {
		// 1. check istio ingress gateway whether has load balancing
		gatewaySvc, err := checkIstioIngressGatewayLB(ctx, err, kclient)
		if err != nil {
			return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to check istio ingress gateway's load balancing, err: %w", err))
		}

		// 2. create gateway for the service
		err = createIstioGateway(ctx, istioClient, ns, gatewayName, servers)
		if err != nil {
			return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to create gateway %s in namespace %s, err: %w", gatewayName, ns, err))
		}

		// 3. patch istio-ingressgateway service's port
		err = patchIstioIngressGatewayServicePort(ctx, gatewaySvc, servers, kclient)
		if err != nil {
			return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to patch istio ingress gateway service's port, err: %w", err))
		}
	}

	// 4. change the related virtualservice
	svcs := []*corev1.Service{}
	deployType := templateProd.ProductFeature.GetDeployType()
	if deployType == setting.K8SDeployType {
		svcs, err = parseK8SProjectServices(ctx, env, serviceName, kclient, ns)
		if err != nil {
			return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to parse k8s project services, err: %w", err))
		}
	} else if deployType == setting.HelmDeployType {
		svcs, err = parseHelmProjectServices(ctx, restConfig, env, envName, productName, serviceName, kclient, ns)
		if err != nil {
			return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to parse helm project services, err: %w", err))
		}
	}

	for _, svc := range svcs {
		err = updateVirtualServiceForPortalService(ctx, svc, istioClient, ns, servers, gatewayName)
		if err != nil {
			return e.ErrSetupIstioGrayscalePortalService.AddErr(fmt.Errorf("failed to update virtualservice for portal service, err: %w", err))
		}
	}

	return nil
}

func ensureDeleteGrayscaleAssociatedEnvs(ctx context.Context, baseProduct *commonmodels.Product, kclient client.Client, istioClient *versionedclient.Clientset) error {
	envs, err := commonutil.FetchGrayEnvs(ctx, baseProduct.ProductName, baseProduct.ClusterID, baseProduct.EnvName)
	if err != nil {
		return fmt.Errorf("failed to list gray environments of %s/%s, err: %w", baseProduct.ProductName, baseProduct.EnvName, err)
	}
	logger := log.SugaredLogger()
	for _, env := range envs {
		err := DeleteProductionProduct("system", env.EnvName, env.ProductName, "", logger)
		if err != nil {
			err = fmt.Errorf("failed to delete gray environment %s/%s, err: %w", env.ProductName, env.EnvName, err)
			log.Error(err)
			return err
		}
	}

	return nil
}

func ensureDisableGrayscaleEnvConfig(ctx context.Context, baseEnv *commonmodels.Product) error {
	if !baseEnv.IstioGrayscale.Enable && !baseEnv.IstioGrayscale.IsBase {
		return nil
	}

	baseEnv.IstioGrayscale = commonmodels.IstioGrayscale{
		Enable: false,
		IsBase: false,
	}

	return commonrepo.NewProductColl().Update(baseEnv)
}

func reGenerateEnvoyFilter(ctx context.Context, clusterID string, headerKeys []string) error {
	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	err = deleteEnvoyFilter(ctx, istioClient, istioNamespace, zadigEnvoyFilter)
	if err != nil {
		return fmt.Errorf("failed to delete EnvoyFilter: %s", err)
	}

	err = ensureEnvoyFilter(ctx, istioClient, clusterID, istioNamespace, zadigEnvoyFilter, headerKeys)
	if err != nil {
		return fmt.Errorf("failed to ensure EnvoyFilter in namespace `%s`: %s", istioNamespace, err)
	}

	return nil
}
