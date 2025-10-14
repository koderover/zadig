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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ServiceFindOption struct {
	ServiceName         string
	Revision            int64
	Type                string
	Source              string
	ProductName         string
	ExcludeStatus       string
	CodehostID          int
	RepoName            string
	BranchName          string
	IgnoreNoDocumentErr bool
}

type ServiceListOption struct {
	ProductName    string
	ServiceName    string
	BuildName      string
	Type           string
	Source         string
	ExcludeProject string
	InServices     []*templatemodels.ServiceInfo
	NotInServices  []*templatemodels.ServiceInfo
}

type ServiceRevision struct {
	ServiceName string
	Revision    int64
}

type SvcRevisionListOption struct {
	ProductName      string
	ServiceRevisions []*ServiceRevision
}

type ServiceAggregateResult struct {
	ServiceID primitive.ObjectID `bson:"service_id"`
}

type ServiceColl struct {
	*mongo.Collection
	mongo.Session
	coll string
}

func NewServiceColl() *ServiceColl {
	name := models.Service{}.TableName()
	return &ServiceColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewServiceCollWithSession(session mongo.Session) *ServiceColl {
	name := models.Service{}.TableName()
	return &ServiceColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
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

// ListServiceAllRevisionsAndStatus will list all revision include deleting status
func (c *ServiceColl) ListServiceAllRevisionsAndStatus(serviceName, productName string) ([]*models.Service, error) {
	resp := make([]*models.Service, 0)
	query := bson.M{}
	query["product_name"] = productName
	query["service_name"] = serviceName

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, mongo.ErrNoDocuments
	}
	return resp, err
}

func (c *ServiceColl) ListMaxRevisionsByProduct(productName string) ([]*models.Service, error) {
	m := bson.M{
		"product_name": productName,
		"status":       bson.M{"$ne": setting.ProductStatusDeleting},
	}

	return c.listMaxRevisions(m, nil)
}

func (c *ServiceColl) ListMaxRevisionsByProductWithFilter(productName string, removeApplicationLinked bool) ([]*models.Service, error) {
	m := bson.M{
		"product_name": productName,
		"status":       bson.M{"$ne": setting.ProductStatusDeleting},
	}

	if removeApplicationLinked {
		// Return services where ApplicationID doesn't have a value (missing, empty, or null)
		m["$or"] = []bson.M{
			{"application_id": bson.M{"$exists": false}}, // field doesn't exist
			{"application_id": ""},                       // field exists but is empty string
			{"application_id": nil},                      // field exists but is null
		}
	}

	return c.listMaxRevisions(m, nil)
}

func (c *ServiceColl) ListMaxRevisionsAllSvcByProduct(productName string) ([]*models.Service, error) {
	m := bson.M{
		"product_name": productName,
	}
	return c.listMaxRevisions(m, nil)
}

func (c *ServiceColl) SearchMaxRevisionsByService(serviceName string) ([]*models.Service, error) {
	pre := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}
	pre["service_name"] = bson.M{"$regex": fmt.Sprintf(".*%s.*", serviceName), "$options": "i"}

	return c.listMaxRevisions(pre, nil)
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

	query := bson.M{}
	query["service_name"] = opt.ServiceName
	if opt.ProductName != "" {
		query["product_name"] = opt.ProductName
	}

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

	err := c.FindOne(mongotool.SessionContext(context.TODO(), c.Session), query, opts).Decode(service)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) && opt.IgnoreNoDocumentErr {
			return nil, nil
		}
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

	_, err := c.DeleteMany(mongotool.SessionContext(context.TODO(), c.Session), query)

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
	_, err := c.InsertOne(mongotool.SessionContext(context.TODO(), c.Session), args)
	return err
}

