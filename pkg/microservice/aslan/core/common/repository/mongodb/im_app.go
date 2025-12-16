/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mongodb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type IMAppColl struct {
	*mongo.Collection

	coll string
}

func NewIMAppColl() *IMAppColl {
	name := models.IMApp{}.TableName()
	return &IMAppColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *IMAppColl) GetCollectionName() string {
	return c.coll
}

func (c *IMAppColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	// drop unique index on app_id
	_, _ = c.Indexes().DropOne(ctx, "app_id_1")

	_, err := c.Indexes().CreateMany(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *IMAppColl) Create(ctx context.Context, args *models.IMApp) (string, error) {
	if args == nil {
		return "", errors.New("im app is nil")
	}
	args.UpdateTime = time.Now().Unix()

	res, err := c.InsertOne(ctx, args)
	if err != nil {
		return "", err
	}
	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (c *IMAppColl) List(ctx context.Context, _type string) ([]*models.IMApp, error) {
	query := bson.M{}
	resp := make([]*models.IMApp, 0)

	if _type != "" {
		query["type"] = _type
	}
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	return resp, cursor.All(ctx, &resp)
}

func (c *IMAppColl) GetByID(ctx context.Context, idString string) (*models.IMApp, error) {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	resp := new(models.IMApp)
	return resp, c.FindOne(ctx, query).Decode(resp)
}

func (c *IMAppColl) GetLarkByAppID(ctx context.Context, appID string) (*models.IMApp, error) {
	query := bson.M{"app_id": appID}

	resp := new(models.IMApp)
	return resp, c.FindOne(ctx, query).Decode(resp)
}

func (c *IMAppColl) GetDingTalkByAppKey(ctx context.Context, appKey string) (*models.IMApp, error) {
	query := bson.M{"dingtalk_app_key": appKey}

	resp := new(models.IMApp)
	return resp, c.FindOne(ctx, query).Decode(resp)
}

func (c *IMAppColl) GetWorkWXByAppID(ctx context.Context, appID string) (*models.IMApp, error) {
	realAgentID, err := strconv.Atoi(appID)
	if err != nil {
		return nil, fmt.Errorf("invalid appID")
	}
	query := bson.M{"agent_id": realAgentID}

	resp := new(models.IMApp)
	return resp, c.FindOne(ctx, query).Decode(resp)
}

func (c *IMAppColl) Update(ctx context.Context, idString string, arg *models.IMApp) error {
	if arg == nil {
		return fmt.Errorf("nil app")
	}
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return fmt.Errorf("invalid id")
	}

	arg.UpdateTime = time.Now().Unix()
	filter := bson.M{"_id": id}
	update := bson.M{"$set": arg}

	_, err = c.UpdateOne(ctx, filter, update)
	return err
}

func (c *IMAppColl) DeleteByID(ctx context.Context, idString string) error {
	id, err := primitive.ObjectIDFromHex(idString)
	if err != nil {
		return err
	}
	query := bson.M{"_id": id}

	_, err = c.DeleteOne(ctx, query)
	return err
}
