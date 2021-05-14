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

type BuildListOption struct {
	Name         string
	Version      string
	Targets      []string
	ProductName  string
	ServiceName  string
	IsSort       bool
	BuildOS      string
	BasicImageID string
	PrivateKeyID string
}

// FindOption ...
type BuildFindOption struct {
	Name        string
	Version     string
	Targets     []string
	ServiceName string
	ProductName string
}

type BuildColl struct {
	*mongo.Collection

	coll string
}

func NewBuildColl() *BuildColl {
	name := models.Build{}.TableName()
	return &BuildColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *BuildColl) GetCollectionName() string {
	return c.coll
}

func (c *BuildColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "name", Value: 1},
			bson.E{Key: "revision", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

func (c *BuildColl) Find(opt *BuildFindOption) (*models.Build, error) {
	if opt == nil {
		return nil, errors.New("nil FindOption")
	}

	query := bson.M{}

	if len(opt.Name) != 0 {
		query["name"] = opt.Name
	}

	if len(opt.Version) != 0 {
		query["version"] = opt.Version
	}

	if len(opt.Targets) > 0 {
		query["targets.service_module"] = bson.M{"$in": opt.Targets}
	}

	//避免同项目下服务组件重名的时，有不同的构建的情况
	if len(opt.ServiceName) > 0 {
		query["targets.service_name"] = opt.ServiceName
	}

	if len(opt.ProductName) != 0 {
		query["product_name"] = opt.ProductName
	}

	resp := new(models.Build)
	err := c.Collection.FindOne(context.TODO(), query).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *BuildColl) List(opt *BuildListOption) ([]*models.Build, error) {
	if opt == nil {
		return nil, errors.New("nil ListOption")
	}

	query := bson.M{}
	if len(strings.TrimSpace(opt.Name)) != 0 {
		query["name"] = opt.Name
	}
	if len(strings.TrimSpace(opt.Version)) != 0 {
		query["version"] = opt.Version
	}
	if len(opt.Targets) > 0 {
		query["targets.service_module"] = bson.M{"$in": opt.Targets}
	}

	//避免同项目下服务组件重名的时，有不同的构建的情况
	if len(opt.ServiceName) > 0 {
		query["targets.service_name"] = opt.ServiceName
	}

	if len(opt.ProductName) != 0 {
		query["product_name"] = opt.ProductName
	}
	if len(opt.BuildOS) != 0 {
		query["pre_build.build_os"] = opt.BuildOS
	}
	if len(opt.BasicImageID) != 0 {
		query["pre_build.image_id"] = opt.BasicImageID
	}
	if len(opt.PrivateKeyID) != 0 {
		query["ssh.id"] = opt.PrivateKeyID
	}

	var resp []*models.Build
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

func (c *BuildColl) Delete(name, version, productName string) error {
	query := bson.M{}
	if len(name) != 0 {
		query["name"] = name
	}
	if len(version) != 0 {
		query["version"] = version
	}
	if len(productName) != 0 {
		query["product_name"] = productName
	}

	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *BuildColl) Create(build *models.Build) error {
	if build == nil {
		return errors.New("nil Module args")
	}

	build.Name = strings.TrimSpace(build.Name)
	build.UpdateTime = time.Now().Unix()

	//double check
	buildModel, err := c.Find(&BuildFindOption{Name: build.Name, Version: build.Version})
	if err == nil {
		return fmt.Errorf("%s%s", buildModel.ProductName, "项目中有相同的构建名称存在,请检查!")
	}

	_, err = c.Collection.InsertOne(context.TODO(), build)

	return err
}

func (c *BuildColl) Update(build *models.Build) error {
	if build == nil {
		return errors.New("nil Module args")
	}

	query := bson.M{"name": build.Name, "version": build.Version}
	if build.ProductName != "" {
		query["product_name"] = build.ProductName
	}

	updateBuild := bson.M{"$set": build}

	_, err := c.Collection.UpdateOne(context.TODO(), query, updateBuild)
	return err
}

func (c *BuildColl) UpdateTargets(name, productName string, targets []*models.ServiceModuleTarget) error {
	query := bson.M{"name": name}
	if productName != "" {
		query["product_name"] = productName
	}

	change := bson.M{"$set": bson.M{
		"targets": targets,
	}}
	_, err := c.Collection.UpdateMany(context.TODO(), query, change)
	return err
}

func (c *BuildColl) UpdateBuildParam(name, version, productName string, params []*models.Parameter) error {
	query := bson.M{"name": name, "version": version}
	if productName != "" {
		query["product_name"] = productName
	}

	change := bson.M{"$set": bson.M{
		"pre_build.parameters": params,
	}}
	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

// DistinctTargets finds modules distinct service templates
func (c *BuildColl) DistinctTargets(excludeModule []string, productName string) (map[string]bool, error) {
	query := bson.M{}

	if len(excludeModule) != 0 {
		query["name"] = bson.M{"$nin": excludeModule}
	}
	if len(productName) != 0 {
		query["product_name"] = productName
	}

	serviceModuleTargets, err := c.Distinct(context.TODO(), "targets", query)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]bool)
	for _, serviceModuleTarget := range serviceModuleTargets {
		if moduleTarget, ok := serviceModuleTarget.(models.ServiceModuleTarget); ok {
			target := fmt.Sprintf("%s-%s-%s", moduleTarget.ProductName, moduleTarget.ServiceName, moduleTarget.ServiceModule)
			resp[target] = true
		}
	}

	return resp, err
}
