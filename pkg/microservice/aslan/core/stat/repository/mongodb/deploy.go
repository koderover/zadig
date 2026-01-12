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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type DeployTotalPipeResp struct {
	ID           DeployTotalItem `bson:"_id"                      json:"_id"`
	TotalSuccess int             `bson:"total_deploy_success"     json:"total_deploy_success"`
	TotalFailure int             `bson:"total_deploy_failure"     json:"total_deploy_failure"`
}

type DeployStat struct {
	TotalSuccess int `bson:"total_deploy_success"     json:"total_deploy_success"`
	TotalFailure int `bson:"total_deploy_failure"     json:"total_deploy_failure"`
}

type DeployTotalItem struct {
	ProductName  string `bson:"product_name"              json:"product_name"`
	TotalSuccess int    `bson:"total_deploy_success"      json:"total_deploy_success"`
	TotalFailure int    `bson:"total_deploy_failure"      json:"total_deploy_failure"`
}

type DeployDailyPipeResp struct {
	ID           DeployDailyItem `bson:"_id"                             json:"_id"`
	TotalSuccess int             `bson:"total_deploy_success"            json:"total_deploy_success"`
	TotalFailure int             `bson:"total_deploy_failure"            json:"total_deploy_failure"`
}

type DeployDailyItem struct {
	Date         string `bson:"date"                      json:"date"`
	TotalSuccess int    `bson:"total_deploy_success"      json:"total_deploy_success"`
	TotalFailure int    `bson:"total_deploy_failure"      json:"total_deploy_failure"`
}

type DeployPipeResp struct {
	ID                  DeployPipeInfo `bson:"_id"                       json:"_id"`
	MaxDeployServiceNum int            `bson:"max_deploy_service_num"            json:"maxDeployServiceNum"`
}

type DeployPipeInfo struct {
	MaxDeployServiceName string `bson:"max_deploy_service_name"           json:"maxDeployServiceName"`
}

type DeployFailurePipeResp struct {
	ID                         DeployFailurePipeInfo `bson:"_id"                       json:"_id"`
	MaxDeployFailureServiceNum int                   `bson:"max_deploy_failure_service_num"    json:"maxDeployFailureServiceNum"`
}

type DeployFailurePipeInfo struct {
	ProductName                 string `bson:"product_name"                      json:"productName"`
	MaxDeployFailureServiceName string `bson:"max_deploy_failure_service_name"   json:"maxDeployFailureServiceName"`
}

type DeployStatColl struct {
	*mongo.Collection

	coll string
}

