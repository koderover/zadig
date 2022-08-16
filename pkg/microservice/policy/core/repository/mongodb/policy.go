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
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type PolicyColl struct {
	*mongo.Collection

	coll string
}

func NewPolicyColl() *PolicyColl {
	name := models.Policy{}.TableName()
	return &PolicyColl{
		Collection: mongotool.Database(config.PolicyDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PolicyColl) GetCollectionName() string {
	return c.coll
}

func (c *PolicyColl) EnsureIndex(ctx context.Context) error {
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

func (c *PolicyColl) GetByNames(names []string) ([]*models.Policy, error) {
	query := bson.M{}
	query["name"] = bson.M{"$in": names}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	var res []*models.Policy
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *PolicyColl) Get(ns, name string) (*models.Policy, bool, error) {
	res := &models.Policy{}

	query := bson.M{"namespace": ns, "name": name}
	err := c.FindOne(context.TODO(), query).Decode(res)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false, nil
		}

		return nil, false, err
	}

	return res, true, nil
}

func (c *PolicyColl) List() ([]*models.Policy, error) {
	var res []*models.Policy

	ctx := context.Background()

	cursor, err := c.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *PolicyColl) ListBy(projectName string) ([]*models.Policy, error) {
	var res []*models.Policy

	ctx := context.Background()
	query := bson.M{"namespace": projectName}

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

func (c *PolicyColl) ListBySpaceAndName(projectName string, name string) ([]*models.Policy, error) {
	var res []*models.Policy

	ctx := context.Background()
	query := bson.M{"namespace": projectName, "name": name}

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

func (c *PolicyColl) Create(obj *models.Policy) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), obj)

	return err
}

func (c *PolicyColl) BulkCreate(args []*models.Policy) error {
	if len(args) == 0 {
		return nil
	}
	var ois []interface{}
	for _, obj := range args {
		ois = append(ois, obj)
	}
	_, err := c.InsertMany(context.TODO(), ois)
	return err
}

func (c *PolicyColl) Delete(name string, projectName string) error {
	query := bson.M{"name": name, "namespace": projectName}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *PolicyColl) DeleteMany(names []string, projectName string) error {
	query := bson.M{}
	if len(projectName) > 0 {
		query = bson.M{"namespace": projectName}
	}
	if len(names) > 0 {
		query["name"] = bson.M{"$in": names}
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)
	return err
}

func (c *PolicyColl) UpdatePolicy(obj *models.Policy) error {
	// avoid panic issue
	if obj == nil {
		return errors.New("nil Policy")
	}

	query := bson.M{"name": obj.Name, "namespace": obj.Namespace}
	change := bson.M{"$set": bson.M{
		"rules":       obj.Rules,
		"description": obj.Description,
		"update_time": obj.UpdateTime,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *PolicyColl) UpdateOrCreate(obj *models.Policy) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	query := bson.M{"name": obj.Name, "namespace": obj.Namespace}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, obj, opts)

	return err
}
