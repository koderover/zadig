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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ServicesInExternalEnvColl struct {
	*mongo.Collection
	mongo.Session
	coll string
}

func NewServicesInExternalEnvColl() *ServicesInExternalEnvColl {
	name := models.ServicesInExternalEnv{}.TableName()
	return &ServicesInExternalEnvColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewServiceInExternalEnvWithSess(session mongo.Session) *ServicesInExternalEnvColl {
	name := models.ServicesInExternalEnv{}.TableName()
	return &ServicesInExternalEnvColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *ServicesInExternalEnvColl) GetCollectionName() string {
	return c.coll
}

func (c *ServicesInExternalEnvColl) EnsureIndex(ctx context.Context) error {
	mods := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "product_name", Value: 1},
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "service_name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "namespace", Value: 1},
				bson.E{Key: "cluster_id", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mods, mongotool.CreateIndexOptions(ctx))

	return err
}

// Create ...
func (c *ServicesInExternalEnvColl) Create(args *models.ServicesInExternalEnv) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil ServicesInExternalEnv")
	}
	_, err := c.InsertOne(mongotool.SessionContext(context.TODO(), c.Session), args)
	return err
}

type ServicesInExternalEnvArgs struct {
	ProductName    string
	ServiceName    string
	EnvName        string
	Namespace      string
	ClusterID      string
	ExcludeEnvName string
}

func (c *ServicesInExternalEnvColl) List(args *ServicesInExternalEnvArgs) ([]*models.ServicesInExternalEnv, error) {
	if args == nil {
		return nil, errors.New("nil ServicesInExternalEnvArgs")
	}
	query := bson.M{}
	if args.ProductName != "" {
		query["product_name"] = args.ProductName
	}

	if args.EnvName != "" {
		query["env_name"] = args.EnvName
	}

	if args.ExcludeEnvName != "" {
		query["env_name"] = bson.M{"$ne": args.ExcludeEnvName}
	}

	if args.Namespace != "" {
		query["namespace"] = args.Namespace
	}

	if args.ClusterID != "" {
		query["cluster_id"] = args.ClusterID
	}

	resp := make([]*models.ServicesInExternalEnv, 0)
	ctx := mongotool.SessionContext(context.Background(), c.Session)

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *ServicesInExternalEnvColl) Delete(args *ServicesInExternalEnvArgs) error {
	if args == nil {
		return errors.New("nil ServicesInExternalEnvArgs")
	}
	query := bson.M{}
	if args.ProductName != "" {
		query["product_name"] = args.ProductName
	}

	if args.EnvName != "" {
		query["env_name"] = args.EnvName
	}

	if args.ServiceName != "" {
		query["service_name"] = args.ServiceName
	}

	_, err := c.DeleteMany(mongotool.SessionContext(context.TODO(), c.Session), query)
	return err
}
