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
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type DeliveryActivityArgs struct {
	ArtifactID string `json:"artifact_id"`
	RepoName   string `json:"repo_name"`
	Branch     string `json:"branch"`
	PerPage    int    `json:"per_page"`
	Page       int    `json:"page"`
}

type DeliveryActivityColl struct {
	*mongo.Collection

	coll string
}

func NewDeliveryActivityColl() *DeliveryActivityColl {
	name := models.DeliveryActivity{}.TableName()
	return &DeliveryActivityColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DeliveryActivityColl) GetCollectionName() string {
	return c.coll
}

func (c *DeliveryActivityColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "artifact_id", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "type", Value: 1},
				bson.E{Key: "commits", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *DeliveryActivityColl) List(args *DeliveryActivityArgs) ([]*models.DeliveryActivity, int, error) {
	var count int64
	var err error
	if args == nil {
		return nil, 0, errors.New("nil delivery_activity args")
	}

	resp := make([]*models.DeliveryActivity, 0)
	query := bson.M{}
	commitMap := bson.M{}

	opt := options.Find().SetSort(bson.D{{"created_time", -1}})

	if args.ArtifactID != "" {
		artifactId, err := primitive.ObjectIDFromHex(args.ArtifactID)
		if err != nil {
			return nil, 0, err
		}
		query["artifact_id"] = artifactId
		count = 0
	} else {
		query["type"] = "build"
		if args.RepoName != "" {
			commitMap["repo_name"] = args.RepoName
		}
		if args.Branch != "" {
			commitMap["branch"] = args.Branch
		}
		query["commits"] = bson.M{"$elemMatch": commitMap}
		count, err = c.CountDocuments(context.TODO(), query)
		if err != nil {
			return nil, 0, fmt.Errorf("find activity err:%v", err)
		}
		opt.SetSkip(int64(args.PerPage * (args.Page - 1))).SetLimit(int64(args.PerPage))
	}

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, 0, err
	}
	return resp, int(count), nil
}

func (c *DeliveryActivityColl) Insert(args *models.DeliveryActivity) error {
	if args == nil {
		return errors.New("nil delivery_activity args")
	}

	_, err := c.Collection.InsertOne(context.TODO(), args)
	return err
}

func (c *DeliveryActivityColl) InsertWithId(id string, args *models.DeliveryActivity) error {
	if args == nil {
		return errors.New("nil delivery_activity args")
	}

	artifactId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	args.ArtifactID = artifactId

	_, err = c.Collection.InsertOne(context.TODO(), args)
	return err
}
