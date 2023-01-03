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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type CreateProductParam struct {
	UserName    string
	RequestId   string
	ProductName string
	EnvType     string
	log         *zap.SugaredLogger
	RegistryID  string
}

type AutoCreator struct {
	Param *CreateProductParam
}

type IProductCreator interface {
	Create(string, string, *models.Product, *zap.SugaredLogger) error
}

func autoCreateProduct(envType, envName, productName, requestId, userName string, log *zap.SugaredLogger) (string, error) {
	autoCreator := &AutoCreator{Param: &CreateProductParam{
		UserName:    userName,
		RequestId:   requestId,
		ProductName: productName,
		EnvType:     envType,
		log:         log,
	}}
	return autoCreator.Create(envName)
}

func (autoCreator *AutoCreator) Create(envName string) (string, error) {
	productName, log := autoCreator.Param.ProductName, autoCreator.Param.log

	// find product in db
	productResp, err := GetProduct(autoCreator.Param.UserName, envName, productName, log)
	if err == nil && productResp != nil { // product exists in db
		if productResp.Error != "" {
			return setting.ProductStatusFailed, errors.New(productResp.Error)
		}
		return productResp.Status, nil
	}

	productObject, err := GetInitProduct(productName, types.GeneralEnv, false, "", log)
	if err != nil {
		log.Errorf("AutoCreateProduct err:%v", err)
		return "", err
	}

	productObject.IsPublic = true
	productObject.Namespace = commonservice.GetProductEnvNamespace(envName, productName, "")
	productObject.UpdateBy = autoCreator.Param.UserName
	productObject.EnvName = envName
	productObject.RegistryID = autoCreator.Param.RegistryID
	if autoCreator.Param.EnvType == setting.HelmDeployType {
		productObject.Source = setting.SourceFromHelm
	}

	err = CreateProduct(autoCreator.Param.UserName, autoCreator.Param.RequestId, productObject, log)
	if err != nil {
		_, messageMap := e.ErrorMessage(err)
		if errMessage, isExist := messageMap["description"]; isExist {
			if message, ok := errMessage.(string); ok {
				return setting.ProductStatusFailed, errors.New(message)
			}
		}
	}
	return setting.ProductStatusCreating, nil
}

func getCreatorBySource(source string) IProductCreator {
	switch source {
	case setting.SourceFromExternal:
		return newExternalProductCreator()
	case setting.SourceFromHelm:
		return newHelmProductCreator()
	case setting.SourceFromPM:
		return newPMProductCreator()
	default:
		return newDefaultProductCreator()
	}
}

type HelmProductCreator struct{}

func newHelmProductCreator() *HelmProductCreator {
	return &HelmProductCreator{}
}

