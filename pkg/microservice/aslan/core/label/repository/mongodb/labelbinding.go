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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type LabelBindingColl struct {
	*mongo.Collection

	coll string
}

func NewLabelBindingColl() *LabelBindingColl {
	name := models.LabelBinding{}.TableName()
	return &LabelBindingColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *LabelBindingColl) GetCollectionName() string {
	return c.coll
}

func (c *LabelBindingColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "resource_name", Value: 1},
			bson.E{Key: "project_name", Value: 1},
			bson.E{Key: "label_id", Value: 1},
			bson.E{Key: "resource_type", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)
	return err
}

type LabelBinding struct {
	Resource   Resource `json:"resource" bson:"resource"`
	LabelID    string   `json:"label_id" bson:"label_id"`
	CreateBy   string   `json:"create_by" bson:"create_by"`
	CreateTime int64    `json:"create_time" bson:"create_time"`
}

func (c *LabelBindingColl) CreateMany(labelBindings []*LabelBinding) error {
	if len(labelBindings) == 0 {
		return nil
	}
	var lbs []models.LabelBinding
	for _, labelBinding := range labelBindings {
		tmplb := models.LabelBinding{
			ResourceType: labelBinding.Resource.Type,
			ResourceName: labelBinding.Resource.Name,
			ProjectName:  labelBinding.Resource.ProjectName,
			LabelID:      labelBinding.LabelID,
			CreateBy:     labelBinding.CreateBy,
			CreateTime:   time.Now().Unix(),
		}
		lbs = append(lbs, tmplb)
	}

	var ois []interface{}
	for _, obj := range lbs {
		ois = append(ois, obj)
	}
	_, err := c.InsertMany(context.TODO(), ois)
	return err
}

type LabelBindingCollFindOpt struct {
	LabelID      string
	LabelIDs     []string
	ResourceID   string
	ResourcesIDs []string
	ResourceType string
}

func (c *LabelBindingColl) FindByOpt(opt *LabelBindingCollFindOpt) (*models.LabelBinding, error) {
	res := &models.LabelBinding{}
	query := bson.M{}
	if opt.LabelID != "" {
		query["label_id"] = opt.LabelID
	}
	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *LabelBindingColl) ListByOpt(opt *LabelBindingCollFindOpt) ([]*models.LabelBinding, error) {
	var ret []*models.LabelBinding
	query := bson.M{}
	if opt.LabelID != "" {
		query["label_id"] = opt.LabelID
	}
	if len(opt.LabelIDs) != 0 {
		query["label_id"] = bson.M{"$in": opt.LabelIDs}
	}
	if opt.ResourceID != "" {
		query["resource_id"] = opt.ResourceID
	}
	if len(opt.ResourcesIDs) != 0 {
		query["resource_id"] = bson.M{"$in": opt.ResourcesIDs}
	}
	if opt.ResourceType != "" {
		query["resource_type"] = opt.ResourceType
	}
	ctx := context.Background()

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}
	return ret, err
}

func (c *LabelBindingColl) BulkDeleteByProject(projectName string) error {
	if projectName == "" {
		return nil
	}

	query := bson.M{}
	query["project_name"] = projectName
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *LabelBindingColl) BulkDeleteByLabelIds(labelIds []string) error {
	if len(labelIds) == 0 {
		return nil
	}

	condition := bson.A{}
	for _, id := range labelIds {
		condition = append(condition, bson.M{
			"label_id": id,
		})
	}
	query := bson.D{{"$or", condition}}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *LabelBindingColl) BulkDeleteByIds(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	condition := bson.A{}
	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		condition = append(condition, bson.M{
			"_id": oid,
		})
	}
	query := bson.D{{"$or", condition}}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *LabelBindingColl) BulkDelete(labelBindings []*LabelBinding) error {
	if len(labelBindings) == 0 {
		return nil
	}
	condition := bson.A{}
	for _, binding := range labelBindings {
		condition = append(condition, bson.M{
			"label_id":      binding.LabelID,
			"resource_name": binding.Resource.Name,
			"project_name":  binding.Resource.ProjectName,
			"resource_type": binding.Resource.Type,
		})
	}
	query := bson.D{{"$or", condition}}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

type Resource struct {
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
	Type        string `json:"type"`
}

type ListLabelBindingsByResources struct {
	Resources []Resource
}

func (c *LabelBindingColl) ListByResources(opt ListLabelBindingsByResources) ([]*models.LabelBinding, error) {
	var res []*models.LabelBinding

	if len(opt.Resources) == 0 {
		return nil, nil
	}
	condition := bson.A{}
	for _, resource := range opt.Resources {
		condition = append(condition, bson.M{
			"resource_type": resource.Type,
			"resource_name": resource.Name,
			"project_name":  resource.ProjectName,
		})
	}
	filter := bson.D{{"$or", condition}}
	cursor, err := c.Collection.Find(context.TODO(), filter)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}
	return res, nil
}
