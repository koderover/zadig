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
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type BasicImageOpt struct {
	Value     string `bson:"value"`
	Label     string `bson:"label"`
	ImageFrom string `bson:"image_from"`
	ImageType string `bson:"image_type"`
}

type BasicImageColl struct {
	*mongo.Collection

	coll string
}

func NewBasicImageColl() *BasicImageColl {
	name := models.BasicImage{}.TableName()
	coll := &BasicImageColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *BasicImageColl) GetSonarTypeImage() ([]*models.BasicImage, error) {
	query := bson.M{}
	query["image_type"] = "sonar"

	ctx := context.Background()
	resp := make([]*models.BasicImage, 0)

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *BasicImageColl) CreateZadigSonarImage() error {
	args := &models.BasicImage{
		Value:      "sonarsource/sonar-scanner-cli",
		Label:      "sonar:latest",
		CreateTime: time.Now().Unix(),
		UpdateTime: time.Now().Unix(),
		UpdateBy:   "Zadig",
		ImageType:  "sonar",
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}
