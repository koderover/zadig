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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ItReportColl struct {
	*mongo.Collection

	coll string
}

func NewItReportColl() *ItReportColl {
	name := models.ItReport{}.TableName()
	coll := &ItReportColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *ItReportColl) GetCollectionName() string {
	return c.coll
}

func (c *ItReportColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "pipeline_name", Value: 1},
			bson.E{Key: "pipeline_task_id", Value: 1},
			bson.E{Key: "test_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *ItReportColl) Upsert(args *models.ItReport) error {
	query := bson.M{"pipeline_name": args.PipelineName, "pipeline_task_id": args.PipelineTaskID, "test_name": args.TestName}
	args.Created = time.Now().Unix()

	_, err := c.UpdateOne(context.TODO(), query, args, options.Update().SetUpsert(true))

	return err
}
