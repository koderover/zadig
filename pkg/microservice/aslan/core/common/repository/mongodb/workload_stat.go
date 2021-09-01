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

	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type WorkLoadsStatColl struct {
	*mongo.Collection

	coll string
}

func NewWorkLoadsStatColl() *WorkLoadsStatColl {
	name := models.WorkLoadStat{}.TableName()
	return &WorkLoadsStatColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WorkLoadsStatColl) Create(args *models.WorkLoadStat) error {
	if args == nil {
		return errors.New("nil WorkLoadsCounter args")
	}
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *WorkLoadsStatColl) Find(cluster string, namespace string) (*models.WorkLoadStat, error) {
	query := bson.M{}

	query["namespace"] = namespace

	if cluster != "" {
		query["cluster_id"] = cluster
	}

	resp := new(models.WorkLoadStat)

	err := c.FindOne(context.TODO(), query).Decode(resp)
	return resp, err
}

func (c *WorkLoadsStatColl) UpdateWorkloads(args *models.WorkLoadStat) error {
	query := bson.M{"namespace": args.Namespace, "cluster_id": args.ClusterID}
	change := bson.M{"$set": bson.M{
		"workloads": args.Workloads,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}
