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
	"github.com/koderover/zadig/pkg/setting"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

// ServiceFindOption ...
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
	ProductName   string
	ServiceName   string
	IsSort        bool
	ExcludeStatus string
	BuildName     string
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
				bson.E{Key: "type", Value: 1},
				bson.E{Key: "revision", Value: 1},
			},
			Options: options.Index().SetUnique(true),
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

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

type grouped struct {
	ID        serviceRevision    `bson:"_id"`
	ServiceID primitive.ObjectID `bson:"service_id"`
}

type serviceRevision struct {
	ServiceName string `bson:"service_name"`
}

func (c *ServiceColl) ListMaxRevisionsForServices(serviceNames []string, serviceType string) ([]*models.Service, error) {
	match := bson.M{
		"service_name": bson.M{"$in": serviceNames},
		"status":       bson.M{"$ne": setting.ProductStatusDeleting},
	}
	if serviceType != "" {
		match["type"] = serviceType
	}

	return c.listMaxRevisions(match)
}

func (c *ServiceColl) ListMaxRevisionsByProduct(productName string) ([]*models.Service, error) {
	return c.listMaxRevisions(bson.M{
		"product_name": productName,
		"status":       bson.M{"$ne": setting.ProductStatusDeleting},
	})
}

// Find 根据service_name和type查询特定版本的配置模板
// 如果 Revision == 0 查询最大版本的配置模板
func (c *ServiceColl) Find(opt *ServiceFindOption) (*models.Service, error) {
	if opt == nil {
		return nil, errors.New("nil ConfigFindOption")
	}
	if opt.ServiceName == "" {
		return nil, fmt.Errorf("service_name is empty")
	}

	query := bson.M{}
	query["service_name"] = opt.ServiceName
	service := new(models.Service)

	if opt.Type != "" {
		query["type"] = opt.Type
	}
	if opt.ProductName != "" {
		query["product_name"] = opt.ProductName
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

// DistinctServices returns distinct service templates with service_name + type
// Could be filtered with team, "all" means all teams
func (c *ServiceColl) DistinctServices(opt *ServiceListOption) ([]models.ServiceTmplRevision, error) {
	var resp []models.ServiceTmplRevision
	var results []models.ServiceTmplPipeResp

	var pipeline []bson.M
	if opt.ExcludeStatus != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"status": bson.M{"$ne": opt.ExcludeStatus}}})
	}
	if opt.ProductName != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"product_name": opt.ProductName}})
	}
	if opt.ServiceName != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"service_name": opt.ServiceName}})
	}
	if opt.BuildName != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"build_name": opt.BuildName}})
	}
	if opt.IsSort {
		pipeline = append(pipeline, bson.M{"$sort": bson.M{"create_time": -1}})
	} else {
		pipeline = append(pipeline, bson.M{"$sort": bson.M{"service_name": 1}})
	}
	pipeline = append(pipeline,
		bson.M{"$group": bson.M{"_id": bson.M{"service_name": "$service_name", "type": "$type", "product_name": "$product_name"},
			"revision":           bson.M{"$max": "$revision"},
			"source":             bson.M{"$first": "$source"},
			"codehost_id":        bson.M{"$first": "$codehost_id"},
			"is_dir":             bson.M{"$first": "$is_dir"},
			"load_path":          bson.M{"$first": "$load_path"},
			"repo_name":          bson.M{"$first": "$repo_name"},
			"repo_owner":         bson.M{"$first": "$repo_owner"},
			"gerrit_remote_name": bson.M{"$first": "$gerrit_remote_name"},
			"branch_name":        bson.M{"$first": "$branch_name"}}})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &results)
	if err != nil {
		return nil, err
	}

	for _, result := range results {
		resp = append(resp, models.ServiceTmplRevision{
			ServiceName:      result.ID.ServiceName,
			Source:           result.Source,
			Type:             result.ID.Type,
			Revision:         result.Revision,
			ProductName:      result.ID.ProductName,
			LoadPath:         result.LoadPath,
			LoadFromDir:      result.LoadFromDir,
			CodehostID:       result.CodehostID,
			RepoName:         result.RepoName,
			RepoOwner:        result.RepoOwner,
			BranchName:       result.BranchName,
			GerritRemoteName: result.ID.GerritRemoteName,
		})
	}

	return resp, nil
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

