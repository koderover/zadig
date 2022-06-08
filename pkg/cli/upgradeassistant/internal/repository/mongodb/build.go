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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type BuildListOption struct {
}

type BuildColl struct {
	*mongo.Collection

	coll string
}

func NewBuildColl() *BuildColl {
	name := models.Build{}.TableName()
	return &BuildColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *BuildColl) List(opt *BuildListOption) ([]*models.Build, error) {
	if opt == nil {
		return nil, errors.New("nil ListOption")
	}

	query := bson.M{}

	var resp []*models.Build
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

func (c *BuildColl) Update(build *models.Build) error {
	if build == nil {
		return errors.New("nil Module args")
	}

	query := bson.M{"name": build.Name}
	if build.ProductName != "" {
		query["product_name"] = build.ProductName
	}

	updateBuild := bson.M{"$set": build}

	_, err := c.Collection.UpdateOne(context.TODO(), query, updateBuild)
	return err
}