func (c *ServiceColl) UpdateServiceHealthCheckStatus(args *models.Service) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil ServiceTmplObject")
	}
	args.ProductName = strings.TrimSpace(args.ProductName)
	args.ServiceName = strings.TrimSpace(args.ServiceName)

	query := bson.M{"product_name": args.ProductName, "service_name": args.ServiceName, "revision": args.Revision}

	changeMap := bson.M{
		"create_by":    args.CreateBy,
		"create_time":  time.Now().Unix(),
		"env_configs":  args.EnvConfigs,
		"env_statuses": args.EnvStatuses,
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
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
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ServiceColl) UpdateServiceVariables(args *models.Service) error {
	if args == nil {
		return errors.New("nil ServiceTmplObject")
	}
	args.ProductName = strings.TrimSpace(args.ProductName)
	args.ServiceName = strings.TrimSpace(args.ServiceName)

	query := bson.M{"product_name": args.ProductName, "service_name": args.ServiceName, "revision": args.Revision}
	changeMap := bson.M{
		"variable_yaml":        args.VariableYaml,
		"service_variable_kvs": args.ServiceVariableKVs,
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ServiceColl) UpdateServiceContainers(args *models.Service) error {
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

func (c *ServiceColl) UpdateServiceEnvConfigs(args *models.Service) error {
	if args == nil {
		return errors.New("nil ServiceTmplObject")
	}
	args.ProductName = strings.TrimSpace(args.ProductName)
	args.ServiceName = strings.TrimSpace(args.ServiceName)

	query := bson.M{"product_name": args.ProductName, "service_name": args.ServiceName, "revision": args.Revision}
	changeMap := bson.M{
		"env_configs": args.EnvConfigs,
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ServiceColl) TransferServiceSource(productName, serviceName, source, newSource, username, yaml string) error {
	query := bson.M{"product_name": productName, "source": source, "service_name": serviceName}

	changeMap := bson.M{
		"create_by":     username,
		"visibility":    setting.PrivateVisibility,
		"source":        newSource,
		"yaml":          yaml,
		"env_name":      "",
		"workload_type": "",
	}
	change := bson.M{"$set": changeMap}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

// ListExternalWorkloadsBy list service only for external services , other service type not use  before refactor
func (c *ServiceColl) ListExternalWorkloadsBy(productName, envName string, serviceNames ...string) ([]*models.Service, error) {
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

	if len(serviceNames) > 0 {
		query["service_name"] = bson.M{"$in": serviceNames}
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

func (c *ServiceColl) BatchUpdateExternalServicesStatus(productName, envName, status string, serviceNames []string) error {
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	if len(serviceNames) == 0 {
		return fmt.Errorf("servicenNames is empty")
	}

	query := bson.M{"product_name": productName, "service_name": bson.M{"$in": serviceNames}}
	if envName != "" {
		query["env_name"] = envName
	}

	change := bson.M{"$set": bson.M{
		"status": status,
	}}

	_, err := c.UpdateMany(context.TODO(), query, change)
	return err
}

// UpdateExternalServiceEnvName only used by external services
func (c *ServiceColl) UpdateExternalServiceEnvName(serviceName, productName, envName string) error {
	if serviceName == "" {
		return fmt.Errorf("serviceName is empty")
	}
	if productName == "" {
		return fmt.Errorf("productName is empty")
	}

	query := bson.M{"service_name": serviceName, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"env_name": envName,
	}}

	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
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

	_, err := c.UpdateMany(mongotool.SessionContext(context.TODO(), c.Session), query, change)
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

func (c *ServiceColl) ListServicesWithSRevision(opt *SvcRevisionListOption) ([]*models.Service, error) {
	productMatch := bson.M{}
	productMatch["product_name"] = opt.ProductName

	var serviceMatch bson.A
	for _, sr := range opt.ServiceRevisions {
		serviceMatch = append(serviceMatch, bson.M{
			"service_name": sr.ServiceName,
			"revision":     sr.Revision,
		})
	}

	pipeline := []bson.M{
		{
			"$match": productMatch,
		},
	}
	if len(opt.ServiceRevisions) > 0 {
		pipeline = append(pipeline, bson.M{
			"$match": bson.M{
				"$or": serviceMatch,
			},
		})
	} else {
		return []*models.Service{}, nil
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	res := make([]*models.Service, 0)
	if err := cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}
	return res, err
}

func (c *ServiceColl) ListMaxRevisionsByProject(serviceName, serviceType string) ([]*models.Service, error) {

	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status":       bson.M{"$ne": setting.ProductStatusDeleting},
				"service_name": serviceName,
				"type":         serviceType,
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"product_name": "$product_name",
					"service_name": "$service_name",
				},
				"revision":   bson.M{"$max": "$revision"},
				"service_id": bson.M{"$last": "$_id"},
			},
		},
	}

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	res := make([]*ServiceAggregateResult, 0)
	if err := cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	var ids []primitive.ObjectID
	for _, service := range res {
		ids = append(ids, service.ServiceID)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	var resp []*models.Service
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
		if opt.BuildName != "" {
			postMatch["build_name"] = opt.BuildName
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
	match := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}
	if productName != "" {
		match["product_name"] = productName
	}
	pipeline := []bson.M{
		{
			"$match": match,
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

func (c *ServiceColl) GetChartTemplateReference(templateName string) ([]*models.Service, error) {
	query := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
		"source": setting.SourceFromChartTemplate,
	}

	postMatch := bson.M{
		"create_from.template_name": templateName,
	}

	return c.listMaxRevisions(query, postMatch)
}

func (c *ServiceColl) GetYamlTemplateReference(templateID string) ([]*models.Service, error) {
	query := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
		"source": setting.ServiceSourceTemplate,
	}

	postMatch := bson.M{
		"template_id": templateID,
	}

	return c.listMaxRevisions(query, postMatch)
}

func (c *ServiceColl) GetYamlTemplateLatestReference(templateID string) ([]*models.Service, error) {
	query := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}

	postMatch := bson.M{
		"source":      setting.ServiceSourceTemplate,
		"template_id": templateID,
	}

	return c.listMaxRevisions(query, postMatch)
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
				"service_id":  bson.M{"$last": "$_id"},
				"visibility":  bson.M{"$last": "$visibility"},
				"build_name":  bson.M{"$last": "$build_name"},
				"template_id": bson.M{"$last": "$template_id"},
				"create_from": bson.M{"$last": "$create_from"},
				"source":      bson.M{"$last": "$source"},
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
