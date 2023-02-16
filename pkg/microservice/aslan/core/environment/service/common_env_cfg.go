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
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type ResourceWithLabel interface {
	GetLabels() map[string]string
	SetLabels(labels map[string]string)
	SetNamespace(ns string)
}

type ResourceResponseBase struct {
	Name           string                  `json:"-"`
	Type           config.CommonEnvCfgType `json:"-"`
	EnvName        string                  `json:"-"`
	ProjectName    string                  `json:"-"`
	YamlData       string                  `json:"yaml_data"`
	UpdateUserName string                  `json:"update_username"`
	CreateTime     time.Time               `json:"create_time"`
	Services       []string                `json:"services"`
	SourceDetail   *models.CreateFromRepo  `json:"source_detail"`
	AutoSync       bool                    `json:"auto_sync"`
}

func (resp *ResourceResponseBase) setSourceDetailData(resource ResourceWithLabel) {
	envResource, err := commonrepo.NewEnvResourceColl().Find(&commonrepo.QueryEnvResourceOption{
		Name:           resp.Name,
		Type:           string(resp.Type),
		EnvName:        resp.EnvName,
		ProductName:    resp.ProjectName,
		IgnoreNotFound: true,
		Active:         true,
	})
	if err != nil {
		log.Warnf("failed to f get %v with name : %s, err: %s", string(resp.Type), resp.Name, err)
		return
	}
	if envResource != nil {
		resp.SourceDetail = envResource.SourceDetail
		resp.AutoSync = envResource.AutoSync
		resp.UpdateUserName = envResource.UpdateUserName
	}
}

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

	resourceData, err := commonrepo.NewEnvResourceColl().Find(&commonrepo.QueryEnvResourceOption{
		ProductName:    productName,
		EnvName:        product.EnvName,
		Name:           objectName,
		Type:           string(commonEnvCfgType),
		IgnoreNotFound: true,
	})
	if err != nil {
		return err
	}

	if resourceData != nil {
		return commonrepo.NewEnvResourceColl().Delete(resourceData.ID)
	}
	return nil
}

// ensureLabelAndNs ensure label 's-product' exists in particular resource(secret/configmap) deployed by zadig
func ensureLabelAndNs(res ResourceWithLabel, namespace, projectName string) (string, error) {
	resLabels := res.GetLabels()
	if resLabels == nil {
		resLabels = make(map[string]string)
	}
	resLabels[setting.ProductLabel] = projectName
	res.SetLabels(resLabels)
	res.SetNamespace(namespace)

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	return string(yamlBytes), err
}

func geneSourceDetail(gitRepoConfig *templatemodels.GitRepoConfig) *models.CreateFromRepo {
	if gitRepoConfig == nil {
		return nil
	}
	ret := &models.CreateFromRepo{
		GitRepoConfig: &templatemodels.GitRepoConfig{
			CodehostID: gitRepoConfig.CodehostID,
			Owner:      gitRepoConfig.Owner,
			Repo:       gitRepoConfig.Repo,
			Branch:     gitRepoConfig.Branch,
			Namespace:  gitRepoConfig.GetNamespace(),
		},
	}
	if len(gitRepoConfig.ValuesPaths) > 0 {
		ret.LoadPath = gitRepoConfig.ValuesPaths[0]
	}
	return ret
}

func getResourceYamlAndName(sourceYaml []byte, namespace, productName string, resType config.CommonEnvCfgType) (name, yamlData string, err error) {
	switch resType {
	case config.CommonEnvCfgTypeConfigMap:
		cm := &corev1.ConfigMap{}
		err = yaml.Unmarshal(sourceYaml, cm)
		if err != nil {
			return
		}
		yamlData, err = ensureLabelAndNs(cm, namespace, productName)
		if err != nil {
			return
		}
		name = cm.Name
	case config.CommonEnvCfgTypeSecret:
		secret := &corev1.Secret{}
		err = json.Unmarshal(sourceYaml, secret)
		if err != nil {
			return
		}
		yamlData, err = ensureLabelAndNs(secret, namespace, productName)
		if err != nil {
			return
		}
		name = secret.Name
	case config.CommonEnvCfgTypeIngress:
		u, errDecode := serializer.NewDecoder().YamlToUnstructured(sourceYaml)
		if errDecode != nil {
			err = fmt.Errorf("Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", string(sourceYaml), errDecode)
			return
		}
		yamlData, err = ensureLabelAndNs(u, namespace, productName)
		if err != nil {
			return
		}
		name = u.GetName()
	case config.CommonEnvCfgTypePvc:
		pvc := &corev1.PersistentVolumeClaim{}
		err = json.Unmarshal(sourceYaml, pvc)
		if err != nil {
			return
		}
		yamlData, err = ensureLabelAndNs(pvc, namespace, productName)
		if err != nil {
			return
		}
		name = pvc.Name
	default:
		err = fmt.Errorf("%s is not support create", resType)
		return
	}
	return
}

