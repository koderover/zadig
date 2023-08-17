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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ProjectViewColl struct {
	*mongo.Collection

	coll string
}

func NewProjectViewColl() *ProjectViewColl {
	name := models.ProjectView{}.TableName()
	return &ProjectViewColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ProjectViewColl) GetCollectionName() string {
	return c.coll
}

func (c *ProjectViewColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *ProjectViewColl) Create(args *models.ProjectView) error {
	if args == nil {
		return errors.New("nil project view args")
	}

	args.CreatedTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *ProjectViewColl) Update(args *models.ProjectView) error {
	if args == nil {
		return errors.New("nil project view args")
	}

	filter := bson.M{"_id": args.ID}
	update := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), filter, update)
	return err
}

func (c *ProjectViewColl) Delete(name string) error {
	filter := bson.M{"name": name}
	_, err := c.DeleteOne(context.Background(), filter)
	return err
}

type ProjectViewOpts struct {
	Name string
	ID   string
}

func (c *ProjectViewColl) Find(opts ProjectViewOpts) (*models.ProjectView, error) {
	res := &models.ProjectView{}
	query := bson.M{}
	if opts.Name != "" {
		query["name"] = opts.Name
	}
	if opts.ID != "" {
		ido, err := primitive.ObjectIDFromHex(opts.ID)
		if err != nil {
			return nil, errors.New("invalid view id")
		}
		query["_id"] = ido
	}
	err := c.FindOne(context.Background(), query).Decode(&res)
	return res, err
}

func (c *ProjectViewColl) List() ([]*models.ProjectView, error) {
	var resp []*models.ProjectView
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

func (c *ProjectViewColl) ListViewNames() ([]string, error) {
	var resp []string
	ctx := context.Background()
	query := bson.M{}

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	for cursor.Next(ctx) {
		var view models.ProjectView
		if err := cursor.Decode(&view); err != nil {
			return nil, err
		}
		resp = append(resp, view.Name)
	}
	return resp, nil
}
