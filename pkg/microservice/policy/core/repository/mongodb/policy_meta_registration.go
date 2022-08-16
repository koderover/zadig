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

type PolicyMetaColl struct {
	*mongo.Collection

	coll string
}

func NewPolicyMetaColl() *PolicyMetaColl {
	name := models.PolicyMeta{}.TableName()
	return &PolicyMetaColl{
		Collection: mongotool.Database(config.PolicyDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *PolicyMetaColl) GetCollectionName() string {
	return c.coll
}

func (c *PolicyMetaColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"resource": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *PolicyMetaColl) List() ([]*models.PolicyMeta, error) {
	var res []*models.PolicyMeta

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

func (c *PolicyMetaColl) UpdateOrCreate(obj *models.PolicyMeta) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	query := bson.M{"resource": obj.Resource}
	opts := options.Replace().SetUpsert(true)
	_, err := c.ReplaceOne(context.TODO(), query, obj, opts)

	return err
}
