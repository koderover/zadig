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
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"strings"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/registry"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/util"
)

type RepoImgResp struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
	Tag   string `json:"tag"`
}

type RepoInfo struct {
	RegType      string `json:"regType"`
	RegProvider  string `json:"regProvider"`
	RegNamespace string `json:"regNamespace"`
	RegAddr      string `json:"regAddr"`
	ID           string `json:"id"`
}

const (
	PERPAGE = 100
	PAGE    = 1
)

// ListRegistries 为了抹掉ak和sk的数据
func ListRegistries(log *zap.SugaredLogger) ([]*commonmodels.RegistryNamespace, error) {
	registryNamespaces, err := commonservice.ListRegistryNamespaces(false, log)
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %v", err)
		return registryNamespaces, fmt.Errorf("RegistryNamespace.List error: %v", err)
	}
	for _, registryNamespace := range registryNamespaces {
		registryNamespace.AccessKey = ""
		registryNamespace.SecretKey = ""
	}
	return registryNamespaces, nil
}

func CreateRegistryNamespace(username string, args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	regOps := new(commonrepo.FindRegOps)
	regOps.IsDefault = true
	defaultReg, isSystemDefault, err := commonservice.FindDefaultRegistry(false, log)
	if err != nil {
		log.Warnf("failed to find the default registry, the error is: %s", err)
	}
	if args.IsDefault {
		if defaultReg != nil && !isSystemDefault {
			defaultReg.IsDefault = false
			err := UpdateRegistryNamespaceDefault(defaultReg, log)
			if err != nil {
				log.Errorf("updateRegistry error: %v", err)
				return fmt.Errorf("RegistryNamespace.Create error: %v", err)
			}
		}
	} else {
		if isSystemDefault {
			log.Errorf("create registry error: There must be at least 1 default registry")
			return fmt.Errorf("RegistryNamespace.Create error: %s", "There must be at least 1 default registry")
		}
	}

	args.UpdateBy = username
	args.Namespace = strings.TrimSpace(args.Namespace)

	if err := commonrepo.NewRegistryNamespaceColl().Create(args); err != nil {
		log.Errorf("RegistryNamespace.Create error: %v", err)
		return fmt.Errorf("RegistryNamespace.Create error: %v", err)
	}

	return SyncDinDForRegistries(log)
}

func UpdateRegistryNamespace(username, id string, args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	regOps := new(commonrepo.FindRegOps)
	regOps.IsDefault = true
	defaultReg, isSystemDefault, err := commonservice.FindDefaultRegistry(false, log)
	if err != nil {
		log.Warnf("failed to find the default registry, the error is: %s", err)
	}
	if args.IsDefault {
		if defaultReg != nil && !isSystemDefault {
			defaultReg.IsDefault = false
			err := UpdateRegistryNamespaceDefault(defaultReg, log)
			if err != nil {
				log.Errorf("updateRegistry error: %v", err)
				return fmt.Errorf("RegistryNamespace.Update error: %v", err)
			}
		}
	} else {
		if isSystemDefault || id == defaultReg.ID.Hex() {
			log.Errorf("create registry error: There must be at least 1 default registry")
			return fmt.Errorf("RegistryNamespace.Create error: %s", "There must be at least 1 default registry")
		}
	}
	args.UpdateBy = username
	args.Namespace = strings.TrimSpace(args.Namespace)

	if err := commonrepo.NewRegistryNamespaceColl().Update(id, args); err != nil {
		log.Errorf("RegistryNamespace.Update error: %v", err)
		return fmt.Errorf("RegistryNamespace.Update error: %v", err)
	}
	return SyncDinDForRegistries(log)
}

func DeleteRegistryNamespace(id string, log *zap.SugaredLogger) error {
	registries, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err != nil {
		log.Errorf("RegistryNamespace.FindAll error: %s", err)
		return err
	}
	var (
		isDefault          = false
		registryNamespaces []*commonmodels.RegistryNamespace
	)
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ExcludeStatus: []string{setting.ProductStatusDeleting},
	})

	for _, env := range envs {
		if env.RegistryID == id {
			return errors.New("The registry cannot be deleted, it's being used by environment")
		}
	}

	// whether it is the default registry
	for _, registry := range registries {
		if registry.ID.Hex() == id && registry.IsDefault {
			isDefault = true
			continue
		}
		registryNamespaces = append(registryNamespaces, registry)
	}

	if err := commonrepo.NewRegistryNamespaceColl().Delete(id); err != nil {
		log.Errorf("RegistryNamespace.Delete error: %s", err)
		return err
	}

	if isDefault && len(registryNamespaces) > 0 {
		registryNamespaces[0].IsDefault = true
		if err := commonrepo.NewRegistryNamespaceColl().Update(registryNamespaces[0].ID.Hex(), registryNamespaces[0]); err != nil {
			log.Errorf("RegistryNamespace.Update error: %s", err)
			return err
		}
	}
	return SyncDinDForRegistries(log)
}

