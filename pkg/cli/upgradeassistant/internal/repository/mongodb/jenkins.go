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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type JenkinsIntegrationColl struct {
	*mongo.Collection

	coll string
}

func NewJenkinsIntegrationColl() *JenkinsIntegrationColl {
	name := models.JenkinsIntegration{}.TableName()
	coll := &JenkinsIntegrationColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *JenkinsIntegrationColl) List() ([]*models.JenkinsIntegration, error) {
	resp := make([]*models.JenkinsIntegration, 0)
	query := bson.M{}

	ctx := context.Background()
	opts := options.Find().SetSort(bson.D{{"updated_at", -1}})

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
