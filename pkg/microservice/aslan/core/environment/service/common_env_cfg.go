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
	"encoding/json"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
)

func DeleteCommonEnvCfg(envName, productName, objectName string, commonEnvCfgType config.CommonEnvCfgType, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return e.ErrDeleteResource.AddErr(err)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return e.ErrDeleteResource.AddErr(err)
	}
	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
		return e.ErrDeleteResource.AddErr(err)
	}

	switch commonEnvCfgType {
	case config.CommonEnvCfgTypeConfigMap:
		err = updater.DeleteConfigMap(product.Namespace, objectName, kubeClient)
	case config.CommonEnvCfgTypeSecret:
		err = updater.DeleteSecretWithName(product.Namespace, objectName, kubeClient)
	case config.CommonEnvCfgTypeIngress:
		err = updater.DeleteIngresseWithName(product.Namespace, objectName, clientset)
	case config.CommonEnvCfgTypePvc:
		err = updater.DeletePvcWithName(product.Namespace, objectName, clientset)
	default:
		return e.ErrDeleteResource.AddDesc(fmt.Sprintf("%s is not support delete", commonEnvCfgType))
	}
	if err != nil {
		log.Error(err)
		return e.ErrDeleteResource.AddDesc(err.Error())
	}
	return nil
}

type CreateCommonEnvCfgArgs struct {
	EnvName          string                  `json:"env_name"`
	ProductName      string                  `json:"product_name"`
	YamlData         string                  `json:"yaml_data"`
	CommonEnvCfgType config.CommonEnvCfgType `json:"common_env_cfg_type"`
}

// ensureLabel ensure label 's-product' exists in particular resource(secret/configmap) deployed by zadig
func ensureLabel(res *v1.ObjectMeta, projectName string) (string, error) {
	resLabels := res.GetLabels()
	if resLabels == nil {
		resLabels = make(map[string]string)
	}
	resLabels[setting.ProductLabel] = projectName
	res.SetLabels(resLabels)
	yamlData, err := yaml.Marshal(res)
	return string(yamlData), err
}