func (c *ServiceColl) List(opt *ServiceFindOption) ([]*models.Service, error) {
	query := bson.M{}
	if opt.ProductName != "" {
		query["product_name"] = opt.ProductName
	}
	if opt.ServiceName != "" {
		query["service_name"] = opt.ServiceName
	}
	if opt.Type != "" {
		query["type"] = opt.Type
	}
	if opt.Revision > 0 {
		query["revision"] = opt.Revision
	}
	if opt.Source != "" {
		query["source"] = opt.Source
	}
	if opt.CodehostID > 0 {
		query["codehost_id"] = opt.CodehostID
	}
	if opt.RepoName != "" {
		query["repo_name"] = opt.RepoName
	}
	if opt.BranchName != "" {
		query["branch_name"] = opt.BranchName
	}
	if opt.ExcludeStatus != "" {
		query["status"] = bson.M{"$ne": opt.ExcludeStatus}
	}

	var resp []*models.Service
	var results []models.ServiceTmplPipeResp
	var pipeline []bson.M
	pipeline = append(pipeline, bson.M{"$match": query})
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"revision": -1}})
	pipeline = append(pipeline,
		bson.M{"$group": bson.M{"_id": bson.M{"service_name": "$service_name", "type": "$type", "product_name": "$product_name"},
			"revision":    bson.M{"$first": "$revision"},
			"containers":  bson.M{"$first": "$containers"},
			"source":      bson.M{"$first": "$source"},
			"visibility":  bson.M{"$first": "$visibility"},
			"src_path":    bson.M{"$first": "$src_path"},
			"codehost_id": bson.M{"$first": "$codehost_id"},
			"repo_owner":  bson.M{"$first": "$repo_owner"},
			"repo_name":   bson.M{"$first": "$repo_name"},
			"branch_name": bson.M{"$first": "$branch_name"},
			"load_path":   bson.M{"$first": "$load_path"},
			"is_dir":      bson.M{"$first": "$is_dir"},
			"repo_uuid":   bson.M{"$first": "$repo_uuid"},
		}})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	if err := cursor.All(context.TODO(), &results); err != nil {
		return nil, err
	}

	for _, result := range results {
		resp = append(resp, &models.Service{
			ServiceName: result.ID.ServiceName,
			Type:        result.ID.Type,
			ProductName: result.ID.ProductName,
			Source:      result.Source,
			Revision:    result.Revision,
			Containers:  result.Containers,
			Visibility:  result.Visibility,
			SrcPath:     result.SrcPath,
			CodehostID:  result.CodehostID,
			RepoOwner:   result.RepoOwner,
			RepoName:    result.RepoName,
			BranchName:  result.BranchName,
			LoadPath:    result.LoadPath,
			LoadFromDir: result.LoadFromDir,
			RepoUUID:    result.RepoUUID,
		})
	}

	return resp, nil
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

	query := bson.M{"product_name": args.ProductName, "service_name": args.ServiceName, "type": args.Type, "revision": args.Revision}

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

func (c *ServiceColl) UpdateStatus(serviceName, serviceType, productName, status string) error {
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

func (c *ServiceColl) listMaxRevisions(match bson.M) ([]*models.Service, error) {
	var pipeResp []*grouped
	pipeline := []bson.M{
		{
			"$match": match,
		},
		{
			"$sort": bson.M{"revision": 1},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"service_name": "$service_name",
				},
				"service_id": bson.M{"$last": "$_id"},
			},
		},
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
