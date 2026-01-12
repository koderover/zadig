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
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type SAEEnvColl struct {
	*mongo.Collection
	mongo.Session

	coll string
}

type SAEEnvFindOptions struct {
	ProjectName       string
	EnvName           string
	Namespace         string
	Production        *bool
	IgnoreNotFoundErr bool
}

type SAEEnvListOptions struct {
	EnvName             string
	ProjectName         string
	Production          *bool
	IsSortByUpdateTime  bool
	IsSortByProductName bool
	InEnvs              []string
	InIDs               []string
}

func NewSAEEnvColl() *SAEEnvColl {
	name := models.SAEEnv{}.TableName()
	return &SAEEnvColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func NewSAEEnvWithSession(session mongo.Session) *SAEEnvColl {
	name := models.Product{}.TableName()
	return &SAEEnvColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *SAEEnvColl) GetCollectionName() string {
	return c.coll
}

func (c *SAEEnvColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "production", Value: 1},
				bson.E{Key: "update_time", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("idx_project_production"),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "production", Value: 1},
				bson.E{Key: "update_time", Value: 1},
			},
			Options: options.Index().SetUnique(false).SetName("idx_project_env_production_time"),
		},
	}

	c.Indexes().DropOne(ctx, "env_name_1_project_name_1")
	c.Indexes().DropOne(ctx, "env_name_1_project_name_1_update_time_1")

	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *SAEEnvColl) Find(opt *SAEEnvFindOptions) (*models.SAEEnv, error) {
	res := &models.SAEEnv{}
	query := bson.M{}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	if opt.EnvName != "" {
		query["env_name"] = opt.EnvName
	}

	err := c.FindOne(mongotool.SessionContext(context.TODO(), c.Session), query).Decode(res)
	if err != nil && mongo.ErrNoDocuments == err && opt.IgnoreNotFoundErr {
		return nil, nil
	}
	return res, err
}

func (c *SAEEnvColl) List(opt *SAEEnvListOptions) ([]*models.SAEEnv, error) {
	var ret []*models.SAEEnv
	query := bson.M{}

	if opt == nil {
		opt = &SAEEnvListOptions{}
	}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	if opt.EnvName != "" {
		query["env_name"] = opt.EnvName
	} else if len(opt.InEnvs) > 0 {
		query["env_name"] = bson.M{"$in": opt.InEnvs}
	}
	if opt.Production != nil {
		query["production"] = *opt.Production
	}
	if len(opt.InIDs) > 0 {
		var oids []primitive.ObjectID
		for _, id := range opt.InIDs {
			oid, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				return nil, err
			}
			oids = append(oids, oid)
		}
		query["_id"] = bson.M{"$in": oids}
	}

	ctx := context.Background()
	opts := options.Find()
	if opt.IsSortByUpdateTime {
		opts.SetSort(bson.D{{"update_time", -1}})
	}
	if opt.IsSortByProductName {
		opts.SetSort(bson.D{{"project_name", 1}})
	}
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *SAEEnvColl) ListProjectsInNames(names []string) ([]*projectEnvs, error) {
	var res []*projectEnvs
	var pipeline []bson.M
	if len(names) > 0 {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"project_name": bson.M{"$in": names}}})
	}

	pipeline = append(pipeline,
		bson.M{
			"$group": bson.M{
				"_id": bson.M{
					"project_name": "$project_name",
				},
				"project_name": bson.M{"$last": "$project_name"},
				"envs":         bson.M{"$push": "$env_name"},
			},
		},
	)

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *SAEEnvColl) Delete(productName, envName string) error {
	query := bson.M{"env_name": envName, "project_name": productName}
	_, err := c.DeleteOne(context.TODO(), query)

	return err
}

// Update  Cannot update owner & product name
func (c *SAEEnvColl) Update(args *models.SAEEnv) error {
	query := bson.M{"env_name": args.EnvName, "project_name": args.ProjectName}
	changePayload := bson.M{
		"update_time": time.Now().Unix(),
	}
	change := bson.M{"$set": changePayload}
	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	return err
}

func (c *SAEEnvColl) Create(args *models.SAEEnv) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil Product")
	}

	now := time.Now().Unix()
	args.CreateTime = now
	args.UpdateTime = now
	_, err := c.InsertOne(mongotool.SessionContext(context.TODO(), c.Session), args)

	return err
}

func (c *SAEEnvColl) Count(projectName string) (int, error) {
	num, err := c.CountDocuments(context.TODO(), bson.M{"project_name": projectName, "status": bson.M{"$ne": setting.ProductStatusDeleting}})

	return int(num), err
}