func ListAllRepos(log *zap.SugaredLogger) ([]*RepoInfo, error) {
	repoInfos := make([]*RepoInfo, 0)
	resp, err := commonservice.ListRegistryNamespaces(false, log)
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %v", err)
		return nil, fmt.Errorf("RegistryNamespace.List error: %v", err)
	}
	for _, rn := range resp {
		repoInfo := new(RepoInfo)
		repoInfo.RegProvider = rn.RegProvider
		repoInfo.RegType = rn.RegType
		repoInfo.RegNamespace = rn.Namespace
		repoInfo.RegAddr = rn.RegAddr
		repoInfo.ID = rn.ID.Hex()

		repoInfos = append(repoInfos, repoInfo)
	}
	return repoInfos, nil
}

func ListReposTags(registryInfo *commonmodels.RegistryNamespace, names []string, logger *zap.SugaredLogger) ([]*RepoImgResp, error) {
	repos, err := registry.NewV2Service(registryInfo.RegProvider).ListRepoImages(registry.ListRepoImagesOption{
		Endpoint: registry.Endpoint{
			Addr:      registryInfo.RegAddr,
			Ak:        registryInfo.AccessKey,
			Sk:        registryInfo.SecretKey,
			Namespace: registryInfo.Namespace,
			Region:    registryInfo.Region,
		},
		Repos: names,
	}, logger)

	images := make([]*RepoImgResp, 0)
	if err == nil {
		for _, repo := range repos.Repos {
			for _, tag := range repo.Tags {
				img := &RepoImgResp{
					Host:  util.TrimURLScheme(registryInfo.RegAddr),
					Owner: repo.Namespace,
					Name:  repo.Name,
					Tag:   tag,
				}
				images = append(images, img)
			}
		}
	} else {
		err = e.ErrListImages.AddErr(err)
	}

	return images, err
}

func GetRepoTags(registryInfo *commonmodels.RegistryNamespace, name string, log *zap.SugaredLogger) (*registry.ImagesResp, error) {
	var resp *registry.ImagesResp
	repos, err := registry.NewV2Service(registryInfo.RegProvider).ListRepoImages(registry.ListRepoImagesOption{
		Endpoint: registry.Endpoint{
			Addr:      registryInfo.RegAddr,
			Ak:        registryInfo.AccessKey,
			Sk:        registryInfo.SecretKey,
			Namespace: registryInfo.Namespace,
			Region:    registryInfo.Region,
		},
		Repos: []string{name},
	}, log)

	if err != nil {
		err = e.ErrListImages.AddErr(err)
	} else {
		if len(repos.Repos) == 0 {
			err = e.ErrListImages.AddDesc(fmt.Sprintf("找不到Repo %s", name))
		} else {
			repo := repos.Repos[0]
			var images []registry.Image
			for _, tag := range repo.Tags {
				images = append(images, registry.Image{Tag: tag})
			}

			resp = &registry.ImagesResp{
				NameSpace: registryInfo.Namespace,
				Name:      name,
				Total:     len(repo.Tags),
				Images:    images,
			}
		}
	}

	return resp, err
}

func UpdateRegistryNamespaceDefault(args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	if err := commonrepo.NewRegistryNamespaceColl().Update(args.ID.Hex(), args); err != nil {
		log.Errorf("UpdateRegistryNamespaceDefault.Update error: %v", err)
		return fmt.Errorf("UpdateRegistryNamespaceDefault.Update error: %v", err)
	}
	return nil
}