func CreateCommonEnvCfg(args *models.CreateUpdateCommonEnvCfgArgs, userName string, log *zap.SugaredLogger) error {
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

	envResource := &models.EnvResource{
		ProductName:    args.ProductName,
		UpdateUserName: userName,
		EnvName:        args.EnvName,
		Namespace:      product.Namespace,
		Type:           string(args.CommonEnvCfgType),
		SourceDetail:   geneSourceDetail(args.GitRepoConfig),
		AutoSync:       args.AutoSync,
	}

	switch args.CommonEnvCfgType {
	case config.CommonEnvCfgTypeConfigMap:
		cm := &corev1.ConfigMap{}
		err = yaml.Unmarshal([]byte(args.YamlData), cm)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}
		yamlData, err := ensureLabelAndNs(cm, product.Namespace, args.ProductName)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}

		if err := updater.CreateConfigMap(cm, kubeClient); err != nil {
			log.Error(err)
			return e.ErrUpdateResource.AddErr(err)
		}
		envResource.Name = cm.Name
		envResource.YamlData = yamlData

	case config.CommonEnvCfgTypeSecret:
		secret := &corev1.Secret{}
		err = json.Unmarshal(js, secret)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}
		yamlData, err := ensureLabelAndNs(secret, product.Namespace, args.ProductName)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}

		if err := updater.UpdateOrCreateSecret(secret, kubeClient); err != nil {
			log.Error(err)
			return e.ErrUpdateResource.AddDesc(err.Error())
		}
		envResource.Name = secret.Name
		envResource.YamlData = yamlData

	case config.CommonEnvCfgTypeIngress:
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(args.YamlData))
		if err != nil {
			log.Errorf("Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", args.YamlData, err)
			return e.ErrUpdateResource.AddDesc("ingress Yaml Name is incorrect")
		}

		yamlData, err := ensureLabelAndNs(u, product.Namespace, args.ProductName)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}

		err = updater.UpdateOrCreateUnstructured(u, kubeClient)
		if err != nil {
			log.Errorf("Failed to UpdateOrCreateIngress %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
			return e.ErrUpdateResource.AddErr(fmt.Errorf("Failed to UpdateOrCreateIngress %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err))
		}
		envResource.Name = u.GetName()
		envResource.YamlData = yamlData

	case config.CommonEnvCfgTypePvc:
		pvc := &corev1.PersistentVolumeClaim{}
		err = json.Unmarshal(js, pvc)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}

		yamlData, err := ensureLabelAndNs(pvc, product.Namespace, args.ProductName)
		if err != nil {
			return e.ErrUpdateResource.AddErr(err)
		}

		if err := updater.CreatePvc(product.Namespace, pvc, clientset); err != nil {
			log.Error(err)
			return e.ErrUpdateResource.AddErr(err)
		}
		envResource.Name = pvc.Name
		envResource.YamlData = yamlData

	default:
		return e.ErrUpdateResource.AddDesc(fmt.Sprintf("%s is not support create", args.CommonEnvCfgType))
	}

	if err = commonrepo.NewEnvResourceColl().Create(envResource); err != nil {
		return e.ErrUpdateResource.AddDesc(err.Error())
	}

	return nil
}

func getLatestEnvResource(name, resType, envName, productName string) (*models.EnvResource, error) {
	return commonrepo.NewEnvResourceColl().Find(
		&commonrepo.QueryEnvResourceOption{
			Name:        name,
			ProductName: productName,
			Type:        resType,
			EnvName:     envName,
			IsSort:      true,
		})
}

