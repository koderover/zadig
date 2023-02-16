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
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ProductionServiceColl struct {
	*mongo.Collection
	coll string
}

func NewProductionServiceColl() *ProductionServiceColl {
	name := "production_template_service"
	return &ProductionServiceColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ProductionServiceColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "product_name", Value: 1},
			bson.E{Key: "service_name", Value: 1},
			bson.E{Key: "revision", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

func (c *ProductionServiceColl) GetCollectionName() string {
	return c.coll
}

func (c *ProductionServiceColl) Create(args *models.Service) error {
	_, err := c.InsertOne(context.Background(), args)
	return err
}

func (c *ProductionServiceColl) Delete(serviceName, productName, status string, revision int64) error {
	query := bson.M{}
	query["service_name"] = serviceName
	query["product_name"] = productName
	query["revision"] = revision

	if status != "" {
		query["status"] = status
	}
	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

type ProductionServiceOption struct {
	ProductName   string
	ExcludeStatus string
}

func (c *ProductionServiceColl) Find(opt *ServiceFindOption) (*models.Service, error) {
	if opt == nil {
		return nil, errors.New("nil ConfigFindOption")
	}
	if opt.ServiceName == "" {
		return nil, errors.New("ServiceName is empty")
	}

	query := bson.M{}
	query["service_name"] = opt.ServiceName
	query["product_name"] = opt.ProductName

	service := new(models.Service)
	if opt.Type != "" {
		query["type"] = opt.Type
	}
	if opt.ExcludeStatus != "" {
		query["status"] = bson.M{"$ne": opt.ExcludeStatus}
	}

	opts := options.FindOne()
	if opt.Revision > 0 {
		query["revision"] = opt.Revision
	} else {
		opts.SetSort(bson.D{{"revision", -1}})
	}

	err := c.FindOne(context.TODO(), query, opts).Decode(service)
	if err != nil {
		if err == mongo.ErrNoDocuments && opt.IgnoreNoDocumentErr {
			return nil, nil
		}
		return nil, err
	}

	return service, nil
}

func (c *ProductionServiceColl) ListMaxRevisions(serviceType string) ([]*models.Service, error) {
	pre := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}
	if serviceType != "" {
		pre["type"] = serviceType
	}
	return c.listMaxRevisions(pre, nil)
}

func (c *ProductionServiceColl) UpdateServiceVariables(args *models.Service) error {
	if args == nil {
		return errors.New("nil ServiceTmplObject")
	}
	args.ProductName = strings.TrimSpace(args.ProductName)
	args.ServiceName = strings.TrimSpace(args.ServiceName)

	query := bson.M{"product_name": args.ProductName, "service_name": args.ServiceName, "revision": args.Revision}
	changeMap := bson.M{
		"variable_yaml": args.VariableYaml,
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductionServiceColl) UpdateServiceContainers(args *models.Service) error {
	if args == nil {
		return errors.New("nil ServiceTmplObject")
	}
	args.ProductName = strings.TrimSpace(args.ProductName)
	args.ServiceName = strings.TrimSpace(args.ServiceName)

	query := bson.M{"product_name": args.ProductName, "service_name": args.ServiceName, "revision": args.Revision}
	changeMap := bson.M{
		"containers": args.Containers,
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductionServiceColl) UpdateStatus(serviceName, productName, status string) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName is empty")
	}
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	query := bson.M{"service_name": serviceName, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"status": status,
	}}

	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

func (c *ProductionServiceColl) listMaxRevisions(preMatch, postMatch bson.M) ([]*models.Service, error) {
	var pipeResp []*grouped
	pipeline := []bson.M{
		{
			"$match": preMatch,
		},
		{
			"$sort": bson.M{"revision": 1},
		},
		{
			"$group": bson.M{
				"_id": bson.D{
					{"product_name", "$product_name"},
					{"service_name", "$service_name"},
				},
				"service_id": bson.M{"$last": "$_id"},
			},
		},
	}

	if len(postMatch) > 0 {
		pipeline = append(pipeline, bson.M{
			"$match": postMatch,
		})
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	if err := cursor.All(context.TODO(), &pipeResp); err != nil {
		return nil, err
	}

	if len(pipeResp) == 0 {
		return nil, nil
	}

	var resp []*models.Service
	var ids []primitive.ObjectID
	for _, p := range pipeResp {
		ids = append(ids, p.ServiceID)
	}

	query := bson.M{"_id": bson.M{"$in": ids}}
	cursor, err = c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
