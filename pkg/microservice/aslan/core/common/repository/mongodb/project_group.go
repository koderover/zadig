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
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ProjectGroupColl struct {
	*mongo.Collection

	coll string
}

func NewProjectGroupColl() *ProjectGroupColl {
	name := models.ProjectGroup{}.TableName()
	return &ProjectGroupColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ProjectGroupColl) GetCollectionName() string {
	return c.coll
}

func (c *ProjectGroupColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *ProjectGroupColl) Create(args *models.ProjectGroup) error {
	if args == nil {
		return errors.New("nil project group args")
	}

	args.CreatedTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *ProjectGroupColl) Update(args *models.ProjectGroup) error {
	if args == nil {
		return errors.New("nil project group args")
	}

	filter := bson.M{"_id": args.ID}
	update := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *ProjectGroupColl) Delete(name string) error {
	filter := bson.M{"name": name}
	_, err := c.DeleteOne(context.Background(), filter)
	return err
}

type ProjectGroupOpts struct {
	Name string
	ID   string
}

func (c *ProjectGroupColl) Find(opts ProjectGroupOpts) (*models.ProjectGroup, error) {
	res := &models.ProjectGroup{}
	query := bson.M{}
	if opts.Name != "" {
		query["name"] = opts.Name
	}
	if opts.ID != "" {
		ido, err := primitive.ObjectIDFromHex(opts.ID)
		if err != nil {
			return nil, errors.New("invalid group id")
		}
		query["_id"] = ido
	}
	err := c.FindOne(context.Background(), query).Decode(&res)
	return res, err
}

func (c *ProjectGroupColl) List() ([]*models.ProjectGroup, error) {
	var resp []*models.ProjectGroup
	ctx := context.Background()
	query := bson.M{}

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := cursor.All(ctx, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *ProjectGroupColl) ListGroupNames() ([]string, error) {
	var resp []string
	ctx := context.Background()
	query := bson.M{}

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	for cursor.Next(ctx) {
		var group models.ProjectGroup
		if err := cursor.Decode(&group); err != nil {
			return nil, err
		}
		resp = append(resp, group.Name)
	}
	return resp, nil
}
