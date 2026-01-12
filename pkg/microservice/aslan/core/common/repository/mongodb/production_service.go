/*
Copyright 2023 The KodeRover Authors.

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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ProductionServiceColl struct {
	*mongo.Collection
	mongo.Session
	coll string
}

func NewProductionServiceColl() *ProductionServiceColl {
	name := "production_template_service"
	return &ProductionServiceColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func NewProductionServiceCollWithSession(session mongo.Session) *ProductionServiceColl {
	name := "production_template_service"
	return &ProductionServiceColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *ProductionServiceColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "product_name", Value: 1},
			bson.E{Key: "service_name", Value: 1},
			bson.E{Key: "revision", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *ProductionServiceColl) GetCollectionName() string {
	return c.coll
}

func (c *ProductionServiceColl) Create(args *models.Service) error {
	args.CreateTime = time.Now().Unix()
	_, err := c.InsertOne(context.Background(), args)
	return err
}

func (c *ProductionServiceColl) Delete(serviceName, serviceType, productName, status string, revision int64) error {
	query := bson.M{}
	query["service_name"] = serviceName
	query["product_name"] = productName
	query["revision"] = revision
	if serviceType != "" {
		query["type"] = serviceType
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

func (c *ProductionServiceColl) DeleteByProject(productName string) error {
	query := bson.M{}
	query["product_name"] = productName

	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

type ProductionServiceDeleteOption struct {
	ServiceName string
	ProductName string
	Type        string
	Status      string
	Revision    int64
}

func (c *ProductionServiceColl) DeleteByOptions(opts ProductionServiceDeleteOption) error {
	query := bson.M{}
	if opts.ServiceName != "" {
		query["service_name"] = opts.ServiceName
	}
	if opts.Type != "" {
		query["type"] = opts.Type
	}
	if opts.ProductName != "" {
		query["product_name"] = opts.ProductName
	}
	if opts.Revision != 0 {
		query["revision"] = opts.Revision
	}
	if opts.Status != "" {
		query["status"] = opts.Status
	}

	if len(query) == 0 {
		return nil
	}

	_, err := c.DeleteMany(context.TODO(), query)

	return err
}

type ProductionServiceOption struct {
	ProductName   string
	ExcludeStatus string
}

func (c *ProductionServiceColl) Find(opt *ServiceFindOption) (*models.Service, error) {
	if opt == nil {
		return nil, errors.New("nil ConfigFindOption")
	}
	if opt.ServiceName == "" {
		return nil, errors.New("ServiceName is empty")
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
		if err == mongo.ErrNoDocuments && opt.IgnoreNoDocumentErr {
			return nil, nil
		}
		return nil, err
	}

	return service, nil
}

func (c *ProductionServiceColl) ListMaxRevisions(opt *ServiceListOption) ([]*models.Service, error) {
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

func (c *ProductionServiceColl) ListMaxRevisionsByProject(productName, serviceType string) ([]*models.Service, error) {
	pre := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}
	pre["product_name"] = productName
	if serviceType != "" {
		pre["type"] = serviceType
	}
	return c.listMaxRevisions(pre, nil)
}

func (c *ProductionServiceColl) SearchMaxRevisionsByService(serviceName string) ([]*models.Service, error) {
	pre := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}
	pre["service_name"] = bson.M{"$regex": fmt.Sprintf(".*%s.*", serviceName), "$options": "i"}
	return c.listMaxRevisions(pre, nil)
}

func (c *ProductionServiceColl) Update(args *models.Service) error {
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
	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	return err
}

// ListServiceAllRevisionsAndStatus will list all revision include deleting status
func (c *ProductionServiceColl) ListServiceAllRevisionsAndStatus(serviceName, productName string) ([]*models.Service, error) {
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

func (c *ProductionServiceColl) ListServiceAllRevisions(productName, serviceName string) ([]*models.Service, error) {
	resp := make([]*models.Service, 0)
	query := bson.M{}
	query["product_name"] = productName
	query["service_name"] = serviceName
	query["status"] = bson.M{"$ne": setting.ProductStatusDeleting}

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

func (c *ProductionServiceColl) UpdateServiceVariables(args *models.Service) error {
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

func (c *ProductionServiceColl) UpdateServiceContainers(args *models.Service) error {
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

func (c *ProductionServiceColl) UpdateStatus(serviceName, productName, status string) error {
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

func (c *ProductionServiceColl) ListMaxRevisionsByProduct(productName string) ([]*models.Service, error) {
	m := bson.M{
		"product_name": productName,
		"status":       bson.M{"$ne": setting.ProductStatusDeleting},
	}

	return c.listMaxRevisions(m, nil)
}

func (c *ProductionServiceColl) ListMaxRevisionsByProductWithFilter(productName string, removeApplicationLinked bool) ([]*models.Service, error) {
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

func (c *ProductionServiceColl) ListServicesWithSRevision(opt *SvcRevisionListOption) ([]*models.Service, error) {
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

func (c *ProductionServiceColl) TransferServiceSource(productName, serviceName, source, newSource, username, yaml string) error {
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

func (c *ProductionServiceColl) listMaxRevisions(preMatch, postMatch bson.M) ([]*models.Service, error) {
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

func (c *ProductionServiceColl) GetYamlTemplateReference(templateID string) ([]*models.Service, error) {
	query := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
		"source": setting.ServiceSourceTemplate,
	}

	postMatch := bson.M{
		"template_id": templateID,
	}

	return c.listMaxRevisions(query, postMatch)
}

func (c *ProductionServiceColl) GetYamlTemplateLatestReference(templateID string) ([]*models.Service, error) {
	query := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
	}

	postMatch := bson.M{
		"source":      setting.ServiceSourceTemplate,
		"template_id": templateID,
	}

	return c.listMaxRevisions(query, postMatch)
}

func (c *ProductionServiceColl) GetChartTemplateReference(templateName string) ([]*models.Service, error) {
	query := bson.M{
		"status": bson.M{"$ne": setting.ProductStatusDeleting},
		"source": setting.SourceFromChartTemplate,
	}

	postMatch := bson.M{
		"create_from.template_name": templateName,
	}

	return c.listMaxRevisions(query, postMatch)
}
