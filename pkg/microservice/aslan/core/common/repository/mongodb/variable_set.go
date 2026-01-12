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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type VariableSetColl struct {
	*mongo.Collection

	coll string
}

type VariableSetFindOption struct {
	ID          string `json:"id"`
	PerPage     int    `json:"perPage"`
	Page        int    `json:"page"`
	ProjectName string `json:"projectName"`
}

func NewVariableSetColl() *VariableSetColl {
	name := models.VariableSet{}.TableName()
	coll := &VariableSetColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *VariableSetColl) GetCollectionName() string {
	return c.coll
}

func (c *VariableSetColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "project_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *VariableSetColl) Create(args *models.VariableSet) error {
	if args == nil {
		return errors.New("nil helm repo args")
	}

	args.CreatedAt = time.Now().Unix()
	args.UpdatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *VariableSetColl) Find(opt *VariableSetFindOption) (*models.VariableSet, error) {
	query := bson.M{}
	if len(opt.ID) > 0 {
		oid, err := primitive.ObjectIDFromHex(opt.ID)
		if err != nil {
			return nil, err
		}
		query["_id"] = oid
	}
	ret := new(models.VariableSet)
	err := c.FindOne(context.TODO(), query).Decode(ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *VariableSetColl) Update(id string, args *models.VariableSet) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"name":          args.Name,
		"description":   args.Description,
		"variable_yaml": args.VariableYaml,
		"project_name":  args.ProjectName,
		"updated_by":    args.UpdatedBy,
		"updated_at":    time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *VariableSetColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}

func (c *VariableSetColl) List(option *VariableSetFindOption) (int64, []*models.VariableSet, error) {
	resp := make([]*models.VariableSet, 0)
	query := bson.M{}
	query["$or"] = []bson.M{{"project_name": bson.M{"$eq": option.ProjectName}}, {"project_name": bson.M{"$exists": false}}}

	ctx := context.Background()

	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return 0, nil, err
	}

	opts := options.Find().SetSort(bson.D{{"created_at", -1}})
	if option.Page > 0 {
		opts.SetSkip(int64(option.PerPage * (option.Page - 1)))
		opts.SetLimit(int64(option.PerPage))
	}

	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return 0, nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return 0, nil, err
	}

	return count, resp, nil
}