func SyncDinDForRegistries(log *zap.SugaredLogger) error {
	regList, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err != nil {
		log.Errorf("failed to list registry to update dind, the error is: %s", err)
		return err
	}

	stsResource := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulset"}

	dynamicClient, err := kubeclient.GetDynamicKubeClient(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		log.Errorf("failed to create dynamic kubernetes clientset for clusterID: %s, the error is: %s", setting.LocalClusterID, err)
		return err
	}

	volumeMountList := make([]map[string]string, 0)
	volumeList := make([]map[string]interface{}, 0)

	insecureRegistryList := make([]string, 0)
	for _, reg := range regList {
		// compatibility changes before 1.11
		if reg.AdvancedSetting != nil {
			// if a registry is marked as insecure, we add a record to insecure-registries
			if !reg.AdvancedSetting.TLSEnabled {
				insecureRegistryList = append(insecureRegistryList, reg.RegAddr)
			}
			// if a registry is marked as secure and a TLS cert is given, we mount this certificate to dind daemon
			if reg.AdvancedSetting.TLSEnabled && reg.AdvancedSetting.TLSCert != "" {
				mountName := fmt.Sprintf("%s-cert", reg.ID.Hex())
				err := ensureCertificateSecret(mountName, config.Namespace(), reg.AdvancedSetting.TLSCert, log)
				if err != nil {
					log.Errorf("failed to ensure secret: %s, the error is: %s", mountName, err)
					return err
				}

				// create volumeMount info
				volumeMountMap := map[string]string{
					"mountPath": "/etc/docker/certs.d",
					"name":      mountName,
				}
				volumeMountList = append(volumeMountList, volumeMountMap)
				// create volume info
				secretItemList := make([]map[string]string, 0)
				secretItemList = append(secretItemList, map[string]string{
					"key":  "cert.crt",
					"path": fmt.Sprintf("%s/%s", reg.RegAddr, "cert.crt"),
				})
				secretInfo := map[string]interface{}{
					"items":      secretItemList,
					"secretName": mountName,
				}
				volumeMap := map[string]interface{}{
					"name":   mountName,
					"secret": secretInfo,
				}
				volumeList = append(volumeList, volumeMap)
			}
		}
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of Deployment before attempting update
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		result, getErr := dynamicClient.Resource(stsResource).Namespace(config.Namespace()).Get(context.TODO(), "dind", metav1.GetOptions{})
		if getErr != nil {
			return err
		}

		// extract spec containers
		containers, found, err := unstructured.NestedSlice(result.Object, "spec", "template", "spec", "containers")
		if err != nil || !found || containers == nil {
			return err
		}

		if err := unstructured.SetNestedField(containers[0].(map[string]interface{}), volumeMountList, "volumeMounts"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(result.Object, containers, "spec", "template", "spec", "containers"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(result.Object, volumeList, "spec", "template", "spec", "volumes"); err != nil {
			return err
		}
		_, updateErr := dynamicClient.Resource(stsResource).Namespace(config.Namespace()).Update(context.TODO(), result, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		log.Errorf("failed to update dind, the error is: %s", retryErr)
	}

	return retryErr
}

func ensureCertificateSecret(secretName, namespace, cert string, log *zap.SugaredLogger) error {
	certificateString := base64.StdEncoding.EncodeToString([]byte(cert))
	datamap := map[string]string{
		"cert.crt": certificateString,
	}

	secretType := &corev1.Secret{}

	gvr, _ := meta.UnsafeGuessKindToResource(secretType.GroupVersionKind())

	dynamicClient, err := kubeclient.GetDynamicKubeClient(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		log.Errorf("failed to create dynamic kubernetes clientset for clusterID: %s, the error is: %s", setting.LocalClusterID, err)
		return err
	}

	secret, err := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}).Namespace(namespace).Get(context.TODO(), "aslan", metav1.GetOptions{})
	// if there is an error, either because of not found or anything else, we try to create a secret with the given information
	if err != nil {
		secret := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name": secretName,
				},
				"type": "Opaque",
				"data": datamap,
			},
		}

		_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			log.Errorf("failed to create secret: %s, the error is: %s", secretName, err)
		}
		return err
	} else {
		//if err := unstructured.SetNestedField(secret.Object, datamap, "data"); err != nil {
		//	log.Errorf("failed to set data in secret object, the error is: %s", err)
		//	return err
		//}
		//_, err := dynamicClient.Resource(gvr).Namespace(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		//if err != nil {
		//	log.Errorf("failed to update secret: %s, the error is: %s", secretName, err)
		//}
		//return err
		log.Infof("got deployment: %+v", secret)
		return nil
	}
}