// Deprecated: this table is removed in 3.1.0, replaced by deploy_stat_weekly. deploy_stat_weekly removed unnecessary field
// and change the time segment to weekly.
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

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *DeployStatColl) Create(args *models.DeployStat) error {
	if args == nil {
		return errors.New("nil deployStat args")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

type DeployStatGetOption struct {
	MaxDeployServiceNum int
	ServiceName         string
}

func (c *DeployStatColl) Get(args *DeployStatGetOption) (*models.DeployStat, error) {
	ret := new(models.DeployStat)
	query := bson.M{"max_deploy_service_num": args.MaxDeployServiceNum, "max_deploy_service_name": args.ServiceName}
	err := c.FindOne(context.TODO(), query).Decode(ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *DeployStatColl) Update(args *models.DeployStat) error {
	if args == nil {
		return errors.New("nil deployStat args")
	}

	query := bson.M{
		"date":         args.Date,
		"product_name": args.ProductName,
	}
	update := bson.M{"$set": args}
	_, err := c.UpdateOne(context.TODO(), query, update)
	return err
}

func (c *DeployStatColl) FindCount() (int, error) {
	count, err := c.CountDocuments(context.TODO(), bson.M{})
	return int(count), err
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

func (c *DeployStatColl) GetDeployTotalAndSuccessByTime(startTime, endTime int64) (int64, int64, error) {
	var result []*DeployStat
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"create_time": bson.M{
					"$gte": startTime,
					"$lte": endTime,
				},
			},
		},
		{
			"$group": bson.M{
				"_id": "null",
				"total_deploy_success": bson.M{
					"$sum": "$total_deploy_success",
				},
				"total_deploy_failure": bson.M{
					"$sum": "$total_deploy_failure",
				},
			},
		},
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return 0, 0, err
	}
	if err := cursor.All(context.TODO(), &result); err != nil {
		return 0, 0, err
	}

	var totalSuccess, totalFailure int64
	for _, res := range result {
		totalSuccess += int64(res.TotalSuccess)
		totalFailure += int64(res.TotalFailure)
	}
	return totalSuccess, totalFailure, nil
}

func (c *DeployStatColl) GetDeployStats(args *models.DeployStatOption) ([]*DeployTotalItem, error) {
	var result []*DeployTotalPipeResp
	var resp []*DeployTotalItem

	filter := bson.M{}
	if args.StartDate > 0 {
		filter["create_time"] = bson.M{"$gte": args.StartDate, "$lte": args.EndDate}
	}
	if len(args.ProductNames) > 0 {
		filter["product_name"] = bson.M{"$in": args.ProductNames}
	}

	pipeline := []bson.M{
		{
			"$match": filter,
		},
		{
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
		},
	}

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

func (c *DeployStatColl) ListDeployStat(option *models.DeployStatOption) ([]*models.DeployStat, error) {
	ret := make([]*models.DeployStat, 0)
	query := bson.M{}
	if option.Limit > 0 {
		var deployStatQuery []bson.M
		if len(option.ProductNames) > 0 {
			query["product_name"] = bson.M{"$in": option.ProductNames}
		}
		if option.StartDate > 0 {
			query["create_time"] = bson.M{
				"$gte": option.StartDate,
				"$lte": option.EndDate,
			}
		}
		deployStatQuery = append(deployStatQuery, bson.M{"$match": query})

		if option.IsMaxDeploy {
			var results []DeployPipeResp
			deployStatQuery = append(deployStatQuery,
				bson.M{"$group": bson.M{
					"_id":                    bson.M{"max_deploy_service_name": "$max_deploy_service_name"},
					"max_deploy_service_num": bson.M{"$max": "$max_deploy_service_num"}},
				})
			deployStatQuery = append(deployStatQuery, bson.M{"$sort": bson.M{"max_deploy_service_num": -1}})
			deployStatQuery = append(deployStatQuery, bson.M{"$limit": 5})
			deployStatQuery = append(deployStatQuery, bson.M{"$skip": 0})

			cursor, err := c.Aggregate(context.TODO(), deployStatQuery)
			if err != nil {
				return nil, err
			}
			if err := cursor.All(context.TODO(), &results); err != nil {
				return nil, err
			}
			for _, result := range results {
				deployItem := &models.DeployStat{
					MaxDeployServiceName: result.ID.MaxDeployServiceName,
					MaxDeployServiceNum:  result.MaxDeployServiceNum,
				}
				ret = append(ret, deployItem)
			}
		} else {
			var results []DeployFailurePipeResp

			deployStatQuery = append(deployStatQuery,
				bson.M{"$group": bson.M{
					"_id": bson.M{
						"max_deploy_failure_service_name": "$max_deploy_failure_service_name",
						"product_name":                    "$product_name",
					},
					"max_deploy_failure_service_num": bson.M{
						"$max": "$max_deploy_failure_service_num",
					},
				}})
			deployStatQuery = append(deployStatQuery, bson.M{"$sort": bson.M{"max_deploy_failure_service_num": -1}})
			deployStatQuery = append(deployStatQuery, bson.M{"$limit": 5})
			deployStatQuery = append(deployStatQuery, bson.M{"$skip": 0})
			cursor, err := c.Aggregate(context.TODO(), deployStatQuery)
			if err != nil {
				return nil, err
			}
			if err := cursor.All(context.TODO(), &results); err != nil {
				return nil, err
			}
			for _, result := range results {
				deployItem := &models.DeployStat{
					ProductName:                 result.ID.ProductName,
					MaxDeployFailureServiceName: result.ID.MaxDeployFailureServiceName,
					MaxDeployFailureServiceNum:  result.MaxDeployFailureServiceNum,
				}
				ret = append(ret, deployItem)
			}
		}
		return ret, nil
	}

	if len(option.ProductNames) > 0 {
		query["product_name"] = bson.M{"$in": option.ProductNames}
	}

	if option.StartDate > 0 {
		query["create_time"] = bson.M{"$gte": option.StartDate, "$lte": option.EndDate}
	}

	opt := &options.FindOptions{}
	if option.IsAsc {
		opt.SetSort(bson.D{{"create_time", 1}})
	} else {
		opt.SetSort(bson.D{{"create_time", -1}})
	}

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *DeployStatColl) GetDeployDailyTotal(args *models.DeployStatOption) ([]*DeployDailyItem, error) {
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
