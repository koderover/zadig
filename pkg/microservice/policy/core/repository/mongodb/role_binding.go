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
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type RoleBindingColl struct {
	*mongo.Collection

	coll string
}

func NewRoleBindingColl() *RoleBindingColl {
	name := models.RoleBinding{}.TableName()
	return &RoleBindingColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
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

func (c *RoleBindingColl) List() ([]*models.RoleBinding, error) {
	var res []*models.RoleBinding

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

func (c *RoleBindingColl) Create(obj *models.RoleBinding) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	_, err := c.InsertOne(context.TODO(), obj)

	return err
}
