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

type PolicyColl struct {
	*mongo.Collection

	coll string
}

func NewPolicyColl() *PolicyColl {
	name := models.Policy{}.TableName()
	return &PolicyColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PolicyColl) GetCollectionName() string {
	return c.coll
}

func (c *PolicyColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"resource": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
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

func (c *PolicyColl) UpdateOrCreate(obj *models.Policy) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	query := bson.M{"resource": obj.Resource}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, obj, opts)

	return err
}
