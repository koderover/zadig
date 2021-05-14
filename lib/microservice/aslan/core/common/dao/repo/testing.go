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
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type ListTestOption struct {
	ProductName  string
	TestType     string
	IsSort       bool
	BuildOS      string
	BasicImageID string
}

type TestingColl struct {
	*mongo.Collection

	coll string
}

func NewTestingColl() *TestingColl {
	name := models.Testing{}.TableName()
	return &TestingColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *TestingColl) GetCollectionName() string {
	return c.coll
}

func (c *TestingColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *TestingColl) List(opt *ListTestOption) ([]*models.Testing, error) {
	query := bson.M{}

	if len(opt.ProductName) > 0 {
		query["product_name"] = opt.ProductName
	}
	if len(opt.TestType) > 0 {
		query["test_type"] = opt.TestType
	}
	if len(opt.BuildOS) > 0 {
		query["pre_test.build_os"] = opt.BuildOS
	}
	if len(opt.BasicImageID) != 0 {
		query["pre_test.image_id"] = opt.BasicImageID
	}

	var resp []*models.Testing
	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"update_time", -1}})
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

func (c *TestingColl) Find(name, productName string) (*models.Testing, error) {
	query := bson.M{}

	query["name"] = name

	if productName != "" {
		query["product_name"] = productName
	}

	resp := new(models.Testing)

	err := c.FindOne(context.TODO(), query).Decode(resp)
	return resp, err
}

func (c *TestingColl) Delete(name, productName string) error {
	query := bson.M{}
	if name != "" {
		query["name"] = name
	}
	if productName != "" {
		query["product_name"] = productName
	}
	if len(query) == 0 {
		return nil
	}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *TestingColl) ListWithScheduleEnabled() ([]*models.Testing, error) {
	resp := make([]*models.Testing, 0)
	query := bson.M{"schedule_enabled": true}

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

func (c *TestingColl) Create(testing *models.Testing) error {
	if testing == nil {
		return errors.New("nil testing args")
	}

	testing.Name = strings.TrimSpace(testing.Name)
	testing.UpdateTime = time.Now().Unix()

	test, err := c.Find(testing.Name, "")
	if err == nil {
		return fmt.Errorf("%s%s", test.ProductName, "项目中有相同的测试名称存在,请检查!")
	}

	_, err = c.InsertOne(context.TODO(), testing)
	return err
}

func (c *TestingColl) Update(testing *models.Testing) error {
	if testing == nil {
		return errors.New("nil testing args")
	}

	query := bson.M{"name": testing.Name}

	change := bson.M{"$set": testing}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
