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

package migrate

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"

	internalmongodb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func init() {
	upgradepath.RegisterHandler("1.7.1", "1.8.0", V171ToV180)
	upgradepath.RegisterHandler("1.8.0", "1.7.1", V180ToV171)
}

// V171ToV180 update all the roleBinding names in this format "{uid}-{roleName}-{roleNamespace}"
// Caution: this migration contains unrecoverable changes, please back up the database in advance
func V171ToV180() error {
	log.Info("Migrating data from 1.7.1 to 1.8.0")
	if err := updateAllRoleBindingNames(); err != nil {
		log.Errorf("Failed to update roleBinding names, err: %s", err)
		return err
	}

	log.Info("Start to patchProductRegistryID")
	err := patchProductRegistryID()
	if err != nil {
		log.Errorf("Failed to patchProductRegistryID, err: %s", err)
		return err
	}

	return nil
}

func V180ToV171() error {
	log.Info("Rollback data from 1.8.0 to 1.7.1")
	return nil
}

func patchProductRegistryID() error {
	// get all products
	products, err := internalmongodb.NewProductColl().List(&internalmongodb.ProductListOptions{})
	if err != nil {
		log.Errorf("Fail to list products, err: %s", err)
		return err
	}
	registry, err := internalmongodb.NewRegistryNamespaceColl().Find(&internalmongodb.FindRegOps{IsDefault: true})
	if err != nil {
		log.Errorf("Fail to find default registry, err: %s", err)
		return err
	}
	// change type to readable string
	for _, v := range products {
		if len(v.RegistryID) == 0 {
			v.RegistryID = registry.ID.Hex()
		}
	}
	err = internalmongodb.NewProductColl().UpdateAllRegistry(products)
	if err != nil {
		log.Errorf("Fail to update products, err: %s", err)
		return err
	}
	return nil
}

func updateAllRoleBindingNames() error {
	coll := newRoleBindingColl()
	rbs, err := coll.List()
	if err != nil {
		log.Errorf("Failed to list roleBindings, err: %s", err)
		return err
	}

	var lastErr error
	for _, rb := range rbs {
		newName := roleBindingName(rb)
		if newName == "" || rb.Name == newName {
			continue
		}

		log.Infof("Update roleBinding name in namespace %s from %s to %s", rb.Namespace, rb.Name, newName)
		if err = updateRoleBindingName(coll, rb, newName); err != nil {
			log.Warnf("Failed to update roleBinding, err: %s", err)
			lastErr = err
			continue
		}
	}

	return lastErr
}

func roleBindingName(rb *models.RoleBinding) string {
	if len(rb.Subjects) != 1 || rb.Subjects[0].Kind != models.UserKind {
		return ""
	}

	return config.RoleBindingNameFromUIDAndRole(rb.Subjects[0].UID, setting.RoleType(rb.RoleRef.Name), rb.RoleRef.Namespace)
}

func updateRoleBindingName(c *mongodb.RoleBindingColl, rb *models.RoleBinding, newName string) error {
	query := bson.M{"name": rb.Name, "namespace": rb.Namespace}

	change := bson.M{"$set": bson.M{
		"name": newName,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func newRoleBindingColl() *mongodb.RoleBindingColl {
	name := models.RoleBinding{}.TableName()
	return &mongodb.RoleBindingColl{
		Collection: mongotool.Database(fmt.Sprintf("%s_policy", config.MongoDatabase())).Collection(name),
	}
}
