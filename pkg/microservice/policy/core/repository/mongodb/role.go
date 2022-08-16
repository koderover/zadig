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

type RoleColl struct {
	*mongo.Collection

	coll string
}

func NewRoleColl() *RoleColl {
	name := models.Role{}.TableName()
	return &RoleColl{
		Collection: mongotool.Database(config.PolicyDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *RoleColl) GetCollectionName() string {
	return c.coll
}

func (c *RoleColl) EnsureIndex(ctx context.Context) error {
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

func (c *RoleColl) Get(ns, name string) (*models.Role, bool, error) {
	res := &models.Role{}

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

func (c *RoleColl) List() ([]*models.Role, error) {
	var res []*models.Role

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

func (c *RoleColl) ListBy(projectName string) ([]*models.Role, error) {
	var res []*models.Role

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

func (c *RoleColl) ListBySpaceAndName(projectName string, name string) ([]*models.Role, error) {
	var res []*models.Role

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

func (c *RoleColl) Create(obj *models.Role) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), obj)

	return err
}

func (c *RoleColl) Delete(name string, projectName string) error {
	query := bson.M{"name": name, "namespace": projectName}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *RoleColl) DeleteMany(names []string, projectName string) error {
	query := bson.M{"namespace": projectName}
	if len(names) > 0 {
		query["name"] = bson.M{"$in": names}
	}
	_, err := c.Collection.DeleteMany(context.TODO(), query)
	return err
}

func (c *RoleColl) UpdateRole(obj *models.Role) error {
	// avoid panic issue
	if obj == nil {
		return errors.New("nil Role")
	}

	query := bson.M{"name": obj.Name, "namespace": obj.Namespace}
	change := bson.M{"$set": bson.M{
		"rules": obj.Rules,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *RoleColl) UpdateOrCreate(obj *models.Role) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	query := bson.M{"name": obj.Name, "namespace": obj.Namespace}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, obj, opts)

	return err
}
