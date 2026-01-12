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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type FindRegOps struct {
	ID          string `json:"id"`
	RegAddr     string `json:"reg_addr"`
	RegType     string `json:"reg_type"`
	RegProvider string `json:"reg_provider"`
	IsDefault   bool   `json:"is_default"`
	Namespace   string `json:"namespace"`
}

type RegistryNamespaceColl struct {
	*mongo.Collection

	coll string
}

func NewRegistryNamespaceColl() *RegistryNamespaceColl {
	name := models.RegistryNamespace{}.TableName()
	coll := &RegistryNamespaceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (r *RegistryNamespaceColl) GetCollectionName() string {
	return r.coll
}

func (r *RegistryNamespaceColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "org_id", Value: 1},
			bson.E{Key: "is_default", Value: 1},
			bson.E{Key: "reg_type", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := r.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (opt FindRegOps) getQuery() bson.M {
	query := bson.M{}

	if opt.ID != "" {
		oid, err := primitive.ObjectIDFromHex(opt.ID)
		if err == nil {
			query["_id"] = oid
		}
	}

	if opt.IsDefault {
		query["is_default"] = true
	}

	if opt.RegType != "" {
		query["reg_type"] = opt.RegType
	}

	if opt.RegAddr != "" {
		query["reg_addr"] = opt.RegAddr
	}

	if opt.RegProvider != "" {
		query["reg_provider"] = opt.RegProvider
	}

	if opt.Namespace != "" {
		query["namespace"] = opt.Namespace
	}
	return query
}

func (r *RegistryNamespaceColl) Find(opt *FindRegOps) (*models.RegistryNamespace, error) {
	query := opt.getQuery()

	res := &models.RegistryNamespace{}
	err := r.FindOne(context.TODO(), query).Decode(res)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return res, nil
}

func (r *RegistryNamespaceColl) FindAll(opt *FindRegOps) ([]*models.RegistryNamespace, error) {
	query := opt.getQuery()

	ctx := context.Background()
	opts := options.Find()
	resp := make([]*models.RegistryNamespace, 0)

	cursor, err := r.Collection.Find(ctx, query, opts)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}
