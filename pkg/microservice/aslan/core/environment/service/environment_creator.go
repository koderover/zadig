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
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
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

	productObject, err := GetInitProduct(productName, log)
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
	case setting.HelmDeployType:
		return newHelmProductCreator()
	case setting.PMDeployType:
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
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), args.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] GetKubeClient error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddErr(err)
	}

	//判断namespace是否存在
	namespace := args.GetNamespace()
	if args.Namespace == "" {
		args.Namespace = namespace
	}
	_, found, err := getter.GetNamespace(args.Namespace, kubeClient)
	if err != nil {
		log.Errorf("GetNamespace error: %v", err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	if found {
		log.Warnf("%s[%s]%s", "namespace", args.Namespace, "已经存在,请换个环境名称尝试!")
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("%s[%s]%s", "namespace", args.Namespace, "已经存在,请换个环境名称尝试!"))
	}

	restConfig, err := kube.GetRESTConfig(args.ClusterID)
	if err != nil {
		log.Errorf("GetRESTConfig error: %v", err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	helmClient, err := helmclient.NewClientFromRestConf(restConfig, args.Namespace)
	if err != nil {
		log.Errorf("[%s][%s] NewClientFromRestConf error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddErr(err)
	}

	// renderset may exist before product created, by setting values.yaml content
	var renderSet *models.RenderSet
	if args.Render == nil || args.Render.Revision == 0 {
		renderSet, _, err = commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
			Name: args.Namespace,
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
			chartInfoMap := make(map[string]*template.RenderChart)
			for _, renderChart := range renderSet.ChartInfos {
				chartInfoMap[renderChart.ServiceName] = renderChart
			}

			// use values.yaml content from predefined env renderset
			for _, singleRenderChart := range args.ChartInfos {
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

	if renderSet == nil {
		renderSet, err = FindHelmRenderSet(args.ProductName, args.Render.Name, log)
		if err != nil {
			log.Errorf("[%s][P:%s] find product renderset error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
	}

	// 设置产品render revision
	args.Render.Revision = renderSet.Revision
	// 记录服务当前对应render版本
	setServiceRender(args)

	args.Status = setting.ProductStatusCreating
	args.RecycleDay = config.DefaultRecycleDay()
	if args.IsForkedProduct {
		args.RecycleDay = 7
	}
	err = commonrepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	eventStart := time.Now().Unix()

	go installProductHelmCharts(user, args.EnvName, requestID, args, renderSet, eventStart, helmClient, log)
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
	err := commonrepo.NewProductColl().Create(args)
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

	eventStart := time.Now().Unix()

	args.Status = setting.ProductStatusCreating
	args.RecycleDay = config.DefaultRecycleDay()
	err := commonrepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	// 异步创建产品
	go createGroups(args.EnvName, user, requestID, args, eventStart, nil, nil, log)
	return nil
}

type DefaultProductCreator struct {
}

func newDefaultProductCreator() *DefaultProductCreator {
	return &DefaultProductCreator{}
}

func (creator *DefaultProductCreator) Create(user, requestID string, args *models.Product, log *zap.SugaredLogger) error {
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), args.ClusterID)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	//判断namespace是否存在
	namespace := args.GetNamespace()
	if args.Namespace == "" {
		args.Namespace = namespace
	}
	_, found, err := getter.GetNamespace(args.Namespace, kubeClient)
	if err != nil {
		log.Errorf("GetNamespace error: %v", err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	if found {
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("%s[%s]%s", "namespace", args.Namespace, "已经存在,请换个环境名称尝试!"))
	}

	//创建角色环境之间的关联关系
	//todo 创建环境暂时不指定角色
	// 检查是否重复创建（TO BE FIXED）;检查k8s集群设置: Namespace/Secret .etc
	if err := preCreateProduct(args.EnvName, args, kubeClient, log); err != nil {
		log.Errorf("CreateProduct preCreateProduct error: %v", err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	eventStart := time.Now().Unix()
	// 检查renderinfo是否为空，避免空指针
	if args.Render == nil {
		args.Render = &models.RenderInfo{ProductTmpl: args.ProductName}
	}

	renderSet := &models.RenderSet{
		ProductTmpl: args.Render.ProductTmpl,
		Name:        args.Render.Name,
		Revision:    args.Render.Revision,
	}
	// 如果是版本回滚，则args.Render.Revision > 0
	if args.Render.Revision == 0 {
		// 检查renderset是否覆盖产品所有key
		renderSet, err = commonservice.ValidateRenderSet(args.ProductName, args.Render.Name, nil, log)
		if err != nil {
			log.Errorf("[%s][P:%s] validate product renderset error: %v", args.EnvName, args.ProductName, err)
			return e.ErrCreateEnv.AddDesc(err.Error())
		}
		// 保存产品信息,并设置产品状态
		// 设置产品render revsion
		args.Render.Revision = renderSet.Revision
		// 记录服务当前对应render版本
		setServiceRender(args)
	}

	args.Status = setting.ProductStatusCreating
	args.RecycleDay = config.DefaultRecycleDay()
	err = commonrepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("[%s][%s] create product record error: %v", args.EnvName, args.ProductName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	// 异步创建产品
	go createGroups(args.EnvName, user, requestID, args, eventStart, renderSet, kubeClient, log)
	return nil
}
