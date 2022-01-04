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
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/kube/util"
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
	EnvName     string            `json:"env_name"`
	ProductName string            `json:"product_name"`
	ServiceName string            `json:"service_name"`
	ConfigName  string            `json:"config_name"`
	Data        map[string]string `json:"data"`
}

type configMap struct {
	Name              string            `json:"name"`
	Data              map[string]string `json:"data"`
	ModifiedBy        string            `json:"modifiedBy"`
	CreationTimestamp string            `json:"creationTimestamp"`
	LastUpdateTime    string            `json:"lastUpdateTime"`
	lastUpdateTime    time.Time
}

type configMapWithHistory struct {
	Current             *configMap   `json:"current"`
	HistoricalRevisions []*configMap `json:"historicalRevisions"`
}

func generateConfigMapResponse(cm *corev1.ConfigMap) *configMap {
	icm := wrapper.ConfigMap(cm)
	u, _ := icm.LastUpdateTime()
	return &configMap{
		Name:              icm.Name,
		Data:              icm.Data,
		ModifiedBy:        icm.ModifiedBy(),
		CreationTimestamp: util.FormatTime(icm.CreationTimestamp.Time),
		LastUpdateTime:    util.FormatTime(u),
		lastUpdateTime:    u,
	}
}

func ListConfigMaps(args *ListConfigMapArgs, log *zap.SugaredLogger) ([]*configMapWithHistory, error) {
	selector := labels.Set{setting.ProductLabel: args.ProductName, setting.ServiceLabel: args.ServiceName}.AsSelector()

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

	// 为了兼容老数据，需要找到当前使用的cm，即labels中没有config-backup=true的cm
	// 当前使用的cm需要放在数组的首位，备份的cm则按照创建时间倒序
	cmWithHistory := make(map[string][]*configMap)
	cmWithName := make(map[string]*configMap)
	for _, cm := range cms {
		icm := wrapper.ConfigMap(cm)
		res := generateConfigMapResponse(cm)
		if icm.Active() {
			cmWithName[res.Name] = res
		} else {
			owner := icm.Owner()
			if owner == "" {
				continue
			}
			cmWithHistory[owner] = append(cmWithHistory[owner], res)
		}
	}

	res := make([]*configMapWithHistory, 0, len(cmWithName))
	for name, cm := range cmWithName {
		history := cmWithHistory[name]
		if len(history) > 1 {
			sort.SliceStable(history, func(i, j int) bool {
				return history[i].lastUpdateTime.After(history[j].lastUpdateTime)
			})
		}

		res = append(res, &configMapWithHistory{
			Current:             cm,
			HistoricalRevisions: history,
		})
	}

	return res, nil
}

func UpdateConfigMap(envName string, args *UpdateConfigMapArgs, userName, userID string, log *zap.SugaredLogger) error {
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

	namespace := product.Namespace
	cfg, found, err := getter.GetConfigMap(namespace, args.ConfigName, kubeClient)
	if err != nil {
		log.Error(err)
		return e.ErrGetConfigMap.AddDesc(err.Error())
	} else if !found {
		return e.ErrGetConfigMap.AddDesc("configMap not found")
	}

	if err := archiveConfigMap(namespace, cfg, kubeClient, log); err != nil {
		return err
	}

	// 将configMap中的变量进行渲染
	renderSet, err := commonservice.GetRenderSet(namespace, 0, log)
	if err != nil {
		log.Errorf("Failed to find render set for product template %s, err: %v", product.ProductName, err)
		return err
	}

	// 渲染变量
	for key, value := range args.Data {
		for _, kv := range renderSet.KVs {
			value = strings.Replace(value, kv.Alias, kv.Value, -1)
		}
		value = kube.ParseSysKeys(product.Namespace, product.EnvName, product.ProductName, args.ServiceName, value)
		args.Data[key] = value
	}

	cfg.Data = args.Data
	// 记录修改configmap的用户
	cfg.Labels[setting.UpdateBy] = kube.MakeSafeLabelValue(userName)
	cfg.Labels[setting.UpdateByID] = userID
	cfg.Labels[setting.UpdateTime] = time.Now().Format("20060102150405")
	cfg.Labels[setting.DirtyLabel] = setting.LabelValueTrue

	as := cfg.GetAnnotations()
	if as == nil {
		as = make(map[string]string)
	}
	as[setting.ModifiedByAnnotation] = userName
	as[setting.EditorIDAnnotation] = userID
	as[setting.LastUpdateTimeAnnotation] = util.FormatTime(time.Now())
	cfg.SetAnnotations(as)

	if err := updater.UpdateConfigMap(cfg, kubeClient); err != nil {
		log.Error(err)
		return e.ErrUpdateConfigMap.AddDesc(err.Error())
	}

	restartArgs := &SvcOptArgs{
		EnvName:     envName,
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
	}

	if err := restartPod(restartArgs, namespace, kubeClient, log); err != nil {
		log.Error(err)
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
	if err := updater.UpdateConfigMap(destinSrc, kubeClient); err != nil {
		log.Error(err)
		return e.ErrUpdateConfigMap.AddDesc(err.Error())
	}

	restartArgs := &SvcOptArgs{
		EnvName:     envName,
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
	}

	if err := restartPod(restartArgs, namespace, kubeClient, log); err != nil {
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

func restartPod(args *SvcOptArgs, ns string, kubeClient client.Client, log *zap.SugaredLogger) error {
	selector := labels.Set{setting.ProductLabel: args.ProductName, setting.ServiceLabel: args.ServiceName}.AsSelector()

	log.Infof("deleting pod from %s where %s", ns, selector)
	return updater.DeletePods(ns, selector, kubeClient)
}
