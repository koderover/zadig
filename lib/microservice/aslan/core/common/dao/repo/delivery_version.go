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

package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type DeliveryVersionArgs struct {
	ID           string `json:"id"`
	OrgID        int    `json:"orgId"`
	ProductName  string `json:"productName"`
	WorkflowName string `json:"workflowName"`
	TaskID       int    `json:"taskId"`
	PerPage      int    `json:"perPage"`
	Page         int    `json:"page"`
}

type DeliveryVersionColl struct {
	*mongo.Collection

	coll string
}

func NewDeliveryVersionColl() *DeliveryVersionColl {
	name := models.DeliveryVersion{}.TableName()
	return &DeliveryVersionColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DeliveryVersionColl) GetCollectionName() string {
	return c.coll
}

func (c *DeliveryVersionColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "org_id", Value: 1},
				bson.E{Key: "workflow_name", Value: 1},
				bson.E{Key: "version", Value: 1},
				bson.E{Key: "deleted_at", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "org_id", Value: 1},
				bson.E{Key: "task_id", Value: 1},
				bson.E{Key: "deleted_at", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *DeliveryVersionColl) Find(args *DeliveryVersionArgs) ([]*models.DeliveryVersion, error) {
	if args == nil {
		return nil, errors.New("nil delivery_version args")
	}

	var resp []*models.DeliveryVersion
	query := bson.M{"org_id": args.OrgID, "deleted_at": 0}
	if args.ProductName != "" {
		query["product_name"] = args.ProductName
	}
	if args.WorkflowName != "" {
		query["workflow_name"] = args.WorkflowName
	}
	if args.TaskID != 0 {
		query["task_id"] = args.TaskID
	}

	ctx := context.Background()
	opts := options.Find()
	if args.Page > 0 {
		opts.SetSort(bson.D{{"created_at", -1}})
		opts.SetSkip(int64(args.PerPage * (args.Page - 1)))
		opts.SetLimit(int64(args.PerPage))
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

func (c *DeliveryVersionColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid, "deleted_at": 0}

	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionColl) ListDeliveryVersions(productName string, orgId int) ([]*models.DeliveryVersion, error) {
	var resp []*models.DeliveryVersion
	query := bson.M{"org_id": orgId, "deleted_at": 0}
	if productName != "" {
		query["product_name"] = productName
	}

	ctx := context.Background()
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *DeliveryVersionColl) Get(args *DeliveryVersionArgs) (*models.DeliveryVersion, error) {
	if args == nil {
		return nil, errors.New("nil delivery_version args")
	}
	resp := new(models.DeliveryVersion)
	var query map[string]interface{}
	if args.ID != "" {
		query = bson.M{"_id": args.ID, "deleted_at": 0}
	} else {
		query = bson.M{"org_id": args.OrgID, "product_name": args.ProductName, "workflow_name": args.WorkflowName, "task_id": args.TaskID, "deleted_at": 0}
	}

	err := c.FindOne(context.TODO(), query).Decode(&resp)
	return resp, err
}

func (c *DeliveryVersionColl) Insert(args *models.DeliveryVersion) error {
	if args == nil {
		return errors.New("nil delivery_version args")
	}

	result, err := c.InsertOne(context.TODO(), args)
	if err != nil || result == nil {
		return err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		args.ID = oid
	}

	return nil
}

func (c *DeliveryVersionColl) Update(args *models.DeliveryVersion) error {
	if args == nil {
		return errors.New("nil delivery_version args")
	}
	query := bson.M{"_id": args.ID, "deleted_at": 0}

	change := bson.M{"$set": bson.M{
		"product_env_info": args.ProductEnvInfo,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionColl) FindProducts(orgID int) ([]string, error) {
	resp := make([]string, 0)
	query := bson.M{"org_id": orgID, "deleted_at": 0}
	ret, err := c.Distinct(context.TODO(), "product_name", query)
	if err != nil {
		return nil, err
	}
	for _, obj := range ret {
		if version, ok := obj.(string); ok {
			resp = append(resp, version)
		}
	}
	return resp, err
}
