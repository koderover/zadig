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
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commontpl "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type ListConfigMapArgs struct {
	EnvName     string `json:"env_name"`
	ProductName string `json:"product_name"`
	ServiceName string `json:"service_name"`
}

type RollBackConfigMapArgs struct {
	EnvName          string `json:"env_name"`
	ProductName      string `json:"product_name"`
	ServiceName      string `json:"service_name"`
	SrcConfigName    string `json:"src_config_name"`
	DestinConfigName string `json:"destin_config_name"`
}

type UpdateConfigMapArgs struct {
	EnvName              string   `json:"env_name"`
	ProductName          string   `json:"product_name"`
	ServiceName          string   `json:"service_name"`
	ConfigName           string   `json:"config_name"`
	YamlData             string   `json:"yaml_data"`
	RestartAssociatedSvc bool     `json:"restart_associated_svc"`
	Services             []string `json:"services"`
}

type ListConfigMapRes struct {
	*ResourceResponseBase
	CmName    string            `json:"cm_name"`
	Immutable bool              `json:"immutable"`
	CmData    map[string]string `json:"cm_data"`
}

func ListConfigMaps(args *ListConfigMapArgs, log *zap.SugaredLogger) ([]*ListConfigMapRes, error) {
	selector := labels.Set{}.AsSelector()
	// Note. when listing configs from [配置管理] on workload page, service name will be passed as query condition
	if args.ServiceName != "" {
		selector = labels.Set{setting.ProductLabel: args.ProductName, setting.ServiceLabel: args.ServiceName}.AsSelector()
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return nil, e.ErrListConfigMaps.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return nil, e.ErrListConfigMaps.AddErr(err)
	}

	cms, err := getter.ListConfigMaps(product.Namespace, selector, kubeClient)
	if err != nil {
		log.Error(err)
		return nil, e.ErrListConfigMaps.AddDesc(err.Error())
	}

	envSvcDepends, err := commonrepo.NewEnvSvcDependColl().List(&commonrepo.ListEnvSvcDependOption{ProductName: args.ProductName, EnvName: args.EnvName})
	if err != nil && commonrepo.IsErrNoDocuments(err) {
		return nil, e.ErrListResources.AddDesc(err.Error())
	}

	var res []*ListConfigMapRes
	var mutex sync.Mutex
	var wg sync.WaitGroup
	cmProcess := func(cm *corev1.ConfigMap) error {
		for labelKey, labelVal := range cm.GetLabels() {
			if labelKey == setting.ConfigBackupLabel && labelVal == setting.LabelValueTrue {
				return nil
			}
		}
		cm.SetManagedFields(nil)
		cm.SetResourceVersion("")
		if cm.APIVersion == "" {
			cm.APIVersion = "v1"
		}
		if cm.Kind == "" {
			cm.Kind = "ConfigMap"
		}
		yamlData, err := yaml.Marshal(cm)
		if err != nil {
			return err
		}

		var tempSvcs []string
	LB:
		for _, svcDepend := range envSvcDepends {
			for _, ccm := range svcDepend.ConfigMaps {
				if ccm == cm.Name {
					tempSvcs = append(tempSvcs, svcDepend.ServiceModule)
					continue LB
				}
			}
		}

		immutable := false
		if cm.Immutable != nil {
			immutable = *cm.Immutable
		}
		resElem := &ListConfigMapRes{
			CmName:    cm.Name,
			Immutable: immutable,
			CmData:    cm.Data,
			ResourceResponseBase: &ResourceResponseBase{
				Name:        cm.Name,
				Type:        config.CommonEnvCfgTypeConfigMap,
				EnvName:     args.EnvName,
				ProjectName: args.ProductName,
				YamlData:    string(yamlData),
				Services:    tempSvcs,
				CreateTime:  cm.GetCreationTimestamp().Time,
			},
		}
		resElem.setSourceDetailData(cm)

		mutex.Lock()
		res = append(res, resElem)
		mutex.Unlock()
		return nil
	}

	for _, cmElem := range cms {
		wg.Add(1)
		go func(cm *corev1.ConfigMap) {
			defer wg.Done()
			err := cmProcess(cm)
			if err != nil {
				log.Errorf("ListConfigMaps ns:%s name:%s cmProcess err:%s", cm.Namespace, cm.Name, err)
			}
		}(cmElem)
	}
	wg.Wait()

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].CmName < res[j].CmName
	})
	return res, nil
}

