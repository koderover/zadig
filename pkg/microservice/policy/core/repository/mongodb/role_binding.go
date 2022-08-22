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

package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ListOptions struct {
	RoleName, RoleNamespace string
}

type RoleBindingColl struct {
	*mongo.Collection

	coll string
}

func NewRoleBindingColl() *RoleBindingColl {
	name := models.RoleBinding{}.TableName()
	return &RoleBindingColl{
		Collection: mongotool.Database(config.PolicyDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *RoleBindingColl) GetCollectionName() string {
	return c.coll
}

func (c *RoleBindingColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "namespace", Value: 1},
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *RoleBindingColl) List(opts ...*ListOptions) ([]*models.RoleBinding, error) {
	var res []*models.RoleBinding

	ctx := context.Background()
	query := bson.M{}
	if len(opts) > 0 {
		opt := opts[0]
		if opt.RoleName != "" {
			query["role_ref.name"] = opt.RoleName
			query["role_ref.namespace"] = opt.RoleNamespace
		}
	}

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *RoleBindingColl) ListBy(projectName, uid string) ([]*models.RoleBinding, error) {
	var res []*models.RoleBinding

	ctx := context.Background()
	query := bson.M{"namespace": projectName}
	if uid != "" {
		query["subjects.uid"] = uid
		query["subjects.kind"] = models.UserKind
	}

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *RoleBindingColl) ListRoleBindingsByUIDs(uids []string) ([]*models.RoleBinding, error) {
	var res []*models.RoleBinding

	ctx := context.Background()
	query := bson.M{}
	if len(uids) > 0 {
		query["subjects.uid"] = bson.M{"$in": uids}
	}

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *RoleBindingColl) ListSystemRoleBindingsByUIDs(uids []string) ([]*models.RoleBinding, error) {
	var res []*models.RoleBinding

	ctx := context.Background()
	query := bson.M{"namespace": "*"}
	if len(uids) > 0 {
		query["subjects.uid"] = bson.M{"$in": uids}
	}

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *RoleBindingColl) Delete(name string, projectName string) error {
	query := bson.M{"name": name, "namespace": projectName}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *RoleBindingColl) DeleteMany(names []string, projectName string, userID string) error {
	query := bson.M{}
	if projectName != "" {
		query["namespace"] = projectName
	}
	if len(names) > 0 {
		query["name"] = bson.M{"$in": names}
	}

	if userID != "" {
		query["subjects.uid"] = userID
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)

	return err
}

func (c *RoleBindingColl) DeleteByRole(roleName string, projectName string) error {
	query := bson.M{"role_ref.name": roleName, "role_ref.namespace": projectName}
	// if projectName == "", delete all rolebindings in all namespaces
	if projectName != "" {
		query["namespace"] = projectName
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)

	return err
}

func (c *RoleBindingColl) DeleteByRoles(roleNames []string, projectName string) error {
	if projectName == "" {
		return fmt.Errorf("projectName is empty")
	}
	if len(roleNames) == 0 {
		return nil
	}

	query := bson.M{"role_ref.name": bson.M{"$in": roleNames}, "role_ref.namespace": projectName, "namespace": projectName}
	_, err := c.Collection.DeleteMany(context.TODO(), query)

	return err
}

func (c *RoleBindingColl) Create(obj *models.RoleBinding) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), obj)

	return err
}

func (c *RoleBindingColl) BulkCreate(objs []*models.RoleBinding) error {
	if len(objs) == 0 {
		return nil
	}

	var ois []interface{}
	for _, obj := range objs {
		ois = append(ois, obj)
	}

	res, err := c.InsertMany(context.TODO(), ois)
	if mongo.IsDuplicateKeyError(err) {
		log.Warnf("Duplicate key found, inserted IDs is %v", res.InsertedIDs)
		return nil
	}

	return err
}

func (c *RoleBindingColl) UpdateOrCreate(obj *models.RoleBinding) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	query := bson.M{"name": obj.Name, "namespace": obj.Namespace}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, obj, opts)

	return err
}

type RoleBinding struct {
	Uid       string `json:"uid"`
	Namespace string `json:"namespace"`
}

type ListRoleBindingsOpt struct {
	RoleBindings []RoleBinding
}

func (c *RoleBindingColl) ListByRoleBindingOpt(opt ListRoleBindingsOpt) ([]*models.RoleBinding, error) {
	var res []*models.RoleBinding

	if len(opt.RoleBindings) == 0 {
		return nil, nil
	}
	condition := bson.A{}
	for _, meta := range opt.RoleBindings {
		condition = append(condition, bson.M{
			"namespace":    meta.Namespace,
			"subjects.uid": meta.Uid,
		})
	}
	filter := bson.D{{"$or", condition}}
	cursor, err := c.Collection.Find(context.TODO(), filter)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}
	return res, nil
}
