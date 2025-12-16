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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EnvSvcDependColl struct {
	*mongo.Collection

	coll string
}

func NewEnvSvcDependColl() *EnvSvcDependColl {
	name := models.EnvSvcDepend{}.TableName()
	return &EnvSvcDependColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *EnvSvcDependColl) GetCollectionName() string {
	return c.coll
}

func (c *EnvSvcDependColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "service_name", Value: 1},
			bson.E{Key: "env_name", Value: 1},
			bson.E{Key: "product_name", Value: 1},
			bson.E{Key: "service_module", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *EnvSvcDependColl) Create(args *models.EnvSvcDepend) error {

	args.CreateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

type ListEnvSvcDependOption struct {
	IsSort        bool
	ProductName   string
	Namespace     string
	EnvName       string
	ServiceName   string
	ServiceModule string
}

func (c *EnvSvcDependColl) List(opt *ListEnvSvcDependOption) ([]*models.EnvSvcDepend, error) {
	query := bson.M{}
	if len(opt.ProductName) > 0 {
		query["product_name"] = opt.ProductName
	}
	if len(opt.Namespace) > 0 {
		query["namespace"] = opt.Namespace
	}
	if len(opt.EnvName) > 0 {
		query["env_name"] = opt.EnvName
	}
	if len(opt.ServiceName) > 0 {
		query["service_name"] = opt.ServiceName
	}
	if len(opt.ServiceModule) > 0 {
		query["service_module"] = opt.ServiceModule
	}

	var resp []*models.EnvSvcDepend
	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"create_time", -1}})
	}
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

func (c *EnvSvcDependColl) Update(id string, envSvcDepend *models.EnvSvcDepend) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"configmaps": envSvcDepend.ConfigMaps,
		"pvcs":       envSvcDepend.Pvcs,
		"secrets":    envSvcDepend.Secrets,
	}}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *EnvSvcDependColl) CreateOrUpdate(envSvcDepend *models.EnvSvcDepend) error {
	opt := &FindEnvSvcDependOption{
		ProductName:   envSvcDepend.ProductName,
		EnvName:       envSvcDepend.EnvName,
		ServiceModule: envSvcDepend.ServiceModule,
	}
	envSvcDependOlder, err := NewEnvSvcDependColl().Find(opt)
	if err != nil {
		if IsErrNoDocuments(err) {
			return NewEnvSvcDependColl().Create(envSvcDepend)
		}
		return err
	}

	oid, err := primitive.ObjectIDFromHex(envSvcDependOlder.ID.Hex())
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"configmaps": envSvcDepend.ConfigMaps,
		"pvcs":       envSvcDepend.Pvcs,
		"secrets":    envSvcDepend.Secrets,
	}}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

type FindEnvSvcDependOption struct {
	Id            string
	CreateTime    string
	ProductName   string
	Namespace     string
	EnvName       string
	ServiceName   string
	ServiceModule string
}

func (c *EnvSvcDependColl) Find(opt *FindEnvSvcDependOption) (*models.EnvSvcDepend, error) {
	if opt == nil {
		return nil, errors.New("FindEnvSvcDependOption cannot be nil")
	}
	query := bson.M{}
	if len(opt.ProductName) > 0 {
		query["product_name"] = opt.ProductName
	}
	if len(opt.Namespace) > 0 {
		query["namespace"] = opt.Namespace
	}
	if len(opt.EnvName) > 0 {
		query["env_name"] = opt.EnvName
	}
	if len(opt.ServiceName) > 0 {
		query["service_name"] = opt.ServiceName
	}
	if len(opt.ServiceModule) > 0 {
		query["service_module"] = opt.ServiceModule
	}
	if len(opt.Id) > 0 {
		oid, err := primitive.ObjectIDFromHex(opt.Id)
		if err != nil {
			return nil, err
		}
		query["_id"] = oid
	}
	opts := options.FindOne()
	if len(opt.CreateTime) > 0 {
		query["create_time"] = opt.CreateTime
	} else {
		opts.SetSort(bson.D{{"create_time", -1}})
	}

	rs := &models.EnvSvcDepend{}
	err := c.FindOne(context.TODO(), query, opts).Decode(&rs)
	if err != nil {
		return nil, err
	}
	return rs, err
}

type DeleteEnvSvcDependOption struct {
	Id            string
	CreateTime    string
	ProductName   string
	Namespace     string
	EnvName       string
	ServiceName   string
	ServiceModule string
}

func (c *EnvSvcDependColl) Delete(opt *DeleteEnvSvcDependOption) error {
	query := bson.M{}
	if len(opt.ProductName) > 0 {
		query["product_name"] = opt.ProductName
	}
	if len(opt.Namespace) > 0 {
		query["namespace"] = opt.Namespace
	}
	if len(opt.EnvName) > 0 {
		query["env_name"] = opt.EnvName
	}
	if len(opt.ServiceName) > 0 {
		query["service_name"] = opt.ServiceName
	}
	if len(opt.ServiceModule) > 0 {
		query["service_module"] = opt.ServiceModule
	}
	if len(opt.Id) > 0 {
		oid, err := primitive.ObjectIDFromHex(opt.Id)
		if err != nil {
			return err
		}
		query["_id"] = oid
	}

	_, err := c.DeleteOne(context.TODO(), query)
	return err
}
