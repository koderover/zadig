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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

const chartTemplateCounterName = "template&chart:%s"

type ChartColl struct {
	*mongo.Collection

	coll string
}

func NewChartColl() *ChartColl {
	name := models.Chart{}.TableName()
	return &ChartColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ChartColl) GetCollectionName() string {
	return c.coll
}

func (c *ChartColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *ChartColl) Get(name string) (*models.Chart, error) {
	res := &models.Chart{}

	query := bson.M{"name": name}
	err := c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}

func (c *ChartColl) Exist(name string) bool {
	res := struct {
		ID primitive.ObjectID `bson:"_id"`
	}{}

	query := bson.M{"name": name}
	opts := options.FindOne().SetProjection(bson.D{{"_id", 1}})
	err := c.FindOne(context.TODO(), query, opts).Decode(&res)

	return err != mongo.ErrNoDocuments
}

func (c *ChartColl) List() ([]*models.Chart, error) {
	var res []*models.Chart

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

func (c *ChartColl) Create(obj *models.Chart) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	rev, err := getNextRevision(obj.Name)
	if err != nil {
		return err
	}
	obj.Revision = rev

	_, err = c.InsertOne(context.TODO(), obj)

	return err
}

func (c *ChartColl) Update(obj *models.Chart) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	rev, err := getNextRevision(obj.Name)
	if err != nil {
		return err
	}
	obj.Revision = rev

	query := bson.M{"name": obj.Name}
	change := bson.M{"$set": obj}
	_, err = c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ChartColl) Delete(name string) error {
	query := bson.M{"name": name}
	_, err := c.DeleteOne(context.TODO(), query)

	return err
}

func getNextRevision(name string) (int64, error) {
	template := fmt.Sprintf(chartTemplateCounterName, name)
	return NewCounterColl().GetNextSeq(template)
}