func (creator *HelmProductCreator) Create(user, requestID string, args *models.Product, log *zap.SugaredLogger) error {
	clusterID := args.ClusterID
	if clusterID == "" {
		projectClusterRelations, err := commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{
			ProjectName: args.ProductName,
		})
		if err != nil {
			return e.ErrCreateEnv.AddDesc("Failed to get the cluster info selected in the project")
		}
		if len(projectClusterRelations) > 0 {
			clusterID = projectClusterRelations[0].ClusterID
		}
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("[%s][%s] GetKubeClient error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddErr(err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	//判断namespace是否存在
	namespace := args.GetNamespace()
	if args.Namespace == "" {
		args.Namespace = namespace
	}

	helmClient, err := helmtool.NewClientFromNamespace(args.ClusterID, args.Namespace)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	// renderset may exist before product created, by setting values.yaml content
	var renderSet *models.RenderSet
	if args.Render == nil || args.Render.Revision == 0 {
		renderSet, _, err = commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
			EnvName:     args.EnvName,
			Name:        args.Namespace,
			ProductTmpl: args.ProductName,
		})

		if err != nil {
			log.Errorf("[%s][P:%s] find product renderset error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		// if env renderset is predefined, set render info
		if renderSet != nil {
			args.Render = &models.RenderInfo{
				ProductTmpl: args.ProductName,
				Name:        renderSet.Name,
				Revision:    renderSet.Revision,
			}
			// user renderchart from renderset
			chartInfoMap := make(map[string]*template.ServiceRender)
			for _, renderChart := range renderSet.ChartInfos {
				chartInfoMap[renderChart.ServiceName] = renderChart
			}

			// use values.yaml content from predefined env renderset
			for _, singleRenderChart := range args.ServiceRenders {
				if renderInEnvRenderset, ok := chartInfoMap[singleRenderChart.ServiceName]; ok {
					singleRenderChart.OverrideValues = renderInEnvRenderset.OverrideValues
					singleRenderChart.OverrideYaml = renderInEnvRenderset.OverrideYaml
				}
			}
		}
	}

	if err = preCreateProduct(args.EnvName, args, kubeClient, log); err != nil {
		log.Errorf("CreateProduct preCreateProduct error: %v", err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	renderSet, err = FindProductRenderSet(args.ProductName, args.Render.Name, args.EnvName, log)
	if err != nil {
		log.Errorf("[%s][P:%s] find product renderset error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	// before create product, do install -dryRun to expose errors earlier
	err = dryRunInstallRelease(args, renderSet, helmClient, log)
	if err != nil {
		log.Errorf("error occurred when installing services in env: %s/%s, err: %s ", args.ProductName, args.EnvName, err)
		return e.ErrCreateEnv.AddErr(err)
	}

	dryRunClient := client.NewDryRunClient(kubeClient)
	err = initEnvConfigSetAction(args.EnvName, args.Namespace, args.ProductName, user, args.EnvConfigs, true, dryRunClient)
	if err != nil {
		log.Errorf("failed to dyrRun env resource [%s][P:%s], the error is: %s", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddErr(err)
	}

	args.Render.Revision = renderSet.Revision
	args.Status = setting.ProductStatusCreating
	args.RecycleDay = config.DefaultRecycleDay()
	args.ClusterID = clusterID
	if args.IsForkedProduct {
		args.RecycleDay = 7
	}
	err = commonrepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	err = initEnvConfigSetAction(args.EnvName, args.Namespace, args.ProductName, user, args.EnvConfigs, false, kubeClient)
	if err != nil {
		log.Errorf("failed to helmInitEnvConfigSet [%s][P:%s], the error is: %s", args.EnvName, args.ProductName, err)
		if err := commonrepo.NewProductColl().UpdateStatusAndError(args.EnvName, args.ProductName, setting.ProductStatusFailed, err.Error()); err != nil {
			log.Errorf("helmInitEnvConfigSet [%s][P:%s] Product.UpdateStatus error: %s", args.EnvName, args.ProductName, err)
		}
	}

	go installProductHelmCharts(user, requestID, args, renderSet, time.Now().Unix(), helmClient, kubeClient, istioClient, log)
	return nil
}

type ExternalProductCreator struct {
}

func newExternalProductCreator() *ExternalProductCreator {
	return &ExternalProductCreator{}
}

func (creator *ExternalProductCreator) Create(user, requestID string, args *models.Product, log *zap.SugaredLogger) error {
	args.Status = setting.ProductStatusUnstable
	args.RecycleDay = config.DefaultRecycleDay()

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), args.ClusterID)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}
	err = kube.EnsureNamespaceLabels(args.Namespace, map[string]string{setting.ProductLabel: args.ProductName}, kubeClient)
	if err != nil {
		log.Errorf("[%s][%s] create add namesapce label error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	err = commonrepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	return nil
}

type PMProductCreator struct {
}

func newPMProductCreator() *PMProductCreator {
	return &PMProductCreator{}
}

func (creator *PMProductCreator) Create(user, requestID string, args *models.Product, log *zap.SugaredLogger) error {
	//创建角色环境之间的关联关系
	//todo 创建环境暂时不指定角色
	// 检查是否重复创建（TO BE FIXED）;检查k8s集群设置: Namespace/Secret .etc
	if err := preCreateProduct(args.EnvName, args, nil, log); err != nil {
		log.Errorf("CreateProduct preCreateProduct error: %v", err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	args.Status = setting.ProductStatusCreating
	args.RecycleDay = config.DefaultRecycleDay()
	err := commonrepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	// 异步创建产品
	go createGroups(user, requestID, args, time.Now().Unix(), nil, nil, nil, nil, log)
	return nil
}

type K8sYamlProductCreator struct {
}

func newDefaultProductCreator() *K8sYamlProductCreator {
	return &K8sYamlProductCreator{}
}

func dryRunServices(args *commonmodels.Product, renderSet *commonmodels.RenderSet, informer informers.SharedInformerFactory, kubeClient client.Client, log *zap.SugaredLogger) error {
	errList := &multierror.Error{}
	for _, group := range args.Services {
		for _, svc := range group {
			if !commonutil.ServiceDeployed(svc.ServiceName, args.ServiceDeployStrategy) {
				continue
			}
			_, err := upsertService(args, svc, nil, renderSet, args.Render, informer, kubeClient, nil, log)
			if err != nil {
				errList = multierror.Append(errList, fmt.Errorf("failed to dryRun apply service: %s, err: %s", svc.ServiceName, err))
			}
		}
	}
	return errList.ErrorOrNil()
}

func (creator *K8sYamlProductCreator) Create(user, requestID string, args *models.Product, log *zap.SugaredLogger) error {
	// get project cluster relation
	clusterID := args.ClusterID
	if clusterID == "" {
		projectClusterRelations, err := commonrepo.NewProjectClusterRelationColl().List(&commonrepo.ProjectClusterRelationOption{
			ProjectName: args.ProductName,
		})
		if err != nil {
			return e.ErrCreateEnv.AddDesc("Failed to get the cluster info selected in the project")
		}
		if len(projectClusterRelations) > 0 {
			clusterID = projectClusterRelations[0].ClusterID
		}
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}
	inf, err := informer.NewInformer(clusterID, args.Namespace, cls)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}
	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to new istio client: %s", err)
	}

	//判断namespace是否存在
	namespace := args.GetNamespace()
	if args.Namespace == "" {
		args.Namespace = namespace
	}

	var renderSet *models.RenderSet
	if args.Render == nil || args.Render.Revision == 0 {
		renderSet, _, err = commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
			EnvName:     args.EnvName,
			Name:        args.Namespace,
			ProductTmpl: args.ProductName,
		})

		if err != nil {
			log.Errorf("[%s][P:%s] find product renderset error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		// if env renderset is predefined, set render info
		if renderSet != nil {
			args.Render = &models.RenderInfo{
				ProductTmpl: args.ProductName,
				Name:        renderSet.Name,
				Revision:    renderSet.Revision,
			}
			//// user renderchart from renderset
			//chartInfoMap := make(map[string]*template.ServiceRender)
			//for _, renderChart := range renderSet.ChartInfos {
			//	chartInfoMap[renderChart.ServiceName] = renderChart
			//}
			//
			//// use values.yaml content from predefined env renderset
			//for _, singleRenderChart := range args.ServiceRenders {
			//	if renderInEnvRenderset, ok := chartInfoMap[singleRenderChart.ServiceName]; ok {
			//		singleRenderChart.OverrideValues = renderInEnvRenderset.OverrideValues
			//		singleRenderChart.OverrideYaml = renderInEnvRenderset.OverrideYaml
			//	}
			//}
		}
	}

	//创建角色环境之间的关联关系
	//todo 创建环境暂时不指定角色
	// 检查是否重复创建（TO BE FIXED）;检查k8s集群设置: Namespace/Secret .etc
	if err := preCreateProduct(args.EnvName, args, kubeClient, log); err != nil {
		log.Errorf("CreateProduct preCreateProduct error: %v", err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	renderSet, _, err = commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
		EnvName:  args.EnvName,
		Name:     args.Render.Name,
		Revision: args.Render.Revision,
	})
	if err != nil {
		return e.ErrCreateEnv.AddErr(fmt.Errorf("failed to find renderset: %v/%v", args.Render.Name, args.Render.Revision))
	}
	//// 如果是版本回滚，则args.Render.Revision > 0
	//if args.Render.Revision == 0 {
	//	// 检查renderset是否覆盖产品所有key
	//	renderSet, err = commonservice.ValidateRenderSet(args.ProductName, args.Render.Name, args.EnvName, nil, log)
	//	if err != nil {
	//		log.Errorf("[%s][P:%s] validate product renderset error: %v", args.EnvName, args.ProductName, err)
	//		return e.ErrCreateEnv.AddDesc(err.Error())
	//	}
	//	args.Render.Revision = renderSet.Revision
	//}

	// before we apply yaml to k8s, we run kubectl apply --dry-run to expose problems early
	dryRunClient := client.NewDryRunClient(kubeClient)
	err = dryRunServices(args, renderSet, inf, dryRunClient, log)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	err = initEnvConfigSetAction(args.EnvName, args.Namespace, args.ProductName, user, args.EnvConfigs, true, dryRunClient)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	args.Status = setting.ProductStatusCreating
	args.RecycleDay = config.DefaultRecycleDay()
	args.ClusterID = clusterID
	err = commonrepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	go createGroups(user, requestID, args, time.Now().Unix(), renderSet, inf, kubeClient, istioClient, log)
	return nil
}

func initEnvConfigSetAction(envName, namespace, productName, userName string, envResources []*models.CreateUpdateCommonEnvCfgArgs, dryRun bool, kubeClient client.Client) error {
	errList := &multierror.Error{
		ErrorFormat: func(es []error) string {
			format := "创建环境配置"
			if len(es) == 1 {
				return fmt.Sprintf(format+" %s 失败:%s", envName, es[0])
			}
			points := make([]string, len(es))
			for i, err := range es {
				points[i] = fmt.Sprintf("* %s", err)
			}
			return fmt.Sprintf(format+" %s 失败:\n%s", envName, strings.Join(points, "\n"))
		},
	}

	clusterLabels := getPredefinedClusterLabels(productName, "", envName)
	delete(clusterLabels, "s-service")

	for _, envResource := range envResources {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(envResource.YamlData))
		if err != nil {
			log.Errorf("Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %s", envResource.YamlData, err)
			errList = multierror.Append(errList, err)
			continue
		}
		switch u.GetKind() {
		case setting.ConfigMap, setting.Ingress, setting.Secret, setting.PersistentVolumeClaim:
			ls := kube.MergeLabels(clusterLabels, u.GetLabels())
			u.SetNamespace(namespace)
			u.SetLabels(ls)
			_, err := ensureLabelAndNs(u, namespace, productName)
			if err != nil {
				errList = multierror.Append(errList, err)
				continue
			}

			err = updater.CreateOrPatchUnstructuredNeverAnnotation(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to initEnvConfigSet %s, manifest is\n%v\n, error: %s", u.GetKind(), u, err)
				errList = multierror.Append(errList, err)
				continue
			}

			if dryRun {
				continue
			}

			u.SetManagedFields(nil)
			yamlData, err := yaml.Marshal(u.UnstructuredContent())
			if err != nil {
				log.Errorf("Failed to initEnvConfigSet yaml.Marshal %s, manifest is\n%v\n, error: %s", u.GetKind(), u, err)
				errList = multierror.Append(errList, err)
				continue
			}
			envResourceObj := &models.EnvResource{
				ProductName:    productName,
				UpdateUserName: userName,
				EnvName:        envName,
				Namespace:      namespace,
				Name:           u.GetName(),
				YamlData:       string(yamlData),
				SourceDetail:   geneSourceDetail(envResource.GitRepoConfig),
				AutoSync:       envResource.AutoSync,
			}

			switch u.GetKind() {
			case setting.ConfigMap:
				envResourceObj.Type = string(config.CommonEnvCfgTypeConfigMap)
			case setting.Ingress:
				envResourceObj.Type = string(config.CommonEnvCfgTypeIngress)
			case setting.Secret:
				envResourceObj.Type = string(config.CommonEnvCfgTypeSecret)
			case setting.PersistentVolumeClaim:
				envResourceObj.Type = string(config.CommonEnvCfgTypePvc)
			}
			if err := commonrepo.NewEnvResourceColl().Create(envResourceObj); err != nil {
				errList = multierror.Append(errList, err)
			}
		default:
			errList = multierror.Append(errList, fmt.Errorf("Failed to initEnvConfigSet %s, manifest is\n%v\n, error: %s", u.GetKind(), u, "kind not support"))
		}
	}

	return errList.ErrorOrNil()
}