func CreateCommonEnvCfg(args *CreateCommonEnvCfgArgs, userName, userID string, log *zap.SugaredLogger) error {
	js, err := yaml.YAMLToJSON([]byte(args.YamlData))
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}
	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", product.ClusterID, err)
		return e.ErrUpdateResource.AddErr(err)
	}

	u, err := serializer.NewDecoder().YamlToUnstructured(js)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}
	if args.CommonEnvCfgType == config.CommonEnvCfgTypePvc {
		if u.GetKind() != setting.PersistentVolumeClaim {
			return e.ErrUpdateResource.AddDesc(fmt.Sprintf("param commonEnvCfgType:%s not match yaml kind %s ", args.CommonEnvCfgType, u.GetKind()))
		}
	} else if u.GetKind() != string(args.CommonEnvCfgType) {
		return e.ErrUpdateResource.AddDesc(fmt.Sprintf("param commonEnvCfgType:%s not match yaml kind %s ", args.CommonEnvCfgType, u.GetKind()))
	}

	switch args.CommonEnvCfgType {
	case config.CommonEnvCfgTypeConfigMap:
		cm := &corev1.ConfigMap{}
		err = json.Unmarshal(js, cm)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}
		cm.Namespace = product.Namespace

		yamlData, err := ensureLabel(&cm.ObjectMeta, args.ProductName)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}

		if err := updater.CreateConfigMap(cm, kubeClient); err != nil {
			log.Error(err)
			return e.ErrUpdateResource.AddErr(err)
		}

		envCM := &models.EnvConfigMap{
			ProductName:    args.ProductName,
			UpdateUserName: userName,
			EnvName:        args.EnvName,
			Name:           cm.Name,
			YamlData:       yamlData,
		}
		if err = commonrepo.NewConfigMapColl().Create(envCM, true); err != nil {
			return e.ErrUpdateResource.AddDesc(err.Error())
		}
	case config.CommonEnvCfgTypeSecret:
		secret := &corev1.Secret{}
		err = json.Unmarshal(js, secret)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}
		secret.Namespace = product.Namespace

		yamlData, err := ensureLabel(&secret.ObjectMeta, args.ProductName)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}

		if err := updater.UpdateOrCreateSecret(secret, kubeClient); err != nil {
			log.Error(err)
			return e.ErrUpdateResource.AddDesc(err.Error())
		}

		envSecret := &models.EnvSecret{
			ProductName:    args.ProductName,
			UpdateUserName: userName,
			EnvName:        args.EnvName,
			Name:           secret.Name,
			YamlData:       yamlData,
		}
		if err = commonrepo.NewSecretColl().Create(envSecret, true); err != nil {
			return e.ErrUpdateResource.AddDesc(err.Error())
		}
	case config.CommonEnvCfgTypeIngress:
		uia := &UpdateCommonEnvCfgArgs{
			EnvName:     args.EnvName,
			ProductName: args.ProductName,
			YamlData:    args.YamlData,
		}
		if err = UpdateOrCreateIngress(uia, userName, userID, true, log); err != nil {
			log.Error(err)
			return e.ErrUpdateResource.AddErr(err)
		}
	case config.CommonEnvCfgTypePvc:
		pvc := &corev1.PersistentVolumeClaim{}
		err = json.Unmarshal(js, pvc)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}
		pvc.Namespace = product.Namespace
		if err := updater.CreatePvc(product.Namespace, pvc, clientset); err != nil {
			log.Error(err)
			return e.ErrUpdateResource.AddErr(err)
		}
		envPvc := &models.EnvPvc{
			ProductName:    args.ProductName,
			UpdateUserName: userName,
			EnvName:        args.EnvName,
			Name:           pvc.Name,
			YamlData:       args.YamlData,
		}
		if err = commonrepo.NewPvcColl().Create(envPvc, true); err != nil {
			return e.ErrUpdateResource.AddDesc(err.Error())
		}
	default:
		return e.ErrUpdateResource.AddDesc(fmt.Sprintf("%s is not support create", args.CommonEnvCfgType))
	}
	return nil
}

type UpdateCommonEnvCfgArgs struct {
	EnvName              string                  `json:"env_name"`
	ProductName          string                  `json:"product_name"`
	ServiceName          string                  `json:"service_name"`
	Name                 string                  `json:"name"`
	YamlData             string                  `json:"yaml_data"`
	RestartAssociatedSvc bool                    `json:"restart_associated_svc"`
	CommonEnvCfgType     config.CommonEnvCfgType `json:"common_env_cfg_type"`
	Services             []string                `json:"services"`
}

func UpdateCommonEnvCfg(args *UpdateCommonEnvCfgArgs, userName, userID string, log *zap.SugaredLogger) error {
	var err error
	u, err := serializer.NewDecoder().YamlToUnstructured([]byte(args.YamlData))
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}
	if args.CommonEnvCfgType == config.CommonEnvCfgTypePvc {
		if u.GetKind() != setting.PersistentVolumeClaim {
			return e.ErrUpdateResource.AddDesc(fmt.Sprintf("param commonEnvCfgType:%s not match yaml kind %s ", args.CommonEnvCfgType, u.GetKind()))
		}
	} else if u.GetKind() != string(args.CommonEnvCfgType) {
		return e.ErrUpdateResource.AddDesc(fmt.Sprintf("param commonEnvCfgType:%s not match yaml kind %s ", args.CommonEnvCfgType, u.GetKind()))
	}

	switch args.CommonEnvCfgType {
	case config.CommonEnvCfgTypeConfigMap:
		err = UpdateConfigMap(args, userName, userID, log)
	case config.CommonEnvCfgTypeSecret:
		err = UpdateSecret(args, userName, userID, log)
	case config.CommonEnvCfgTypeIngress:
		err = UpdateOrCreateIngress(args, userName, userID, false, log)
	case config.CommonEnvCfgTypePvc:
		err = UpdatePvc(args, userName, userID, log)
	default:
		return e.ErrUpdateResource.AddDesc(fmt.Sprintf("%s is not support update", args.CommonEnvCfgType))
	}
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

