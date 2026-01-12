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
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type LabelBindingColl struct {
	*mongo.Collection
	coll string
}

func NewLabelBindingColl() *LabelBindingColl {
	name := models.LabelBinding{}.TableName()
	return &LabelBindingColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *LabelBindingColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelBindingColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "project_key", Value: 1},
				bson.E{Key: "production", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "label_id", Value: 1},
				bson.E{Key: "value", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		// each key can only be assigned to a service once
		{
			Keys: bson.D{
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "project_key", Value: 1},
				bson.E{Key: "production", Value: 1},
				bson.E{Key: "label_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *LabelBindingColl) Create(args *models.LabelBinding) error {
	if args == nil {
		return fmt.Errorf("given labelbinding is nil")
	}

	args.CreatedAt = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

type LabelBindingListOption struct {
	ServiceName string
	ProjectKey  string
	Production  *bool
	LabelFilter map[string]string
	LabelID     string
}

func (c *LabelBindingColl) List(opt *LabelBindingListOption) ([]*models.LabelBinding, error) {
	var bindings []*models.LabelBinding

	query := bson.M{}

	if len(opt.ServiceName) > 0 {
		query["service_name"] = opt.ServiceName
	}

	if len(opt.LabelID) > 0 {
		query["label_id"] = opt.LabelID
	}

	if len(opt.ProjectKey) > 0 {
		query["project_key"] = opt.ProjectKey
	}

	if opt.Production != nil {
		query["production"] = *opt.Production
	}

	cursor, err := c.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &bindings)
	if err != nil {
		return nil, err
	}

	return bindings, err
}

func (c *LabelBindingColl) ListService(opt *LabelBindingListOption) ([]*models.LabelBinding, error) {
	var bindings []*models.LabelBinding

	query := bson.M{}

	labelMatch := make([]bson.M, 0)

	if opt.LabelFilter != nil && len(opt.LabelFilter) > 0 {
		for filterKey, val := range opt.LabelFilter {
			labelMatch = append(labelMatch,
				bson.M{
					"label_id": filterKey,
					"value":    val,
				})
		}
	}

	searchQueryFlag := false

	if len(opt.ServiceName) > 0 {
		searchQueryFlag = true
		query["service_name"] = opt.ServiceName
	}

	if len(opt.LabelID) > 0 {
		query["label_id"] = opt.LabelID
	}

	if len(opt.ProjectKey) > 0 {
		searchQueryFlag = true
		query["project_key"] = opt.ProjectKey
	}

	if opt.Production != nil {
		searchQueryFlag = true
		query["production"] = *opt.Production
	}

	andQuery := make([]bson.M, 0)

	if searchQueryFlag {
		andQuery = append(andQuery, query)
	}

	if opt.LabelFilter != nil && len(opt.LabelFilter) > 0 {
		andQuery = append(andQuery, bson.M{
			"$or": labelMatch,
		})
	}

	finalQuery := bson.M{}

	if len(andQuery) > 0 {
		finalQuery = bson.M{
			"$and": andQuery,
		}
	}

	pipeline := []bson.M{
		{
			"$match": finalQuery,
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"service_name": "$service_name",
					"project_key":  "$project_key",
					"production":   "$production",
				},
				"count": bson.M{"$sum": 1},
			},
		},
	}

	if opt.LabelFilter != nil && len(opt.LabelFilter) > 0 {
		pipeline = append(pipeline, bson.M{
			"$match": bson.M{
				"count": len(opt.LabelFilter),
			},
		})
	}

	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"_id":          0,
			"service_name": "$_id.service_name",
			"project_key":  "$_id.project_key",
			"production":   "$_id.production",
		},
	})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &bindings)
	if err != nil {
		return nil, err
	}

	return bindings, err
}

func (c *LabelBindingColl) GetByID(id string) (*models.LabelBinding, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	res := &models.LabelBinding{}
	err = c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}

func (c *LabelBindingColl) Update(id string, args *models.LabelBinding) error {
	bindingID, err := primitive.ObjectIDFromHex(id)

	if err != nil {
		return err
	}
	_, err = c.UpdateOne(context.TODO(),
		bson.M{"_id": bindingID}, bson.M{"$set": bson.M{
			"value":      args.Value,
			"updated_at": time.Now().Unix(),
		}},
	)

	return err
}

func (c *LabelBindingColl) DeleteByID(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *LabelBindingColl) DeleteByService(serviceName, projectKey string, production bool) error {
	query := bson.M{
		"service_name": serviceName,
		"project_key":  projectKey,
		"production":   production,
	}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *LabelBindingColl) DeleteByLabelID(id string) error {
	query := bson.M{
		"label_id": id,
	}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}
