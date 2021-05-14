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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	templatemodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/template"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	templaterepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

var DefaultCleanWhiteList = []string{"spockadmin"}

func CleanProductCronJob(log *xlog.Logger) {

	log.Info("[CleanProductCronJob] started ...")
	defer log.Info("[CleanProductCronJob] end")

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
	if err != nil {
		log.Errorf("[Product.List] error: %v", err)
		return
	}

	wl := sets.NewString(DefaultCleanWhiteList...)
	wl.Insert(config.CleanSkippedList()...)
	for _, product := range products {
		if wl.Has(product.EnvName) {
			log.Infof("clean up skipped. user %s in whitelist.", product.EnvName)
			continue
		}

		if product.RecycleDay == 0 {
			log.Infof("clean up skipped. user %s 不被回收.", product.EnvName)
			continue
		}

		if time.Now().Unix()-product.UpdateTime > int64(60*60*24*product.RecycleDay) {
			title := "系统清理产品信息"
			content := fmt.Sprintf("产品 [%s] 已经连续%d天没有使用, 系统已自动删除该产品, 如有需要请重新创建产品。", product.ProductName, product.RecycleDay)

			if err := commonservice.DeleteProduct("robot", product.EnvName, product.ProductName, log); err != nil {
				log.Errorf("[%s][P:%s] delete product error: %v", product.EnvName, product.ProductName, err)

				// 如果有错误，重试删除
				if err := commonservice.DeleteProduct("robot", product.EnvName, product.ProductName, log); err != nil {
					content = fmt.Sprintf("产品 [%s] 系统自动清理失败，请手动删除产品。", product.ProductName)
					log.Errorf("[%s][P:%s] retry delete product error: %v", product.EnvName, product.ProductName, err)
				}
			}

			commonservice.SendMessage(product.EnvName, title, content, log)

			log.Warnf("[%s] product %s deleted", product.EnvName, product.ProductName)
		}
	}
}

func GetInitProduct(productTmplName string, log *xlog.Logger) (*commonmodels.Product, error) {
	ret := &commonmodels.Product{}

	prodTmpl, err := templaterepo.NewProductColl().Find(productTmplName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", productTmplName, err)
		log.Error(errMsg)
		return nil, e.ErrGetProduct.AddDesc(errMsg)
	}

	if prodTmpl.ProductFeature == nil || prodTmpl.ProductFeature.DeployType == setting.K8SDeployType {
		err = commonservice.FillProductTemplateVars([]*templatemodels.Product{prodTmpl}, log)
	}
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.FillProductTemplate] %s error: %v", productTmplName, err)
		log.Error(errMsg)
		return nil, e.ErrGetProduct.AddDesc(errMsg)
	}

	//返回中的ProductName即产品模板的名称
	ret.ProductName = prodTmpl.ProductName
	ret.Revision = prodTmpl.Revision
	ret.Enabled = prodTmpl.Enabled
	ret.Services = [][]*commonmodels.ProductService{}
	ret.UpdateBy = prodTmpl.UpdateBy
	ret.CreateTime = prodTmpl.CreateTime
	ret.Visibility = prodTmpl.Visibility
	ret.Render = &commonmodels.RenderInfo{Name: "", Descritpion: ""}
	ret.Vars = prodTmpl.Vars
	ret.ChartInfos = prodTmpl.ChartInfos

	for _, names := range prodTmpl.Services {
		servicesResp := make([]*commonmodels.ProductService, 0)

		for _, serviceName := range names {
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   serviceName,
				ExcludeStatus: setting.ProductStatusDeleting,
			}

			serviceTmpls, err := commonrepo.NewServiceColl().List(opt)
			if err != nil {
				errMsg := fmt.Sprintf("[ServiceTmpl.List] %s error: %v", opt.ServiceName, err)
				log.Error(errMsg)
				return nil, e.ErrGetProduct.AddDesc(errMsg)
			}
			for _, serviceTmpl := range serviceTmpls {
				serviceResp := &commonmodels.ProductService{
					ServiceName: serviceTmpl.ServiceName,
					Type:        serviceTmpl.Type,
					Revision:    serviceTmpl.Revision,
				}
				if serviceTmpl.Type == setting.K8SDeployType {
					serviceResp.Containers = make([]*commonmodels.Container, 0)
					for _, c := range serviceTmpl.Containers {
						container := &commonmodels.Container{
							Name:  c.Name,
							Image: c.Image,
						}
						serviceResp.Containers = append(serviceResp.Containers, container)
					}
				}
				servicesResp = append(servicesResp, serviceResp)
			}
		}
		ret.Services = append(ret.Services, servicesResp)
	}

	return ret, err
}

