/*
Copyright 2023 The KodeRover Authors.

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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ScanningTemplateQueryOption struct {
	ID   string
	Name string
}

type ScanningTemplateColl struct {
	*mongo.Collection
	coll string
}

func NewScanningTemplateColl() *ScanningTemplateColl {
	name := models.ScanningTemplate{}.TableName()
	return &ScanningTemplateColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ScanningTemplateColl) GetCollectionName() string {
	return c.coll
}

func (c *ScanningTemplateColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *ScanningTemplateColl) Create(obj *models.ScanningTemplate) error {
	if obj == nil {
		return fmt.Errorf("nil object")
	}

	obj.UpdatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), obj)
	return err
}

func (c *ScanningTemplateColl) Find(opt *ScanningTemplateQueryOption) (*models.ScanningTemplate, error) {
	if opt == nil {
		return nil, errors.New("nil FindOption")
	}
	query := bson.M{}
	if len(opt.ID) > 0 {
		id, err := primitive.ObjectIDFromHex(opt.ID)
		if err != nil {
			return nil, err
		}
		query["_id"] = id
	}
	if len(opt.Name) > 0 {
		query["name"] = opt.Name
	}
	resp := new(models.ScanningTemplate)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ScanningTemplateColl) List(pageNum, pageSize int) ([]*models.ScanningTemplate, int, error) {
	resp := make([]*models.ScanningTemplate, 0)
	query := bson.M{}
	count, err := c.EstimatedDocumentCount(context.TODO())
	if err != nil {
		return nil, 0, err
	}

	opt := options.Find()
	if pageNum != 0 && pageSize != 0 {
		opt.SetSkip(int64((pageNum - 1) * pageSize)).SetLimit(int64(pageSize))
	}

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, 0, err
	}
	return resp, int(count), nil
}

func (c *ScanningTemplateColl) Update(idStr string, obj *models.ScanningTemplate) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	change := bson.M{"$set": obj}
	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ScanningTemplateColl) DeleteByID(idStr string) error {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *ScanningTemplateColl) ListByCursor() (*mongo.Cursor, error) {
	query := bson.M{}

	return c.Collection.Find(context.TODO(), query)
}