func UpdateConfigMap(args *models.CreateUpdateCommonEnvCfgArgs, userName string, log *zap.SugaredLogger) error {
	cm := &corev1.ConfigMap{}
	err := yaml.Unmarshal([]byte(args.YamlData), cm)
	if err != nil {
		return e.ErrUpdateConfigMap.AddErr(err)
	}
	if cm.Name != args.Name {
		return e.ErrUpdateConfigMap.AddDesc("configMap Yaml Name is incorrect")
	}
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return e.ErrUpdateConfigMap.AddErr(err)
	}

	namespace := product.Namespace

	for key, value := range cm.Data {
		// TODO  need fill variable yaml?
		//for _, kv := range renderSet.KVs {
		//	value = strings.Replace(value, kv.Alias, kv.Value, -1)
		//}
		value = kube.ParseSysKeys(product.Namespace, product.EnvName, product.ProductName, args.ServiceName, value)
		cm.Data[key] = value
	}

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
		return e.ErrUpdateConfigMap.AddErr(err)
	}

	yamlData, err := ensureLabelAndNs(cm, product.Namespace, args.ProductName)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	if err := updater.UpdateConfigMap(namespace, cm, clientset); err != nil {
		log.Error(err)
		return e.ErrUpdateConfigMap.AddDesc(err.Error())
	}
	envCM := &models.EnvResource{
		ProductName:    args.ProductName,
		UpdateUserName: userName,
		EnvName:        args.EnvName,
		Namespace:      product.Namespace,
		Name:           cm.Name,
		YamlData:       yamlData,
		Type:           string(config.CommonEnvCfgTypeConfigMap),
		SourceDetail:   args.SourceDetail,
		AutoSync:       args.AutoSync,
	}
	if err := commonrepo.NewEnvResourceColl().Create(envCM); err != nil {
		return e.ErrUpdateConfigMap.AddDesc(err.Error())
	}
	tplProduct, err := commontpl.NewProductColl().Find(args.ProductName)
	if err != nil {
		return e.ErrUpdateConfigMap.AddDesc(err.Error())
	}
	//TODO: helm not support restart service
	if !args.RestartAssociatedSvc || tplProduct.ProductFeature.DeployType != setting.K8SDeployType {
		return nil
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return e.ErrUpdateConfigMap.AddErr(err)
	}
	if err := restartPod(cm.Name, args.ProductName, args.EnvName, namespace, config.CommonEnvCfgTypeConfigMap, clientset, kubeClient); err != nil {
		return e.ErrRestartService.AddDesc(err.Error())
	}
	return nil
}

func RollBackConfigMap(envName string, args *RollBackConfigMapArgs, userName, userID string, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return e.ErrUpdateConfigMap.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return e.ErrUpdateConfigMap.AddErr(err)
	}

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
		return e.ErrUpdateConfigMap.AddErr(err)
	}

	namespace := product.Namespace
	srcCfg, found, err := getter.GetConfigMap(namespace, args.SrcConfigName, kubeClient)
	if err != nil {
		log.Error(err)
		return e.ErrGetConfigMap.AddDesc(err.Error())
	} else if !found {
		return e.ErrGetConfigMap.AddDesc("source configMap not found")
	}

	destinSrc, found, err := getter.GetConfigMap(namespace, args.DestinConfigName, kubeClient)
	if err != nil {
		log.Error(err)
		return e.ErrGetConfigMap.AddDesc(err.Error())
	} else if !found {
		return e.ErrGetConfigMap.AddDesc("target configMap not found")
	}

	if err := archiveConfigMap(namespace, destinSrc, kubeClient, log); err != nil {
		log.Error(err)
		return err
	}

	destinSrc.Data = srcCfg.Data
	destinSrc.Labels[setting.UpdateBy] = kube.MakeSafeLabelValue(userName)
	destinSrc.Labels[setting.UpdateByID] = userID
	destinSrc.Labels[setting.UpdateTime] = time.Now().Format("20060102150405")
	// 回滚时显示回滚版本的时间
	if updateTime, ok := srcCfg.Labels[setting.UpdateTime]; ok {
		destinSrc.Labels[setting.UpdateTime] = updateTime
	}
	if err := updater.UpdateConfigMap(namespace, destinSrc, clientset); err != nil {
		log.Error(err)
		return e.ErrUpdateConfigMap.AddDesc(err.Error())
	}

	restartArgs := &SvcOptArgs{
		EnvName:     envName,
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
	}

	if err := restartK8sPod(restartArgs, namespace, clientset); err != nil {
		log.Error(err)
		return e.ErrRestartService.AddDesc(err.Error())
	}

	return nil
}

