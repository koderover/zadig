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
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type DeployListOption struct {
	Name         string
	ProjectName  string
	ServiceName  string
	IsSort       bool
	PrivateKeyID string
	PageNum      int64
	PageSize     int64
}

// FindOption ...
type DeployFindOption struct {
	Name        string
	ServiceName string
	ProjectName string
}

type DeployColl struct {
	*mongo.Collection

	coll string
}

func NewDeployColl() *DeployColl {
	name := models.Deploy{}.TableName()
	return &DeployColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *DeployColl) GetCollectionName() string {
	return c.coll
}

func (c *DeployColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "project_name", Value: 1},
			bson.E{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *DeployColl) Find(opt *DeployFindOption) (*models.Deploy, error) {
	if opt == nil {
		return nil, errors.New("nil FindOption")
	}

	query := bson.M{}

	if len(opt.Name) != 0 {
		query["name"] = opt.Name
	}

	if len(opt.ServiceName) > 0 {
		query["service_name"] = opt.ServiceName
	}

	if len(opt.ProjectName) != 0 {
		query["project_name"] = opt.ProjectName
	}

	if len(query) == 0 {
		return nil, errors.New("empty query")
	}

	resp := new(models.Deploy)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DeployColl) List(opt *DeployListOption) ([]*models.Deploy, error) {
	if opt == nil {
		return nil, errors.New("nil ListOption")
	}

	query := bson.M{}
	if len(strings.TrimSpace(opt.Name)) != 0 {
		query["name"] = opt.Name
	}

	if len(opt.ServiceName) > 0 {
		query["service_name"] = opt.ServiceName
	}

	if len(opt.ProjectName) != 0 {
		query["project_name"] = opt.ProjectName
	}
	if len(opt.PrivateKeyID) != 0 {
		query["ssh.id"] = opt.PrivateKeyID
	}

	var resp []*models.Deploy
	ctx := context.Background()
	opts := options.Find()
	if opt.IsSort {
		opts.SetSort(bson.D{{"update_time", -1}})
	}
	if opt.PageNum > 0 && opt.PageSize > 0 {
		opts.SetSkip((opt.PageNum - 1) * opt.PageSize)
		opts.SetLimit(opt.PageSize)
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

func (c *DeployColl) Delete(projectName, name string) error {
	query := bson.M{}
	if len(name) != 0 {
		query["name"] = name
	}
	if len(projectName) != 0 {
		query["project_name"] = projectName
	}

	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *DeployColl) Create(deploy *models.Deploy) error {
	if deploy == nil {
		return errors.New("nil Module args")
	}

	deploy.Name = strings.TrimSpace(deploy.Name)
	deploy.UpdateTime = time.Now().Unix()

	//double check
	deployModel, err := c.Find(&DeployFindOption{Name: deploy.Name})
	if err == nil {
		return fmt.Errorf("%s%s", deployModel.ProjectName, "项目中有相同的部署名称存在,请检查!")
	}

	_, err = c.Collection.InsertOne(context.TODO(), deploy)

	return err
}

func (c *DeployColl) Update(deploy *models.Deploy) error {
	if deploy == nil {
		return errors.New("nil Module args")
	}

	query := bson.M{
		"name":         deploy.Name,
		"project_name": deploy.ProjectName,
	}

	updateBuild := bson.M{"$set": deploy}

	_, err := c.Collection.UpdateOne(context.TODO(), query, updateBuild)
	return err
}

func (c *DeployColl) ListByCursor(opt *DeployListOption) (*mongo.Cursor, error) {
	query := bson.M{}

	return c.Collection.Find(context.TODO(), query)
}

func (c *DeployColl) Exist(opt *DeployFindOption) (bool, error) {
	if opt == nil {
		return false, errors.New("nil FindOption")
	}

	query := bson.M{}

	if len(opt.Name) != 0 {
		query["name"] = opt.Name
	}

	if len(opt.ServiceName) > 0 {
		query["service_name"] = opt.ServiceName
	}

	if len(opt.ProjectName) != 0 {
		query["project_name"] = opt.ProjectName
	}

	if len(query) == 0 {
		return false, errors.New("empty query")
	}

	resp := new(models.Deploy)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
