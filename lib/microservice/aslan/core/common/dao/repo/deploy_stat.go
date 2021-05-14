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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type DeployStatOption struct {
	StartDate    int64
	EndDate      int64
	Limit        int
	Skip         int
	IsAsc        bool
	IsMaxDeploy  bool
	ProductNames []string
}

// DeployTotalPipeResp ...
type DeployTotalPipeResp struct {
	ID           DeployTotalItem `bson:"_id"                      json:"_id"`
	TotalSuccess int             `bson:"total_deploy_success"     json:"total_deploy_success"`
	TotalFailure int             `bson:"total_deploy_failure"     json:"total_deploy_failure"`
}

// DeployTotalItem ...
type DeployTotalItem struct {
	ProductName  string `bson:"product_name"              json:"product_name"`
	TotalSuccess int    `bson:"total_deploy_success"      json:"total_deploy_success"`
	TotalFailure int    `bson:"total_deploy_failure"      json:"total_deploy_failure"`
}

// DeployDailyPipeResp ...
type DeployDailyPipeResp struct {
	ID           DeployDailyItem `bson:"_id"                             json:"_id"`
	TotalSuccess int             `bson:"total_deploy_success"            json:"total_deploy_success"`
	TotalFailure int             `bson:"total_deploy_failure"            json:"total_deploy_failure"`
}

// DeployDailyItem ...
type DeployDailyItem struct {
	Date         string `bson:"date"                      json:"date"`
	TotalSuccess int    `bson:"total_deploy_success"      json:"total_deploy_success"`
	TotalFailure int    `bson:"total_deploy_failure"      json:"total_deploy_failure"`
}

type DeployStatColl struct {
	*mongo.Collection

	coll string
}

func NewDeployStatColl() *DeployStatColl {
	name := models.DeployStat{}.TableName()
	return &DeployStatColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *DeployStatColl) GetCollectionName() string {
	return c.coll
}

func (c *DeployStatColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "product_name", Value: 1},
			bson.E{Key: "date", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *DeployStatColl) GetDeployTotalAndSuccess() ([]*DeployTotalItem, error) {
	var result []*DeployTotalPipeResp
	var resp []*DeployTotalItem

	pipeline := []bson.M{{
		"$group": bson.M{
			"_id": bson.M{
				"product_name": "$product_name",
			},
			"total_deploy_success": bson.M{
				"$sum": "$total_deploy_success",
			},
			"total_deploy_failure": bson.M{
				"$sum": "$total_deploy_failure",
			},
		},
	}}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}
	if err := cursor.All(context.TODO(), &result); err != nil {
		return nil, err
	}
	for _, res := range result {
		deployItem := &DeployTotalItem{
			ProductName:  res.ID.ProductName,
			TotalSuccess: res.TotalSuccess,
			TotalFailure: res.TotalFailure,
		}
		resp = append(resp, deployItem)
	}

	return resp, nil
}

func (c *DeployStatColl) GetDeployDailyTotal(args *DeployStatOption) ([]*DeployDailyItem, error) {
	var result []*DeployDailyPipeResp
	var resp []*DeployDailyItem

	pipeline := []bson.M{{
		"$sort": bson.M{
			"create_time": 1,
		},
	}}

	if args.StartDate > 0 || args.EndDate > 0 {
		timeRange := bson.M{}
		if args.StartDate > 0 {
			timeRange["$gte"] = args.StartDate
		}
		if args.EndDate > 0 {
			timeRange["$lte"] = args.EndDate
		}
		pipeline = append(pipeline, bson.M{"$match": bson.M{"create_time": timeRange}})
	}

	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"date": "$date",
			},
			"total_deploy_success": bson.M{
				"$sum": "$total_deploy_success",
			},
			"total_deploy_failure": bson.M{
				"$sum": "$total_deploy_failure",
			},
		},
	})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}
	if err := cursor.All(context.TODO(), &result); err != nil {
		return nil, err
	}
	for _, res := range result {
		deployDailyItem := &DeployDailyItem{
			Date:         res.ID.Date,
			TotalSuccess: res.TotalSuccess,
			TotalFailure: res.TotalFailure,
		}
		resp = append(resp, deployDailyItem)
	}

	return resp, nil
}