// archiveConfigMap 备份当前configmap，时间戳最小间隔为秒，需要控制每秒只能更新一次configmap, 只保留最近10次配置
func archiveConfigMap(namespace string, cfg *corev1.ConfigMap, kubeClient client.Client, log *zap.SugaredLogger) error {
	archiveLabel := make(map[string]string)

	for k, v := range cfg.Labels {
		archiveLabel[k] = v
	}
	archiveLabel[setting.ConfigBackupLabel] = setting.LabelValueTrue
	archiveLabel[setting.InactiveConfigLabel] = setting.LabelValueTrue
	archiveLabel[setting.OwnerLabel] = cfg.Name

	// 兼容历史数据
	// 如果当前使用的版本的label没有update-time，将创建时间设置为update-time
	if _, ok := archiveLabel[setting.UpdateTime]; !ok {
		archiveLabel[setting.UpdateTime] = cfg.CreationTimestamp.Format("20060102150405")
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-bak-%s", cfg.Name, time.Now().Format("20060102150405")),
			Namespace: cfg.Namespace,
			Labels:    archiveLabel,
		},
		Data: cfg.Data,
	}

	if err := updater.CreateConfigMap(configMap, kubeClient); err != nil {
		log.Error(err)
		return e.ErrCreateConfigMap.AddDesc(err.Error())
	}

	cleanArchiveConfigMap(namespace, configMap.Labels, kubeClient, log)

	return nil
}

func cleanArchiveConfigMap(namespace string, ls map[string]string, kubeClient client.Client, log *zap.SugaredLogger) {
	selector := labels.Set{
		setting.ProductLabel:      ls[setting.ProductLabel],
		setting.ServiceLabel:      ls[setting.ServiceLabel],
		setting.ConfigBackupLabel: ls[setting.ConfigBackupLabel],
	}.AsSelector()

	cms, err := getter.ListConfigMaps(namespace, selector, kubeClient)
	if err != nil {
		log.Errorf("kubeCli.ListConfigMaps error: %v", err)
		return
	}

	sort.SliceStable(cms, func(i, j int) bool { return !cms[i].CreationTimestamp.Before(&cms[j].CreationTimestamp) })
	for k, v := range cms {
		if k < 10 {
			continue
		}

		if err := updater.DeleteConfigMap(namespace, v.Name, kubeClient); err != nil {
			log.Errorf("kubeCli.DeleteConfigMap error: %v", err)
		}
	}
}

type MigrateHistoryConfigMapsRes struct {
}

func MigrateHistoryConfigMaps(envName, productName string, log *zap.SugaredLogger) ([]*models.EnvResource, error) {

	res := make([]*models.EnvResource, 0)
	products := make([]*models.Product, 0)
	var err error
	if productName != "" {
		products, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:          productName,
			EnvName:       envName,
			ExcludeStatus: []string{setting.ProductStatusDeleting, setting.ProductStatusCreating},
		})
	} else {
		products, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			ExcludeStatus: []string{setting.ProductStatusDeleting, setting.ProductStatusCreating},
		})
	}
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	for _, product := range products {
		if product.IsExisted {
			continue
		}
		kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
		if err != nil {
			return nil, e.ErrListResources.AddErr(err)
		}
		cms, err := getter.ListConfigMaps(product.Namespace, labels.Set{setting.ProductLabel: product.ProductName, setting.ConfigBackupLabel: setting.LabelValueTrue}.AsSelector(), kubeClient)
		if err != nil {
			return nil, e.ErrListResources.AddErr(err)
		}

		for _, cm := range cms {
			cmNames := strings.Split(cm.Name, "-bak-")
			cmName := cmNames[0]
			envResource := &models.EnvResource{
				ProductName: product.ProductName,
				EnvName:     product.EnvName,
				Namespace:   product.Namespace,
				Name:        cmName,
				Type:        string(config.CommonEnvCfgTypeConfigMap),
			}
			cmLables := cm.GetLabels()
			if _, ok := cmLables[setting.UpdateTime]; ok {
				tm, err := time.Parse("20060102150405", cmLables[setting.UpdateTime])
				if err != nil {
					return nil, e.ErrListResources.AddErr(err)
				}
				envResource.CreateTime = tm.Unix()
			}

			delete(cmLables, setting.UpdateTime)
			delete(cmLables, setting.ConfigBackupLabel)
			cm.SetLabels(cmLables)
			cm.SetManagedFields(nil)
			cm.SetResourceVersion("")
			cm.SetAnnotations(make(map[string]string))
			cm.SetName(cmName)

			yamlData, err := ensureLabelAndNs(cm, product.Namespace, productName)
			if err != nil {
				return nil, e.ErrListResources.AddDesc(err.Error())
			}

			envResource.YamlData = yamlData
			err = commonrepo.NewEnvResourceColl().Create(envResource)
			if err != nil {
				return nil, e.ErrListResources.AddErr(err)
			}
			res = append(res, envResource)
		}
	}
	return res, nil
}