type ListCommonEnvCfgHistoryArgs struct {
	EnvName          string `json:"envName"`
	ProductName      string `json:"productName"`
	Name             string
	CommonEnvCfgType config.CommonEnvCfgType `json:"commonEnvCfgType"`
}
type ListCommonEnvCfgHistoryRes struct {
	ID             primitive.ObjectID `json:"id,omitempty"`
	ProductName    string             `json:"product_name"`
	CreateTime     int64              `json:"create_time"`
	UpdateUserName string             `json:"update_user_name"`
	Namespace      string             `json:"namespace,omitempty"`
	EnvName        string             `json:"env_name"`
	Name           string             `json:"name"`
	YamlData       string             `json:"yaml_data"`
}

func ListCommonEnvCfgHistory(args *ListCommonEnvCfgHistoryArgs, log *zap.SugaredLogger) ([]*ListCommonEnvCfgHistoryRes, error) {
	opts := &commonrepo.ListEnvCfgOption{
		IsSort:      true,
		ProductName: args.ProductName,
		EnvName:     args.EnvName,
		Name:        args.Name,
	}
	switch args.CommonEnvCfgType {
	case config.CommonEnvCfgTypeConfigMap:
		res, err := commonrepo.NewConfigMapColl().List(opts)
		if err != nil {
			return nil, e.ErrListResources.AddErr(err)
		}
		var listRes []*ListCommonEnvCfgHistoryRes
		for _, resElem := range res {
			listElem := &ListCommonEnvCfgHistoryRes{
				ID:             resElem.ID,
				ProductName:    resElem.ProductName,
				CreateTime:     resElem.CreateTime,
				UpdateUserName: resElem.UpdateUserName,
				Namespace:      resElem.Namespace,
				EnvName:        resElem.EnvName,
				Name:           resElem.Name,
				YamlData:       resElem.YamlData,
			}
			listRes = append(listRes, listElem)
		}
		return listRes, nil
	case config.CommonEnvCfgTypeSecret:
		res, err := commonrepo.NewSecretColl().List(opts)
		if err != nil {
			return nil, e.ErrListResources.AddErr(err)
		}
		var listRes []*ListCommonEnvCfgHistoryRes
		for _, resElem := range res {
			listElem := &ListCommonEnvCfgHistoryRes{
				ID:             resElem.ID,
				ProductName:    resElem.ProductName,
				CreateTime:     resElem.CreateTime,
				UpdateUserName: resElem.UpdateUserName,
				Namespace:      resElem.Namespace,
				EnvName:        resElem.EnvName,
				Name:           resElem.Name,
				YamlData:       resElem.YamlData,
			}
			listRes = append(listRes, listElem)
		}
		return listRes, nil
	case config.CommonEnvCfgTypeIngress:
		res, err := commonrepo.NewIngressColl().List(opts)
		if err != nil {
			return nil, e.ErrListResources.AddErr(err)
		}
		var listRes []*ListCommonEnvCfgHistoryRes
		for _, resElem := range res {
			listElem := &ListCommonEnvCfgHistoryRes{
				ID:             resElem.ID,
				ProductName:    resElem.ProductName,
				CreateTime:     resElem.CreateTime,
				UpdateUserName: resElem.UpdateUserName,
				Namespace:      resElem.Namespace,
				EnvName:        resElem.EnvName,
				Name:           resElem.Name,
				YamlData:       resElem.YamlData,
			}
			listRes = append(listRes, listElem)
		}
		return listRes, nil
	case config.CommonEnvCfgTypePvc:
		res, err := commonrepo.NewPvcColl().List(opts)
		if err != nil {
			return nil, e.ErrListResources.AddErr(err)
		}
		var listRes []*ListCommonEnvCfgHistoryRes
		for _, resElem := range res {
			listElem := &ListCommonEnvCfgHistoryRes{
				ID:             resElem.ID,
				ProductName:    resElem.ProductName,
				CreateTime:     resElem.CreateTime,
				UpdateUserName: resElem.UpdateUserName,
				Namespace:      resElem.Namespace,
				EnvName:        resElem.EnvName,
				Name:           resElem.Name,
				YamlData:       resElem.YamlData,
			}
			listRes = append(listRes, listElem)
		}
		return listRes, nil
	default:
		return nil, e.ErrUpdateResource.AddDesc(fmt.Sprintf("%s is not support update", args.CommonEnvCfgType))
	}
}

