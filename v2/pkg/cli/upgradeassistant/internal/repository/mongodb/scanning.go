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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ScanningListOption struct {
	Type string
}

type ScanningColl struct {
	*mongo.Collection

	coll string
}

func NewScanningColl() *ScanningColl {
	name := models.Scanning{}.TableName()
	coll := &ScanningColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *ScanningColl) List(opt *ScanningListOption) ([]*models.Scanning, error) {
	if opt == nil {
		return nil, errors.New("nil ListOption")
	}

	query := bson.M{}

	if len(opt.Type) != 0 {
		query["scanner_type"] = opt.Type
	}

	var resp []*models.Scanning
	ctx := context.Background()
	opts := options.Find()
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *ScanningColl) Update(id primitive.ObjectID, update *models.Scanning) error {
	query := bson.M{"_id": id}
	change := bson.M{"$set": update}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
