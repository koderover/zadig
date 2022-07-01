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

package migrate

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.12.0", "1.13.0", V1120ToV1130)
	upgradepath.RegisterHandler("1.13.0", "1.12.0", V1130ToV1120)
}

func V1120ToV1130() error {
	return CombineEnvResources()
}

func V1130ToV1120() error {
	return RemoveEnvResource()
}

func CombineEnvResources() error {
	log.Info("start CombineEnvResources")
	resourceList := make([]*models.EnvResource, 0)

	cms, err := internalmongodb.NewConfigMapColl().List()
	if err != nil {
		return err
	}
	resourceList = append(resourceList, ConvertCmToResource(cms)...)

	ings, err := internalmongodb.NewIngressColl().List()
	if err != nil {
		return err
	}
	resourceList = append(resourceList, ConvertIngToResource(ings)...)

	secrets, err := internalmongodb.NewSecretColl().List()
	if err != nil {
		return err
	}
	resourceList = append(resourceList, ConvertSecretToResource(secrets)...)

	pvcs, err := internalmongodb.NewPvcColl().List()
	if err != nil {
		return err
	}
	resourceList = append(resourceList, ConvertPVCToResource(pvcs)...)

	return internalmongodb.NewEnvResourceColl().BatchInsert(resourceList)
}

func ConvertCmToResource(cms []*models.EnvConfigMap) []*models.EnvResource {
	resourceList := make([]*models.EnvResource, 0, len(cms))
	for _, cm := range cms {
		resourceList = append(resourceList, &models.EnvResource{
			ID:             primitive.ObjectID{},
			Type:           "ConfigMap",
			ProductName:    cm.ProductName,
			CreateTime:     cm.CreateTime,
			UpdateUserName: cm.UpdateUserName,
			Namespace:      cm.Namespace,
			EnvName:        cm.EnvName,
			Name:           cm.Name,
			YamlData:       cm.YamlData,
			DeletedAt:      0,
		})
	}
	return resourceList
}

func ConvertIngToResource(ings []*models.EnvIngress) []*models.EnvResource {
	resourceList := make([]*models.EnvResource, 0, len(ings))
	for _, ingress := range ings {
		resourceList = append(resourceList, &models.EnvResource{
			ID:             primitive.ObjectID{},
			Type:           "Ingress",
			ProductName:    ingress.ProductName,
			CreateTime:     ingress.CreateTime,
			UpdateUserName: ingress.UpdateUserName,
			Namespace:      ingress.Namespace,
			EnvName:        ingress.EnvName,
			Status:         ingress.Status,
			Name:           ingress.Name,
			YamlData:       ingress.YamlData,
			DeletedAt:      0,
		})
	}
	return resourceList
}

func ConvertSecretToResource(secrets []*models.EnvSecret) []*models.EnvResource {
	resourceList := make([]*models.EnvResource, 0, len(secrets))
	for _, secret := range secrets {
		resourceList = append(resourceList, &models.EnvResource{
			ID:             primitive.ObjectID{},
			Type:           "Secret",
			ProductName:    secret.ProductName,
			CreateTime:     secret.CreateTime,
			UpdateUserName: secret.UpdateUserName,
			Namespace:      secret.Namespace,
			EnvName:        secret.EnvName,
			Status:         secret.Status,
			Name:           secret.Name,
			YamlData:       secret.YamlData,
			DeletedAt:      0,
		})
	}
	return resourceList
}

func ConvertPVCToResource(pvcs []*models.EnvPvc) []*models.EnvResource {
	resourceList := make([]*models.EnvResource, 0, len(pvcs))
	for _, pvc := range pvcs {
		resourceList = append(resourceList, &models.EnvResource{
			ID:             primitive.ObjectID{},
			Type:           "PVC",
			ProductName:    pvc.ProductName,
			CreateTime:     pvc.CreateTime,
			UpdateUserName: pvc.UpdateUserName,
			Namespace:      pvc.Namespace,
			EnvName:        pvc.EnvName,
			Status:         pvc.Status,
			Name:           pvc.Name,
			YamlData:       pvc.YamlData,
			DeletedAt:      0,
		})
	}
	return resourceList
}

func RemoveEnvResource() error {
	return internalmongodb.NewEnvResourceColl().Drop(context.TODO())
}
