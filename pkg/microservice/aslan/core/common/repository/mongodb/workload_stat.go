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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type WorkLoadsStatColl struct {
	*mongo.Collection
	mongo.Session
	coll string
}

func NewWorkLoadsStatColl() *WorkLoadsStatColl {
	name := models.WorkloadStat{}.TableName()
	return &WorkLoadsStatColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func NewWorkLoadsStatCollWithSession(session mongo.Session) *WorkLoadsStatColl {
	name := models.WorkloadStat{}.TableName()
	return &WorkLoadsStatColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *WorkLoadsStatColl) Create(args *models.WorkloadStat) error {
	if args == nil {
		return errors.New("nil WorkLoadsCounter args")
	}
	_, err := c.InsertOne(mongotool.SessionContext(context.TODO(), c.Session), args)
	return err
}

func (c *WorkLoadsStatColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "namespace", Value: 1},
			bson.E{Key: "cluster_id", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *WorkLoadsStatColl) GetCollectionName() string {
	return c.coll
}

func (c *WorkLoadsStatColl) Find(clusterID string, namespace string) (*models.WorkloadStat, error) {
	query := bson.M{}
	query["namespace"] = namespace
	query["cluster_id"] = clusterID

	resp := new(models.WorkloadStat)

	err := c.FindOne(mongotool.SessionContext(context.TODO(), c.Session), query).Decode(resp)
	return resp, err
}

func (c *WorkLoadsStatColl) FindByProductName(productName string) ([]*models.WorkloadStat, error) {
	workloads := make([]*models.WorkloadStat, 0)
	query := bson.M{
		"workloads.product_name": productName,
	}
	ctx := context.Background()
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &workloads)
	if err != nil {
		return nil, err
	}
	return workloads, nil
}

func (c *WorkLoadsStatColl) UpdateWorkloads(args *models.WorkloadStat) error {
	query := bson.M{"namespace": args.Namespace, "cluster_id": args.ClusterID}
	change := bson.M{"$set": bson.M{
		"workloads": args.Workloads,
	}}
	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change, options.Update().SetUpsert(true))
	if err != nil {
		log.Errorf("UpdateOne err:%s - workloads:%+v", err, args.Workloads)
	}

	return err
}