func GetProduct(username, envName, productName string, log *xlog.Logger) (*ProductResp, error) {
	log.Infof("[User:%s][EnvName:%s][Product:%s] GetProduct", username, envName, productName)
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[User:%s][EnvName:%s][Product:%s] Product.FindByOwner error: %v", username, envName, productName, err)
		return nil, e.ErrGetEnv
	}

	if prod.Source != setting.SourceFromHelm && prod.Source != setting.SourceFromExternal {
		err = FillProductVars([]*commonmodels.Product{prod}, log)
		if err != nil {
			return nil, err
		}
	}

	resp := buildProductResp(prod.EnvName, prod, log)
	return resp, nil
}

func buildProductResp(envName string, prod *commonmodels.Product, log *xlog.Logger) *ProductResp {
	prodResp := &ProductResp{
		ID:          prod.ID.Hex(),
		ProductName: prod.ProductName,
		Namespace:   prod.Namespace,
		Services:    [][]string{},
		Status:      setting.PodUnstable,
		EnvName:     prod.EnvName,
		UpdateTime:  prod.UpdateTime,
		UpdateBy:    prod.UpdateBy,
		Render:      prod.Render,
		Error:       prod.Error,
		Vars:        prod.Vars[:],
		IsPublic:    prod.IsPublic,
		ClusterId:   prod.ClusterId,
		RecycleDay:  prod.RecycleDay,
		Source:      prod.Source,
	}

	if prod.ClusterId != "" {
		clusterService, err := kube.NewService(config.HubServerAddress())
		if err != nil {
			prodResp.Status = setting.ClusterNotFound
			prodResp.Error = "未找到该环境绑定的集群"
			return prodResp
		}
		cluster, err := clusterService.GetCluster(prod.ClusterId, log)
		if err != nil {
			prodResp.Status = setting.ClusterNotFound
			prodResp.Error = "未找到该环境绑定的集群"
			return prodResp
		}
		prodResp.IsProd = cluster.Production

		if !clusterService.ClusterConnected(prod.ClusterId) {
			prodResp.Status = setting.ClusterDisconnected
			prodResp.Error = "集群未连接"
			return prodResp
		}
	}

	if prod.Status == setting.ProductStatusCreating {
		prodResp.Status = setting.PodCreating
		return prodResp
	}
	if prod.Status == setting.ProductStatusUpdating {
		prodResp.Status = setting.PodUpdating
		return prodResp
	}
	if prod.Status == setting.ProductStatusDeleting {
		prodResp.Status = setting.PodDeleting
		return prodResp
	}
	if prod.Status == setting.ProductStatusUnknown {
		prodResp.Status = setting.ClusterUnknown
		return prodResp
	}

	var (
		servicesResp = make([]*commonservice.ServiceResp, 0)
		errObj       error
	)

	prodResp.Services = prod.GetGroupServiceNames()
	servicesResp, _, errObj = ListGroups("", envName, prod.ProductName, -1, -1, log)

	if errObj != nil {
		prodResp.Error = errObj.Error()
	} else {
		allRunning := true
		for _, serviceResp := range servicesResp {
			// Service是物理机部署时，无需判断状态
			if serviceResp.Type == setting.K8SDeployType && serviceResp.Status != setting.PodRunning && serviceResp.Status != setting.PodSucceeded {
				allRunning = false
				break
			}
		}

		if allRunning {
			prodResp.Status = setting.PodRunning
			prodResp.Error = ""
		}
	}

	return prodResp
}

func CleanProducts() {
	logger := xlog.NewDummy()

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
	if err != nil {
		logger.Errorf("ListProducts error: %v\n", err)
		return
	}

	for _, prod := range products {
		_, err := templaterepo.NewProductColl().Find(prod.ProductName)
		if err != nil && err.Error() == "not found" {
			logger.Errorf("集成环境所属的项目不存在，准备删除此集成环境, namespace:%s, 项目:%s\n", prod.Namespace, prod.ProductName)
			err = commonservice.DeleteProduct("CleanProducts", prod.EnvName, prod.ProductName, logger)
			if err != nil {
				logger.Errorf("delete product failed, namespace:%s, err:%v\n", prod.Namespace, err)
				continue
			}
		}
	}
}

func ResetProductsStatus() {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
	if err != nil {
		fmt.Printf("ResetProductsStatus error: %v\n", err)
		return
	}

	for _, prod := range products {

		if prod.Status == setting.ProductStatusCreating || prod.Status == setting.ProductStatusUpdating || prod.Status == setting.ProductStatusDeleting {
			if err := commonrepo.NewProductColl().UpdateStatus(prod.EnvName, prod.ProductName, setting.ProductStatusFailed); err != nil {
				fmt.Printf("update product status error: %v\n", err)
			}
		}
	}
}
