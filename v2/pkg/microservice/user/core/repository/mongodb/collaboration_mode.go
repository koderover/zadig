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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type CollaborationModeColl struct {
	*mongo.Collection

	coll string
}

func NewCollaborationModeColl() *CollaborationModeColl {
	name := models.CollaborationMode{}.TableName()
	return &CollaborationModeColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *CollaborationModeColl) GetCollectionName() string {
	return c.coll
}

func (c *CollaborationModeColl) ListUserCollaborationMode(uid string) ([]*models.CollaborationMode, error) {
	var ret []*models.CollaborationMode
	query := bson.M{}

	query["members"] = uid
	query["is_deleted"] = false

	ctx := context.Background()
	opts := options.Find()
	opts.SetSort(bson.D{{"create_time", -1}})
	cursor, err := c.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
