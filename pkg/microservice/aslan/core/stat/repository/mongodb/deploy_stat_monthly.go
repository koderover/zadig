/*
Copyright 2024 The KodeRover Authors.

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
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MonthlyDeployStatColl struct {
	*mongo.Collection

	coll string
}

func NewMonthlyDeployStatColl() *MonthlyDeployStatColl {
	name := "deploy_stat_monthly"
	return &MonthlyDeployStatColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *MonthlyDeployStatColl) GetCollectionName() string {
	return c.coll
}

func (c *MonthlyDeployStatColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "project_key", Value: 1},
			bson.E{Key: "date", Value: 1},
			bson.E{Key: "production", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *MonthlyDeployStatColl) Upsert(args *models.WeeklyDeployStat) error {
	if args == nil {
		return fmt.Errorf("upsert data cannot be empty for %s", "deploy_stat_weekly")
	}

	filter := bson.M{
		"project_key": args.ProjectKey,
		"date":        args.Date,
		"production":  args.Production,
	}

	update := bson.M{
		"$set": args,
	}

	updateOptions := options.Update().SetUpsert(true)

	_, err := c.UpdateOne(context.TODO(), filter, update, updateOptions)
	return err
}

// CalculateStat returns the combined project deploy stats, grouped only by production field
func (c *MonthlyDeployStatColl) CalculateStat(startTime, endTime int64, projects []string, productionType config.ProductionType) ([]*models.WeeklyDeployStat, error) {
	query := bson.M{
		"create_time": bson.M{"$gte": startTime, "$lte": endTime},
	}

	if len(projects) > 0 {
		query["project_key"] = bson.M{"$in": projects}
	}

	switch productionType {
	case config.Production:
		query["production"] = true
	case config.Testing:
		query["production"] = false
	case config.Both:
		break
	default:
		return nil, fmt.Errorf("invlid production type: %s", productionType)
	}

	pipeline := make([]bson.M, 0)

	pipeline = append(pipeline, bson.M{
		"$match": query,
	})

	//group by production, add the fields
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"production": "$production",
				"date":       "$date",
			},
			"total_success": bson.M{
				"$sum": "$success",
			},
			"total_failed": bson.M{
				"$sum": "$failed",
			},
			"total_timeout": bson.M{
				"$sum": "$timeout",
			},
		},
	})

	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":        0,
			"production": "$_id.production",
			"date":       "$_id.date",
			"success":    "$total_success",
			"failed":     "$total_failed",
			"timeout":    "$total_timeout",
		},
	})
	pipeline = append(pipeline, bson.M{
		"$sort": bson.M{
			"date": 1,
		},
	})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	resp := make([]*models.WeeklyDeployStat, 0)
	if err := cursor.All(context.TODO(), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
