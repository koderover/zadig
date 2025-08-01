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

type DeliveryVersionV2Args struct {
	ID           string `json:"id"`
	ProjectName  string `json:"projectName"`
	Version      string `json:"version"`
	WorkflowName string `json:"workflowName"`
	TaskID       int    `json:"taskId"`
	PerPage      int    `json:"perPage"`
	Page         int    `json:"page"`
}

type DeliveryVersionV2Coll struct {
	*mongo.Collection

	coll string
}

func NewDeliveryVersionV2Coll() *DeliveryVersionV2Coll {
	name := models.DeliveryVersionV2{}.TableName()
	return &DeliveryVersionV2Coll{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DeliveryVersionV2Coll) GetCollectionName() string {
	return c.coll
}

func (c *DeliveryVersionV2Coll) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "version", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *DeliveryVersionV2Coll) Find(projectName, version string) (*models.DeliveryVersionV2, error) {
	if projectName == "" || version == "" {
		return nil, errors.New("nil delivery_version args")
	}

	query := bson.M{"deleted_at": 0}
	query["project_name"] = projectName
	query["version"] = version

	ctx := context.Background()
	opts := options.FindOne()
	resp := new(models.DeliveryVersionV2)
	err := c.Collection.FindOne(ctx, query, opts).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *DeliveryVersionV2Coll) FindByID(id string) (*models.DeliveryVersionV2, error) {
	if id == "" {
		return nil, errors.New("nil delivery_version args")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": oid, "deleted_at": 0}

	ctx := context.Background()
	opts := options.FindOne()
	resp := new(models.DeliveryVersionV2)
	err = c.Collection.FindOne(ctx, query, opts).Decode(&resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *DeliveryVersionV2Coll) List(args *DeliveryVersionV2Args) ([]*models.DeliveryVersionV2, int, error) {
	if args == nil {
		return nil, 0, errors.New("nil delivery_version args")
	}

	query := bson.M{"deleted_at": 0}
	if args.ProjectName != "" {
		query["project_name"] = args.ProjectName
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
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}

	resp := make([]*models.DeliveryVersionV2, 0)
	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, 0, err
	}

	return resp, int(count), nil
}

func (c *DeliveryVersionV2Coll) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid, "deleted_at": 0}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *DeliveryVersionV2Coll) DeleteByProjectName(projectName string) error {
	query := bson.M{"project_name": projectName, "deleted_at": 0}

	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionV2Coll) ListDeliveryVersions(productName string) ([]*models.DeliveryVersionV2, error) {
	var resp []*models.DeliveryVersionV2
	query := bson.M{"deleted_at": 0}
	if productName != "" {
		query["project_name"] = productName
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

func (c *DeliveryVersionV2Coll) Get(args *DeliveryVersionV2Args) (*models.DeliveryVersionV2, error) {
	if args == nil {
		return nil, errors.New("nil delivery_version args")
	}
	resp := new(models.DeliveryVersionV2)
	var query map[string]interface{}
	if args.ID != "" {
		oid, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return nil, err
		}
		query = bson.M{"_id": oid, "deleted_at": 0}
	} else if len(args.Version) > 0 {
		query = bson.M{"project_name": args.ProjectName, "version": args.Version, "deleted_at": 0}
	} else {
		query = bson.M{"project_name": args.ProjectName, "workflow_name": args.WorkflowName, "task_id": args.TaskID, "deleted_at": 0}
	}

	err := c.FindOne(context.TODO(), query).Decode(&resp)
	return resp, err
}

func (c *DeliveryVersionV2Coll) Create(args *models.DeliveryVersionV2) error {
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

func (c *DeliveryVersionV2Coll) UpdateStatusByName(versionName, projectName string, status setting.DeliveryVersionStatus, errorStr string) error {
	query := bson.M{
		"version":      versionName,
		"project_name": projectName,
		"deleted_at":   0,
	}
	change := bson.M{"$set": bson.M{
		"status": status,
		"error":  errorStr,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionV2Coll) UpdateTaskID(versionName, projectName string, taskID int32) error {
	query := bson.M{
		"version":      versionName,
		"project_name": projectName,
		"deleted_at":   0,
	}
	change := bson.M{"$set": bson.M{
		"task_id": taskID,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionV2Coll) UpdateWorkflowTask(versionName, projectName, workflowName string, taskID int32) error {
	query := bson.M{
		"version":      versionName,
		"project_name": projectName,
		"deleted_at":   0,
	}
	change := bson.M{"$set": bson.M{
		"task_id":       taskID,
		"workflow_name": workflowName,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionV2Coll) Update(args *models.DeliveryVersionV2) error {
	if args == nil {
		return errors.New("nil delivery_version args")
	}

	args.ID = primitive.NilObjectID
	query := bson.M{"project_name": args.ProjectName, "version": args.Version}
	change := bson.M{"$set": args}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionV2Coll) UpdateServiceStatus(args *models.DeliveryVersionV2) error {
	if args == nil {
		return errors.New("nil delivery_version args")
	}

	query := bson.M{"project_name": args.ProjectName, "version": args.Version, "deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"services": args.Services,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *DeliveryVersionV2Coll) FindProducts() ([]string, error) {
	resp := make([]string, 0)
	query := bson.M{"deleted_at": 0}
	ret, err := c.Distinct(context.TODO(), "project_name", query)
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