func UpdateCommonEnvCfg(args *models.CreateUpdateCommonEnvCfgArgs, userName string, updateContentOnly bool, log *zap.SugaredLogger) error {
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

	// when rollback/autoSync env configs, git config should not change
	if updateContentOnly {
		if args.LatestEnvResource == nil {
			args.LatestEnvResource, err = getLatestEnvResource(args.Name, string(args.CommonEnvCfgType), args.EnvName, args.ProductName)
		}
		if err != nil {
			return err
		}
		if args.LatestEnvResource == nil {
			return fmt.Errorf("failed to find env resource %s:%s", args.Name, string(args.CommonEnvCfgType))
		}
		args.AutoSync = args.LatestEnvResource.AutoSync
		args.SourceDetail = args.LatestEnvResource.SourceDetail
	} else {
		args.SourceDetail = geneSourceDetail(args.GitRepoConfig)
	}

	switch args.CommonEnvCfgType {
	case config.CommonEnvCfgTypeConfigMap:
		err = UpdateConfigMap(args, userName, log)
	case config.CommonEnvCfgTypeSecret:
		err = UpdateSecret(args, userName, log)
	case config.CommonEnvCfgTypeIngress:
		err = UpdateOrCreateIngress(args, userName, false, log)
	case config.CommonEnvCfgTypePvc:
		err = UpdatePvc(args, userName, log)
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
	EnvName          string                  `json:"env_name"            form:"envName"`
	ProjectName      string                  `json:"project_name"        form:"projectName"`
	Name             string                  `json:"name"                form:"name"`
	CommonEnvCfgType config.CommonEnvCfgType `json:"common_env_cfg_type" form:"commonEnvCfgType"`
	AutoSync         bool                    `json:"auto_sync"           form:"autoSync"`
}

type SyncEnvResourceArg struct {
	EnvName     string `json:"env_name"`
	ProductName string `json:"product_name"`
	Name        string `json:"name"`
	Type        string `json:"type"`
}

type ListCommonEnvCfgHistoryRes struct {
	ID             primitive.ObjectID     `json:"id,omitempty"`
	ProductName    string                 `json:"product_name"`
	CreateTime     int64                  `json:"create_time"`
	UpdateUserName string                 `json:"update_user_name"`
	Namespace      string                 `json:"namespace,omitempty"`
	EnvName        string                 `json:"env_name"`
	Name           string                 `json:"name"`
	Type           string                 `json:"type"`
	YamlData       string                 `json:"yaml_data"`
	SourceDetail   *models.CreateFromRepo `json:"source_detail"`
}

func GetResourceByCfgType(namespace, name string, cfgType config.CommonEnvCfgType, client client.Client, clientset *kubernetes.Clientset) (ResourceWithLabel, error) {
	switch cfgType {
	case config.CommonEnvCfgTypeConfigMap:
		cm, _, err := getter.GetConfigMap(namespace, name, client)
		return cm, err
	case config.CommonEnvCfgTypeSecret:
		secret, _, err := getter.GetSecret(namespace, name, client)
		return secret, err
	case config.CommonEnvCfgTypePvc:
		pvc, _, err := getter.GetPvc(namespace, name, client)
		return pvc, err
	case config.CommonEnvCfgTypeIngress:
		ingress, _, err := getter.GetUnstructuredIngress(namespace, name, client, clientset)
		return ingress, err
	}
	return nil, fmt.Errorf("unrecognized env config type: %s", cfgType)
}

func ListEnvResourceHistory(args *ListCommonEnvCfgHistoryArgs, log *zap.SugaredLogger) ([]*ListCommonEnvCfgHistoryRes, error) {
	if len(args.CommonEnvCfgType) == 0 {
		return nil, e.ErrListResources.AddDesc("env resource type can't be nil")
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProjectName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}

	envResource, err := GetResourceByCfgType(product.Namespace, args.Name, args.CommonEnvCfgType, kubeClient, clientset)
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	if envResource == nil {
		return nil, e.ErrListResources.AddDesc(fmt.Sprintf("failed to get resource %s:%s from namespace: %s", args.Name, args.CommonEnvCfgType, product.Namespace))
	}

	opts := &commonrepo.QueryEnvResourceOption{
		IsSort:      true,
		ProductName: args.ProjectName,
		EnvName:     args.EnvName,
		Name:        args.Name,
		Type:        string(args.CommonEnvCfgType),
	}
	res, err := commonrepo.NewEnvResourceColl().List(opts)
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
			EnvName:        resElem.EnvName,
			Name:           resElem.Name,
			YamlData:       resElem.YamlData,
			SourceDetail:   resElem.SourceDetail,
		}
		listRes = append(listRes, listElem)
	}
	return listRes, nil
}

