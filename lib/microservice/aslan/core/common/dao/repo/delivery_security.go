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

package repo

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type DeliverySecurityArgs struct {
	ID        string `json:"id"`
	ImageID   string `json:"imageId"`
	ImageName string `json:"imageName"`
	Severity  string `json:"severity"`
}

type DeliverySecurityColl struct {
	*mongo.Collection

	coll string
}

func NewDeliverySecurityColl() *DeliverySecurityColl {
	name := models.DeliverySecurity{}.TableName()
	return &DeliverySecurityColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DeliverySecurityColl) GetCollectionName() string {
	return c.coll
}

func (c *DeliverySecurityColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "image_name", Value: 1},
				bson.E{Key: "deleted_at", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "image_id", Value: 1},
				bson.E{Key: "deleted_at", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *DeliverySecurityColl) Insert(args *models.DeliverySecurity) error {
	if args == nil {
		return errors.New("nil delivery_security args")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *DeliverySecurityColl) Find(args *DeliverySecurityArgs) ([]*models.DeliverySecurity, error) {
	if args == nil {
		return nil, errors.New("nil delivery_security args")
	}
	resp := make([]*models.DeliverySecurity, 0)
	query := bson.M{"deleted_at": 0}
	if args.ImageName != "" {
		query["image_name"] = args.ImageName
	}
	if args.ImageID != "" {
		imageID, err := primitive.ObjectIDFromHex(args.ImageID)
		if err != nil {
			return nil, err
		}
		query["image_id"] = imageID
	}
	if args.Severity != "" {
		query["severity"] = args.Severity
	}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DeliverySecurityColl) FindStatistics(imageIDHex string) (map[string]int, error) {
	if imageIDHex == "" {
		return nil, errors.New("nil delivery_security args")
	}
	imageID, err := primitive.ObjectIDFromHex(imageIDHex)
	if err != nil {
		return nil, err
	}
	query := bson.M{"image_id": imageID, "deleted_at": 0}
	severityMap := make(map[string]int)
	severityTypes := []string{"Critical", "High", "Medium", "Low", "Negligible", "Unknown"}
	total := 0
	for _, severityType := range severityTypes {
		query["severity"] = severityType
		count, err := c.CountDocuments(context.TODO(), query)
		if err != nil {
			continue
		}
		severityMap[severityType] = int(count)
		total = total + int(count)
	}
	severityMap["Total"] = total
	return severityMap, nil
}
