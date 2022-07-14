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

package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type LabelColl struct {
	*mongo.Collection
	coll string
}

func NewLabelColl() *LabelColl {
	name := models.Label{}.TableName()
	return &LabelColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *LabelColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "key", Value: 1},
			bson.E{Key: "value", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *LabelColl) BulkCreate(labels []*models.Label) error {
	if len(labels) == 0 {
		return nil
	}
	var ois []interface{}
	for _, label := range labels {
		label.CreateTime = time.Now().Unix()
		ois = append(ois, label)
	}

	_, err := c.InsertMany(context.TODO(), ois)
	return err
}

func (c *LabelColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *LabelColl) BulkDelete(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var oids []primitive.ObjectID
	for _, v := range ids {
		oid, err := primitive.ObjectIDFromHex(v)
		if err != nil {
			return err
		}
		oids = append(oids, oid)
	}
	query := bson.M{}
	query["_id"] = bson.M{"$in": oids}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *LabelColl) BulkDeleteByProject(projectName string) error {
	if projectName == "" {
		return nil
	}

	query := bson.M{}
	query["project_name"] = projectName
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *LabelColl) ListByProjectName(projectName string) ([]*models.Label, error) {
	if len(projectName) == 0 {
		return nil, nil
	}
	query := bson.M{}
	var res []*models.Label
	query["project_name"] = projectName
	ctx := context.Background()
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

func (c *LabelColl) Find(id string) (*models.Label, error) {
	res := new(models.Label)
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	opts := options.FindOne()
	query := bson.M{"_id": oid}
	err = c.FindOne(context.TODO(), query, opts).Decode(res)
	return res, err
}

type Label struct {
	Key         string               `json:"key"`
	Value       string               `json:"value"`
	Type        setting.ResourceType `json:"type"`
	ProjectName string               `json:"project_name"`
}

type ListLabelOpt struct {
	Labels []Label
}

func (c *LabelColl) List(opt ListLabelOpt) ([]*models.Label, error) {
	var res []*models.Label
	if len(opt.Labels) == 0 {
		return nil, nil
	}
	condition := bson.A{}
	for _, label := range opt.Labels {
		condition = append(condition, bson.M{
			"key":   label.Key,
			"value": label.Value,
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

func (c *LabelColl) ListByIDs(ids []string) ([]*models.Label, error) {
	var res []*models.Label
	ctx := context.Background()
	query := bson.M{}
	if len(ids) != 0 {
		var oids []primitive.ObjectID
		for _, id := range ids {
			oid, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				return nil, err
			}
			oids = append(oids, oid)
		}
		query["_id"] = bson.M{"$in": oids}
	}
	opts := options.Find()
	opts.SetSort(bson.D{{"key", 1}})
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