func ListLatestEnvResources(args *ListCommonEnvCfgHistoryArgs, log *zap.SugaredLogger) ([]*ListCommonEnvCfgHistoryRes, error) {
	opts := &commonrepo.QueryEnvResourceOption{
		ProductName: args.ProjectName,
		EnvName:     args.EnvName,
		Name:        args.Name,
		AutoSync:    args.AutoSync,
	}
	res, err := commonrepo.NewEnvResourceColl().ListLatestResource(opts)
	if err != nil {
		return nil, e.ErrListResources.AddErr(err)
	}
	var listRes []*ListCommonEnvCfgHistoryRes
	for _, resElem := range res {
		listElem := &ListCommonEnvCfgHistoryRes{
			ProductName: resElem.ID.ProductName,
			EnvName:     resElem.ID.EnvName,
			Type:        resElem.ID.Type,
			Name:        resElem.ID.Name,
			CreateTime:  resElem.CreateTime,
		}
		listRes = append(listRes, listElem)
	}
	return listRes, nil
}

func SyncEnvResource(args *SyncEnvResourceArg, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return e.ErrUpdateResource.AddErr(fmt.Errorf("failed to find product: %s:%s, err: %s", args.ProductName, args.EnvName, err))
	}
	envResource, err := commonrepo.NewEnvResourceColl().Find(&commonrepo.QueryEnvResourceOption{
		ProductName: args.ProductName,
		EnvName:     args.EnvName,
		Name:        args.Name,
		Type:        args.Type,
	})
	if err != nil {
		return e.ErrUpdateResource.AddErr(fmt.Errorf("failed to find env resource: %s:%s of env: %s:%s, err: %s", args.Type, args.Name, args.ProductName, args.EnvName, err))
	}

	if !envResource.AutoSync {
		return nil
	}

	if envResource.SourceDetail == nil || envResource.SourceDetail.GitRepoConfig == nil {
		return e.ErrUpdateResource.AddErr(fmt.Errorf("failed to parse gitops config for resource: %s:%s of env: %s:%s", args.Type, args.Name, args.ProductName, args.EnvName))
	}

	repoConfig := envResource.SourceDetail.GitRepoConfig
	sourceYaml, err := fsservice.DownloadFileFromSource(&fsservice.DownloadFromSourceArgs{
		CodehostID: repoConfig.CodehostID,
		Namespace:  repoConfig.GetNamespace(),
		Owner:      repoConfig.Owner,
		Repo:       repoConfig.Repo,
		Path:       envResource.SourceDetail.LoadPath,
		Branch:     repoConfig.Branch,
	})
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	js, err := yaml.YAMLToJSON(sourceYaml)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	resName, yamlData, err := getResourceYamlAndName(js, product.Namespace, args.ProductName, config.CommonEnvCfgType(envResource.Type))
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	// asset resource name be the same
	if resName != envResource.Name {
		return e.ErrUpdateResource.AddDesc(fmt.Sprintf("resource name not match, expect: %s while parsed: %s", envResource.Name, resName))
	}

	equal, err := yamlutil.Equal(yamlData, envResource.YamlData)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}

	if equal {
		return nil
	}

	err = UpdateCommonEnvCfg(&models.CreateUpdateCommonEnvCfgArgs{
		EnvName:              args.EnvName,
		ProductName:          args.ProductName,
		Name:                 args.Name,
		YamlData:             yamlData,
		RestartAssociatedSvc: false,
		CommonEnvCfgType:     config.CommonEnvCfgType(envResource.Type),
		AutoSync:             envResource.AutoSync,
		LatestEnvResource:    envResource,
	}, "cron", true, log)
	if err != nil {
		return e.ErrUpdateResource.AddErr(err)
	}
	return nil
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
