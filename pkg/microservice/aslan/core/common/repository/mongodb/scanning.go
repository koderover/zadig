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

type ScanningListOption struct {
	ProjectName   string
	ScanningNames []string
	TemplateID    string
}

type ScanningColl struct {
	*mongo.Collection

	coll string
}

func NewScanningColl() *ScanningColl {
	name := models.Scanning{}.TableName()
	return &ScanningColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ScanningColl) GetCollectionName() string {
	return c.coll
}

func (c *ScanningColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *ScanningColl) Create(scanning *models.Scanning) error {
	if scanning == nil {
		return errors.New("nil scanning args")
	}

	scanning.CreatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), scanning)
	return err
}

func (c *ScanningColl) Update(idString string, scanning *models.Scanning) error {
	if scanning == nil {
		return fmt.Errorf("nil object")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}

	scanning.UpdatedAt = time.Now().Unix()

	filter := bson.M{"_id": id}
	update := bson.M{"$set": scanning}

	_, err = c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *ScanningColl) List(listOption *ScanningListOption, pageNum, pageSize int64) ([]*models.Scanning, int64, error) {
	query := bson.M{}
	resp := make([]*models.Scanning, 0)
	ctx := context.Background()

	opt := options.Find()

	if pageNum != 0 && pageSize != 0 {
		opt.
			SetSkip((pageNum - 1) * pageSize).
			SetLimit(pageSize)
	}

	if listOption != nil {
		if len(listOption.ProjectName) > 0 {
			query["project_name"] = listOption.ProjectName
		}

		if len(listOption.ScanningNames) > 0 {
			query["name"] = bson.M{"$in": listOption.ScanningNames}
		}

		if len(listOption.TemplateID) > 0 {
			query["template_id"] = listOption.TemplateID
		}
	}

	cursor, err := c.Collection.Find(ctx, query, opt)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, 0, err
	}
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	return resp, count, nil
}

func (c *ScanningColl) Find(projectName, name string) (*models.Scanning, error) {
	resp := new(models.Scanning)
	query := bson.M{"name": name, "project_name": projectName}
	err := c.FindOne(context.TODO(), query).Decode(resp)
	return resp, err
}

// (CAUTION) This function is used to get the scanning by id,
// but it will not fill the scanning template info.
// If you need to get the scanning template info at once, please use the service layer to get it.
func (c *ScanningColl) GetByID(idstring string) (*models.Scanning, error) {
	resp := new(models.Scanning)
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ScanningColl) DeleteByID(idstring string) error {
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *ScanningColl) GetScanningTemplateReference(templateID string) ([]*models.Scanning, error) {
	query := bson.M{
		"template_id": templateID,
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	ret := make([]*models.Scanning, 0)
	err = cursor.All(context.TODO(), &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *ScanningColl) ListByCursor() (*mongo.Cursor, error) {
	query := bson.M{}

	return c.Collection.Find(context.TODO(), query)
}
