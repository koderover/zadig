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
	"time"

	"github.com/koderover/zadig/v2/pkg/setting"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type CICDToolIntegrationColl struct {
	*mongo.Collection

	coll string
}

func NewCICDToolColl() *CICDToolIntegrationColl {
	name := models.JenkinsIntegration{}.TableName()
	coll := &CICDToolIntegrationColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *CICDToolIntegrationColl) GetCollectionName() string {
	return c.coll
}

func (c *CICDToolIntegrationColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetSparse(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *CICDToolIntegrationColl) Get(id string) (*models.JenkinsIntegration, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	res := &models.JenkinsIntegration{}
	err = c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}

func (c *CICDToolIntegrationColl) Create(args *models.JenkinsIntegration) error {
	if args == nil {
		return errors.New("nil jenkins integration args")
	}

	args.UpdatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *CICDToolIntegrationColl) Update(ID string, args *models.JenkinsIntegration) error {
	oldID, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return err
	}

	changeQuery := bson.M{
		"update_by":  args.UpdateBy,
		"updated_at": time.Now().Unix(),
		"name":       args.Name,
	}

	if args.Type == setting.CICDToolTypeJenkins {
		changeQuery["url"] = args.URL
		changeQuery["username"] = args.Username
		changeQuery["password"] = args.Password
	}

	if args.Type == setting.CICDToolTypeBlueKing {
		changeQuery["host"] = args.Host
		changeQuery["app_code"] = args.AppCode
		changeQuery["app_secret"] = args.AppSecret
		changeQuery["bk_username"] = args.BKUserName
	}

	query := bson.M{"_id": oldID}
	change := bson.M{"$set": changeQuery}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *CICDToolIntegrationColl) Delete(ID string) error {
	oldID, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oldID}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}

func (c *CICDToolIntegrationColl) List(toolType string) ([]*models.JenkinsIntegration, error) {
	resp := make([]*models.JenkinsIntegration, 0)
	query := bson.M{}
	if toolType != "" && toolType != setting.CICDToolTypeJenkins {
		query["type"] = toolType
	}

	// compatibility code
	if toolType == setting.CICDToolTypeJenkins {
		query["$or"] = []bson.M{
			{
				"type": setting.CICDToolTypeJenkins,
			},
			{
				"type": bson.M{
					"$exists": false,
				},
			},
		}
	}

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