func restartPod(name, productName, envName, namespace string, commonEnvCfgType config.CommonEnvCfgType, clientset *kubernetes.Clientset, kubcli client.Client) error {
	opts := &commonrepo.ListEnvSvcDependOption{
		ProductName: productName,
		EnvName:     envName,
	}
	envSvcProducts, err := commonrepo.NewEnvSvcDependColl().List(opts)
	if err != nil {
		return err
	}
	tplProduct, err := template.NewProductColl().Find(productName)
	if err != nil {
		return err
	}

	for _, esp := range envSvcProducts {
		switch commonEnvCfgType {
		case config.CommonEnvCfgTypeConfigMap:
			if !checkExistInList(name, esp.ConfigMaps) {
				continue
			}
		case config.CommonEnvCfgTypeSecret:
			if !checkExistInList(name, esp.Secrets) {
				continue
			}
		case config.CommonEnvCfgTypePvc:
			if !checkExistInList(name, esp.Pvcs) {
				continue
			}
		default:
			return fmt.Errorf("unsupport type %s to restart service", commonEnvCfgType)
		}

		restartArgs := &SvcOptArgs{
			EnvName:     envName,
			ProductName: productName,
			ServiceName: esp.ServiceName,
		}
		if tplProduct.ProductFeature.DeployType == "k8s" {
			if err := restartK8sPod(restartArgs, namespace, clientset); err != nil {
				log.Error(err)
				return err
			}
			continue
		}

		if err := restartPodHelmByWorkload(restartArgs, namespace, clientset, kubcli); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

func restartK8sPod(args *SvcOptArgs, ns string, clientset *kubernetes.Clientset) error {
	selector := labels.Set{setting.ProductLabel: args.ProductName, setting.ServiceLabel: args.ServiceName}.AsSelector()
	log.Infof("deleting pod from %s where %s", ns, selector)
	return updater.DeletePods(ns, selector, clientset)
}

func restartPodHelmByWorkload(args *SvcOptArgs, ns string, clientset *kubernetes.Clientset, kucli client.Client) error {
	deployment, found, err := getter.GetDeployment(ns, args.ServiceName, kucli)
	if err != nil {
		return err
	}
	if found {
		selector := labels.Set(deployment.GetLabels()).AsSelector()
		log.Infof("deleting deploy %s pod from %s where %s", args.ServiceName, ns, selector)
		return updater.DeletePods(ns, selector, clientset)
	}

	sts, found, err := getter.GetStatefulSet(ns, args.ServiceName, kucli)
	if err != nil {
		return err
	}
	if found {
		selector := labels.Set(sts.GetLabels()).AsSelector()
		log.Infof("deleting sts %s pod from %s where %s", args.ServiceName, ns, selector)
		return updater.DeletePods(ns, selector, clientset)
	}

	return nil
}

func checkExistInList(target string, arrList []string) bool {
	for _, arr := range arrList {
		if target == arr {
			return true
		}
	}
	return false
}
