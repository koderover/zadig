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

type ListPolicyOptions struct {
	PolicyName, PolicyNamespace string
}

type PolicyBindingColl struct {
	*mongo.Collection

	coll string
}

func NewPolicyBindingColl() *PolicyBindingColl {
	name := models.PolicyBinding{}.TableName()
	return &PolicyBindingColl{
		Collection: mongotool.Database(config.PolicyDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PolicyBindingColl) GetCollectionName() string {
	return c.coll
}

func (c *PolicyBindingColl) EnsureIndex(ctx context.Context) error {
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

func (c *PolicyBindingColl) List(opts ...*ListPolicyOptions) ([]*models.PolicyBinding, error) {
	var res []*models.PolicyBinding

	ctx := context.Background()
	query := bson.M{}
	if len(opts) > 0 {
		opt := opts[0]
		if opt.PolicyName != "" {
			query["policy_ref.name"] = opt.PolicyName
			query["policy_ref.namespace"] = opt.PolicyNamespace
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

func (c *PolicyBindingColl) ListBy(projectName, uid string) ([]*models.PolicyBinding, error) {
	var res []*models.PolicyBinding

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

func (c *PolicyBindingColl) ListByUser(uid string) ([]*models.PolicyBinding, error) {
	var res []*models.PolicyBinding

	ctx := context.Background()
	query := bson.M{}
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

func (c *PolicyBindingColl) Delete(name string, projectName string) error {
	query := bson.M{"name": name, "namespace": projectName}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *PolicyBindingColl) DeleteMany(names []string, projectName string, userID string) error {
	query := bson.M{}
	if projectName != "" {
		query["namespace"] = projectName
	}
	if len(names) > 0 {
		query["name"] = bson.M{"$in": names}
	}

	if userID != "" {
		query = bson.M{"subjects.uid": userID}
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)

	return err
}

func (c *PolicyBindingColl) DeleteByPolicy(policyName string, projectName string) error {
	query := bson.M{"policy_ref.name": policyName, "policy_ref.namespace": projectName}
	// if projectName == "", delete all policybindings in all namespaces
	if projectName != "" {
		query["namespace"] = projectName
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)

	return err
}

func (c *PolicyBindingColl) DeleteByPolicies(policyNames []string, projectName string) error {
	query := bson.M{}
	if len(projectName) != 0 {
		query = bson.M{"policy_ref.namespace": projectName, "namespace": projectName}
	}

	if len(policyNames) != 0 {
		query["policy_ref.name"] = bson.M{"$in": policyNames}
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)

	return err
}

func (c *PolicyBindingColl) Create(obj *models.PolicyBinding) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), obj)

	return err
}

func (c *PolicyBindingColl) BulkCreate(objs []*models.PolicyBinding) error {
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

func (c *PolicyBindingColl) UpdateOrCreate(obj *models.PolicyBinding) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	query := bson.M{"name": obj.Name, "namespace": obj.Namespace}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, obj, opts)

	return err
}
