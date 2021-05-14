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

package template

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/template"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ProductColl struct {
	*mongo.Collection

	coll string
}

func NewProductColl() *ProductColl {
	name := template.Product{}.TableName()
	return &ProductColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ProductColl) GetCollectionName() string {
	return c.coll
}

func (c *ProductColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"product_name": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *ProductColl) Find(productName string) (*template.Product, error) {
	res := &template.Product{}
	query := bson.M{"product_name": productName}
	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *ProductColl) FindProjectName(project string) (*template.Product, error) {
	resp := &template.Product{}
	query := bson.M{"project_name": project}
	err := c.FindOne(context.TODO(), query).Decode(resp)
	return resp, err
}

func (c *ProductColl) List(productName string) ([]*template.Product, error) {
	var resp []*template.Product
	query := bson.M{}
	if productName != "" {
		query["product_name"] = productName
	}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type ProductListOpt struct {
	IsOpensource string
}

// ListWithOption ...
func (c *ProductColl) ListWithOption(opt *ProductListOpt) ([]*template.Product, error) {
	var resp []*template.Product

	query := bson.M{}
	if opt.IsOpensource != "" {
		query["is_opensource"] = stringToBool(opt.IsOpensource)
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// TODO: make it common
func stringToBool(source string) bool {
	return source == "true"
}

func (c *ProductColl) Create(args *template.Product) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil ProductTmpl")
	}

	args.ProjectName = strings.TrimSpace(args.ProjectName)
	args.ProductName = strings.TrimSpace(args.ProductName)

	now := time.Now().Unix()
	args.CreateTime = now
	args.UpdateTime = now

	//增加double check
	_, err := c.Find(args.ProductName)
	if err == nil {
		return errors.New("有相同的项目主键存在,请检查")
	}

	if args.ProjectName != "" {
		_, err = c.FindProjectName(args.ProjectName)
		if err == nil {
			return errors.New("有相同的项目名称存在,请检查")
		}
	}

	_, err = c.InsertOne(context.TODO(), args)
	return err
}

// Update existing ProductTmpl
func (c *ProductColl) Update(productName string, args *template.Product) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil ProductTmpl")
	}

	args.ProjectName = strings.TrimSpace(args.ProjectName)

	query := bson.M{"product_name": productName}
	change := bson.M{"$set": bson.M{
		"project_name": strings.TrimSpace(args.ProjectName),
		"revision":     args.Revision,
		"services":     args.Services,
		"update_time":  time.Now().Unix(),
		"update_by":    args.UpdateBy,
		"teams":        args.Teams,
		"enabled":      args.Enabled,
		"description":  args.Description,
		"visibility":   args.Visibility,
		"user_ids":     args.UserIDs,
		"team_id":      args.TeamID,
		"timeout":      args.Timeout,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductColl) UpdateOnboardingStatus(productName string, status int) error {
	query := bson.M{"product_name": productName}
	change := bson.M{"$set": bson.M{
		"onboarding_status": status,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductColl) Delete(productName string) error {
	query := bson.M{"product_name": productName}

	_, err := c.DeleteOne(context.TODO(), query)

	return err
}
