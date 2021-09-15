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
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/setting"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type ServiceFindOption struct {
	ServiceName   string
	Revision      int64
	Type          string
	Source        string
	ProductName   string
	ExcludeStatus string
	CodehostID    int
	RepoName      string
	BranchName    string
}

type ServiceListOption struct {
	ProductName    string
	ServiceName    string
	BuildName      string
	Type           string
	Source         string
	Visibility     string
	ExcludeProject string
	InServices     []*templatemodels.ServiceInfo
	NotInServices  []*templatemodels.ServiceInfo
}

type ServiceColl struct {
	*mongo.Collection

	coll string
}

func NewServiceColl() *ServiceColl {
	name := models.Service{}.TableName()
	return &ServiceColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ServiceColl) GetCollectionName() string {
	return c.coll
}

func (c *ServiceColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "product_name", Value: 1},
				bson.E{Key: "revision", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "product_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.M{"revision": 1},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "type", Value: 1},
				bson.E{Key: "revision", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "product_name", Value: 1},
				bson.E{Key: "service_name", Value: 1},
				bson.E{Key: "type", Value: 1},
				bson.E{Key: "revision", Value: 1},
				bson.E{Key: "status", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	// 仅用于升级 release v1.3.1, 将在下一版本移除
	_, _ = c.Indexes().DropOne(ctx, "service_name_1_type_1_revision_1")

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

type grouped struct {
	ID        serviceID          `bson:"_id"`
	ServiceID primitive.ObjectID `bson:"service_id"`
}

type serviceID struct {
	ServiceName string `bson:"service_name"`
	ProductName string `bson:"product_name"`
}

func (c *ServiceColl) ListMaxRevisionsForServices(services []*templatemodels.ServiceInfo, serviceType string) ([]*models.Service, error) {
	var srs []bson.D
	for _, s := range services {
		// be care for the order
		srs = append(srs, bson.D{
			{"product_name", s.Owner},
			{"service_name", s.Name},
		})
	}

	pre := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}
	if serviceType != "" {
		pre["type"] = serviceType
	}
	post := bson.M{
		"_id": bson.M{"$in": srs},
	}

	return c.listMaxRevisions(pre, post)
}

// TODO refactor mouuii
// ListExternalServicesBy list service only for external services  ,other service type not use  before refactor
func (c *ServiceColl) ListExternalWorkloadsBy(productName, envName string) ([]*models.Service, error) {
	services := make([]*models.Service, 0)
	query := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}
	if productName != "" {
		query["product_name"] = productName
	}
	if envName != "" {
		query["env_name"] = envName
	}
	ctx := context.Background()
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &services)
	if err != nil {
		return nil, err
	}
	return services, nil
}

func (c *ServiceColl) ListMaxRevisionsByProduct(productName string) ([]*models.Service, error) {
	m := bson.M{
		"product_name": productName,
		"status":       bson.M{"$ne": setting.ProductStatusDeleting},
	}

	return c.listMaxRevisions(m, nil)
}

// Find 根据service_name和type查询特定版本的配置模板
// 如果 Revision == 0 查询最大版本的配置模板
func (c *ServiceColl) Find(opt *ServiceFindOption) (*models.Service, error) {
	if opt == nil {
		return nil, errors.New("nil ConfigFindOption")
	}
	if opt.ServiceName == "" {
		return nil, fmt.Errorf("ServiceName is empty")
	}
	if opt.ProductName == "" {
		return nil, fmt.Errorf("ProductName is empty")
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
		return nil, err
	}

	return service, nil
}

func (c *ServiceColl) Delete(serviceName, serviceType, productName, status string, revision int64) error {
	query := bson.M{}
	if serviceName != "" {
		query["service_name"] = serviceName
	}
	if serviceType != "" {
		query["type"] = serviceType
	}
	if productName != "" {
		query["product_name"] = productName
	}
	if revision != 0 {
		query["revision"] = revision
	}
	if status != "" {
		query["status"] = status
	}

	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)

	return err
}

func (c *ServiceColl) Create(args *models.Service) error {
	//avoid panic issue
	if args == nil {
		return errors.New("nil Service")
	}

	args.ServiceName = strings.TrimSpace(args.ServiceName)
	args.CreateTime = time.Now().Unix()
	if args.Yaml != "" {
		// 原来的hash256函数，只有这里用到，直接整合进逻辑
		h := sha256.New()
		h.Write([]byte(args.Yaml + "\n"))
		args.Hash = fmt.Sprintf("%x", h.Sum(nil))
	}
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *ServiceColl) Update(args *models.Service) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil ServiceTmplObject")
	}
	args.ProductName = strings.TrimSpace(args.ProductName)
	args.ServiceName = strings.TrimSpace(args.ServiceName)

	query := bson.M{"product_name": args.ProductName, "service_name": args.ServiceName, "revision": args.Revision}

	changeMap := bson.M{
		"create_by":   args.CreateBy,
		"create_time": time.Now().Unix(),
	}
	//非容器部署服务在探活过程中会更新health_check相关的参数，其他的情况只会更新服务共享属性
	if len(args.EnvStatuses) > 0 {
		changeMap["env_statuses"] = args.EnvStatuses
	} else {
		changeMap["visibility"] = args.Visibility
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

// UpdateExternalServicesStatus only used by external services
func (c *ServiceColl) UpdateExternalServicesStatus(serviceName, productName, status, envName string) error {
	if serviceName == "" && envName == "" {
		return fmt.Errorf("serviceName and envName can't  be both empty")
	}
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	query := bson.M{"product_name": productName}
	if serviceName != "" {
		query["service_name"] = serviceName
	}
	if envName != "" {
		query["env_name"] = envName
	}
	change := bson.M{"$set": bson.M{
		"status": status,
	}}

	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

func (c *ServiceColl) UpdateStatus(serviceName, productName, status string) error {
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

// ListAllRevisions 列出历史所有的service
// TODO 返回k8s所有service
func (c *ServiceColl) ListAllRevisions() ([]*models.Service, error) {
	resp := make([]*models.Service, 0)
	query := bson.M{}
	query["status"] = bson.M{"$ne": setting.ProductStatusDeleting}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *ServiceColl) ListMaxRevisions(opt *ServiceListOption) ([]*models.Service, error) {
	preMatch := bson.M{"status": bson.M{"$ne": setting.ProductStatusDeleting}}
	postMatch := bson.M{}

	if opt != nil {
		if opt.ProductName != "" {
			preMatch["product_name"] = opt.ProductName
		}
		if opt.ServiceName != "" {
			preMatch["service_name"] = opt.ServiceName
		}
		if opt.BuildName != "" {
			preMatch["build_name"] = opt.BuildName
		}
		if opt.Source != "" {
			preMatch["source"] = opt.Source
		}
		if opt.Type != "" {
			preMatch["type"] = opt.Type
		}
		if opt.ExcludeProject != "" {
			preMatch["product_name"] = bson.M{"$ne": opt.ExcludeProject}
		}

		// post options (anything that changes over revision should be added in post options)
		if opt.Visibility != "" {
			postMatch["visibility"] = opt.Visibility
		}
		if len(opt.NotInServices) > 0 {
			var srs []bson.D
			for _, s := range opt.NotInServices {
				// be care for the order
				srs = append(srs, bson.D{
					{"product_name", s.Owner},
					{"service_name", s.Name},
				})
			}
			postMatch["_id"] = bson.M{"$nin": srs}
		}
		if len(opt.InServices) > 0 {
			var srs []bson.D
			for _, s := range opt.InServices {
				// be care for the order
				srs = append(srs, bson.D{
					{"product_name", s.Owner},
					{"service_name", s.Name},
				})
			}
			postMatch["_id"] = bson.M{"$in": srs}
		}
	}

	return c.listMaxRevisions(preMatch, postMatch)
}

func (c *ServiceColl) Count(productName string) (int, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"product_name": productName,
				"status":       bson.M{"$ne": setting.ProductStatusDeleting},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"product_name": "$product_name",
					"service_name": "$service_name",
				},
			},
		},
		{
			"$count": "count",
		},
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return 0, err
	}

	var cs []struct {
		Count int `bson:"count"`
	}
	if err := cursor.All(context.TODO(), &cs); err != nil || len(cs) == 0 {
		return 0, err
	}

	return cs[0].Count, nil
}

func (c *ServiceColl) listMaxRevisions(preMatch, postMatch bson.M) ([]*models.Service, error) {
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
				"visibility": bson.M{"$last": "$visibility"},
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
