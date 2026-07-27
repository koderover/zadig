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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ProductFindOptions struct {
	Name              string
	EnvName           string
	Namespace         string
	Production        *bool
	IgnoreNotFoundErr bool
}

// ClusterId is a primitive.ObjectID{}.Hex()
type ProductListOptions struct {
	EnvName             string
	Name                string
	Namespace           string
	IsPublic            bool
	ClusterID           string
	IsSortByUpdateTime  bool
	IsSortByProductName bool
	ExcludeStatus       []string
	ExcludeSource       string
	Source              string
	InProjects          []string
	InEnvs              []string
	InIDs               []string

	// New Since v1.11.0
	ShareEnvEnable  *bool
	ShareEnvIsBase  *bool
	ShareEnvBaseEnv *string

	// New Since v2.1.0
	IstioGrayscaleEnable  *bool
	IstioGrayscaleIsBase  *bool
	IstioGrayscaleBaseEnv *string

	Production *bool
}

type projectEnvs struct {
	ID          projectID `bson:"_id"`
	ProjectName string    `bson:"project_name"`
	Envs        []string  `bson:"envs"`
}

type projectID struct {
	ProductName string `bson:"product_name"`
}

type ProductColl struct {
	*mongo.Collection
	mongo.Session

	coll string
}

func NewProductColl() *ProductColl {
	name := models.Product{}.TableName()
	return &ProductColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func NewProductCollWithSession(session mongo.Session) *ProductColl {
	name := models.Product{}.TableName()
	return &ProductColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		Session:    session,
		coll:       name,
	}
}

func (c *ProductColl) GetCollectionName() string {
	return c.coll
}

func (c *ProductColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "product_name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "product_name", Value: 1},
				bson.E{Key: "update_time", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

type ProductEnvFindOptions struct {
	Name      string
	Namespace string
}

func (c *ProductColl) Find(opt *ProductFindOptions) (*models.Product, error) {
	res := &models.Product{}
	query := bson.M{}
	if opt.Name != "" {
		query["product_name"] = opt.Name
	}
	if opt.EnvName != "" {
		query["env_name"] = opt.EnvName
	}
	if opt.Namespace != "" {
		query["namespace"] = opt.Namespace
	}
	if opt.Production != nil {
		if *opt.Production {
			query["production"] = true
		} else {
			query["$or"] = []bson.M{{"production": bson.M{"$eq": false}}, {"production": bson.M{"$exists": false}}}
		}
	}

	err := c.FindOne(mongotool.SessionContext(context.TODO(), c.Session), query).Decode(res)
	if err != nil && mongo.ErrNoDocuments == err && opt.IgnoreNotFoundErr {
		return nil, nil
	}
	res.LintServices()
	return res, err
}

func (c *ProductColl) EnvCount() (int64, error) {
	query := bson.M{"status": bson.M{"$ne": setting.ProductStatusDeleting}}

	ctx := context.Background()
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

type Product struct {
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
}

type ListProductOpt struct {
	Products []Product
}

func (c *ProductColl) ListByProducts(opt ListProductOpt) ([]*models.Product, error) {
	var res []*models.Product

	if len(opt.Products) == 0 {
		return nil, nil
	}
	condition := bson.A{}
	for _, pro := range opt.Products {
		condition = append(condition, bson.M{
			"env_name":     pro.Name,
			"product_name": pro.ProjectName,
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

func (c *ProductColl) List(opt *ProductListOptions) ([]*models.Product, error) {
	var ret []*models.Product
	query := bson.M{}

	if opt == nil {
		opt = &ProductListOptions{}
	}
	if opt.EnvName != "" {
		query["env_name"] = opt.EnvName
	} else if len(opt.InEnvs) > 0 {
		query["env_name"] = bson.M{"$in": opt.InEnvs}
	}
	if opt.Name != "" {
		query["product_name"] = opt.Name
	}
	if opt.Namespace != "" {
		query["namespace"] = opt.Namespace
	}
	if opt.IsPublic {
		query["is_public"] = opt.IsPublic
	}
	if opt.ClusterID != "" {
		query["cluster_id"] = opt.ClusterID
	}
	if opt.Source != "" {
		query["source"] = opt.Source
	}
	if opt.ExcludeSource != "" {
		query["source"] = bson.M{"$ne": opt.ExcludeSource}
	}
	if len(opt.ExcludeStatus) > 0 {
		query["status"] = bson.M{"$nin": opt.ExcludeStatus}
	}
	if len(opt.InProjects) > 0 {
		query["product_name"] = bson.M{"$in": opt.InProjects}
	}
	if len(opt.InIDs) > 0 {
		var oids []primitive.ObjectID
		for _, id := range opt.InIDs {
			oid, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				return nil, err
			}
			oids = append(oids, oid)
		}
		query["_id"] = bson.M{"$in": oids}
	}
	if opt.ShareEnvEnable != nil {
		query["share_env.enable"] = *opt.ShareEnvEnable
	}
	if opt.ShareEnvIsBase != nil {
		query["share_env.is_base"] = *opt.ShareEnvIsBase
	}
	if opt.ShareEnvBaseEnv != nil {
		query["share_env.base_env"] = *opt.ShareEnvBaseEnv
	}
	if opt.IstioGrayscaleEnable != nil {
		query["istio_grayscale.enable"] = *opt.IstioGrayscaleEnable
	}
	if opt.IstioGrayscaleIsBase != nil {
		query["istio_grayscale.is_base"] = *opt.IstioGrayscaleIsBase
	}
	if opt.IstioGrayscaleBaseEnv != nil {
		query["istio_grayscale.base_env"] = *opt.IstioGrayscaleBaseEnv
	}
	if opt.Production != nil {
		if *opt.Production {
			query["production"] = true
		} else {
			query["$or"] = []bson.M{{"production": bson.M{"$eq": false}}, {"production": bson.M{"$exists": false}}}
		}
	}

	ctx := context.Background()
	opts := options.Find()
	if opt.IsSortByUpdateTime {
		opts.SetSort(bson.D{{"update_time", -1}})
	}
	if opt.IsSortByProductName {
		opts.SetSort(bson.D{{"product_name", 1}})
	}
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *ProductColl) ListProjectsInNames(names []string) ([]*projectEnvs, error) {
	var res []*projectEnvs
	var pipeline []bson.M
	if len(names) > 0 {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"product_name": bson.M{"$in": names}}})
	}

	pipeline = append(pipeline,
		bson.M{
			"$group": bson.M{
				"_id": bson.M{
					"product_name": "$product_name",
				},
				"project_name": bson.M{"$last": "$product_name"},
				"envs":         bson.M{"$push": "$env_name"},
			},
		},
	)

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (c *ProductColl) UpdateStatusAndError(envName, projectName, status, errorMsg string) error {
	query := bson.M{"env_name": envName, "product_name": projectName}
	change := bson.M{"$set": bson.M{
		"status": status,
		"error":  errorMsg,
	}}
	ctx := context.TODO()

	_, err := c.UpdateOne(ctx, query, change)
	return err
}

func (c *ProductColl) UpdateStatus(owner, productName, status string) error {
	query := bson.M{"env_name": owner, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"status": status,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductColl) UpdateErrors(owner, productName, errorMsg string) error {
	query := bson.M{"env_name": owner, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"error": errorMsg,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) UpdateRegistry(envName, productName, registryId string) error {
	query := bson.M{"env_name": envName, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"registry_id": registryId,
	}}

	ctx := context.TODO()
	if c.Session != nil {
		ctx = mongo.NewSessionContext(ctx, c.Session)
	}
	_, err := c.UpdateOne(ctx, query, change)
	return err
}

func (c *ProductColl) Delete(owner, productName string) error {
	query := bson.M{"env_name": owner, "product_name": productName}
	_, err := c.DeleteOne(context.TODO(), query)

	return err
}

func (c *ProductColl) UpdateGlobalVariable(args *models.Product) error {
	query := bson.M{"env_name": args.EnvName, "product_name": args.ProductName}
	change := bson.M{"$set": bson.M{
		"update_time":      time.Now().Unix(),
		"update_by":        args.UpdateBy,
		"global_variables": args.GlobalVariables,
	}}
	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	return err
}

// Update  Cannot update owner & product name
func (c *ProductColl) Update(args *models.Product) error {
	query := bson.M{"env_name": args.EnvName, "product_name": args.ProductName}
	changePayload := bson.M{
		"update_time":      time.Now().Unix(),
		"update_by":        args.UpdateBy,
		"services":         args.Services,
		"status":           args.Status,
		"revision":         args.Revision,
		"error":            args.Error,
		"share_env":        args.ShareEnv,
		"istio_grayscale":  args.IstioGrayscale,
		"global_variables": args.GlobalVariables,
		"default_values":   args.DefaultValues,
		"yaml_data":        args.YamlData,
	}
	if len(args.Source) > 0 {
		changePayload["source"] = args.Source
	}
	if args.ServiceDeployStrategy != nil {
		changePayload["service_deploy_strategy"] = args.ServiceDeployStrategy
	}
	if args.PreSleepStatus != nil {
		changePayload["pre_sleep_status"] = args.PreSleepStatus
	}
	change := bson.M{"$set": changePayload}
	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	return err
}

func (c *ProductColl) Create(args *models.Product) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil Product")
	}

	now := time.Now().Unix()
	args.CreateTime = now
	args.UpdateTime = now
	_, err := c.InsertOne(mongotool.SessionContext(context.TODO(), c.Session), args)

	return err
}

// @todo UpdateGroup needs to be optimized
// Service info may be override when updating multiple services in same group at the sametime
func (c *ProductColl) UpdateServicesGroup(productName, envName string, groupIndex int, group []*models.ProductService, updateBy string) error {
	serviceGroup := fmt.Sprintf("services.%d", groupIndex)
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
	}
	change := bson.M{
		"update_time": time.Now().Unix(),
		"update_by":   updateBy,
		serviceGroup:  group,
	}

	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, bson.M{"$set": change})

	return err
}

// @todo UpdateServices needs to be optimized
// Service info may be override when updating multiple services at the sametime
func (c *ProductColl) UpdateAllServices(productName, envName string, services [][]*models.ProductService, updateBy string) error {
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
	}
	change := bson.M{
		"update_time": time.Now().Unix(),
		"update_by":   updateBy,
		"services":    services,
	}

	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, bson.M{"$set": change})

	return err
}

// Note: Only use for update a service
// UpdateOneService updates a specific service in a group by its index
func (c *ProductColl) UpdateOneService(productName, envName string, groupIndex, serviceIndex int, service *models.ProductService, updateBy string) error {
	servicePath := fmt.Sprintf("services.%d.%d", groupIndex, serviceIndex)
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
		fmt.Sprintf("%s.service_name", servicePath): service.ServiceName, // ensure service_name equals
		servicePath: bson.M{"$exists": true}, // ensure the service exists
	}
	change := bson.M{"$set": bson.M{
		"update_time": time.Now().Unix(),
		"update_by":   updateBy,
		servicePath:   service,
	}}

	result, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no matching record found to update at %s", servicePath)
	}

	return nil
}

// UpdateServiceValuesSource updates only the external Values source metadata of a service.
// It intentionally leaves yaml_content and override_values unchanged so saving a source
// configuration does not trigger or emulate an import.
func (c *ProductColl) UpdateServiceValuesSource(productName, envName string, production, isHelmChartDeploy bool, identifier string, sourceData *templatemodels.CustomYaml, updateBy string) error {
	if sourceData == nil {
		return errors.New("nil values source data")
	}
	query, serviceFilter := buildServiceValuesSourceQuery(productName, envName, production, isHelmChartDeploy, identifier)
	valuesPath := "services.$[].$[svc].render.override_yaml"
	change := bson.M{"$set": bson.M{
		"update_time":                  time.Now().Unix(),
		"update_by":                    updateBy,
		valuesPath + ".source":         sourceData.Source,
		valuesPath + ".source_id":      sourceData.SourceID,
		valuesPath + ".source_detail":  sourceData.SourceDetail,
		valuesPath + ".auto_sync":      sourceData.AutoSync,
		valuesPath + ".auto_sync_yaml": sourceData.AutoSyncYaml,
	}}
	arrayFilters := options.ArrayFilters{Filters: []interface{}{serviceFilter}}
	updateOptions := options.UpdateOptions{ArrayFilters: &arrayFilters}

	result, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change, &updateOptions)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("no matching service found to update values source: %s", identifier)
	}
	return nil
}

func buildServiceValuesSourceQuery(productName, envName string, production, isHelmChartDeploy bool, identifier string) (bson.M, bson.M) {
	identifierField := "service_name"
	typeFilter := interface{}(bson.M{"$ne": setting.HelmChartDeployType})
	if isHelmChartDeploy {
		identifierField = "release_name"
		typeFilter = setting.HelmChartDeployType
	}
	matchService := bson.M{identifierField: identifier, "type": typeFilter}
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
		"services":     bson.M{"$elemMatch": bson.M{"$elemMatch": matchService}},
	}
	if production {
		query["production"] = true
	} else {
		query["$or"] = []bson.M{{"production": bson.M{"$eq": false}}, {"production": bson.M{"$exists": false}}}
	}
	serviceFilter := bson.M{
		fmt.Sprintf("svc.%s", identifierField): identifier,
		"svc.type":                             typeFilter,
	}
	return query, serviceFilter
}

// Note: Only use for add a service
// AddOneService adds a specific service in a group by its index if it does not already exist
func (c *ProductColl) AddOneService(productName, envName string, groupIndex, serviceIndex int, service *models.ProductService, updateBy string) error {
	servicePath := fmt.Sprintf("services.%d.%d", groupIndex, serviceIndex)
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
		servicePath:    bson.M{"$exists": false}, // ensure the service does not exist
	}
	change := bson.M{"$set": bson.M{
		"update_time": time.Now().Unix(),
		"update_by":   updateBy,
		servicePath:   service,
	}}

	result, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("service already exists at %s", servicePath)
	}

	return nil
}

func (c *ProductColl) UpdateDeployStrategyAndGlobalVariable(envName, productName string, deployStrategy map[string]setting.ServiceDeployStrategy, globalVariables []*types.GlobalVariableKV, updateBy string) error {
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
	}
	change := bson.M{
		"update_time":             time.Now().Unix(),
		"update_by":               updateBy,
		"global_variables":        globalVariables,
		"service_deploy_strategy": deployStrategy,
	}

	_, err := c.UpdateOne(context.TODO(), query, bson.M{"$set": change})

	return err
}

func (c *ProductColl) UpdateDeployStrategy(envName, productName string, deployStrategy map[string]setting.ServiceDeployStrategy, updateBy string) error {
	query := bson.M{
		"env_name":     envName,
		"product_name": productName,
	}
	change := bson.M{
		"update_time":             time.Now().Unix(),
		"update_by":               updateBy,
		"service_deploy_strategy": deployStrategy,
	}

	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, bson.M{"$set": change})

	return err
}

func (c *ProductColl) UpdateProductVariables(product *models.Product) error {
	query := bson.M{"env_name": product.EnvName, "product_name": product.ProductName}

	change := bson.M{"$set": bson.M{
		"default_values":   product.DefaultValues,
		"yaml_data":        product.YamlData,
		"global_variables": product.GlobalVariables,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductColl) UpdateProductRecycleDay(envName, productName string, recycleDay int) error {
	query := bson.M{"env_name": envName, "product_name": productName}

	change := bson.M{"$set": bson.M{
		"recycle_day": recycleDay,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) UpdateProductAlias(envName, productName, alias string) error {
	query := bson.M{"env_name": envName, "product_name": productName}

	change := bson.M{"$set": bson.M{
		"alias": alias,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) UpdateIsPublic(envName, productName string, isPublic bool, updateBy string) error {
	query := bson.M{"env_name": envName, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"update_time": time.Now().Unix(),
		"update_by":   updateBy,
		"is_public":   isPublic,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) UpdateIstioGrayscale(envName, productName string, istioGrayscale models.IstioGrayscale, updateBy string) error {
	query := bson.M{"env_name": envName, "product_name": productName}
	change := bson.M{"$set": bson.M{
		"update_time":     time.Now().Unix(),
		"update_by":       updateBy,
		"istio_grayscale": istioGrayscale,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *ProductColl) Count(productName string) (int, error) {
	num, err := c.CountDocuments(context.TODO(), bson.M{"product_name": productName, "status": bson.M{"$ne": setting.ProductStatusDeleting}})

	return int(num), err
}

// UpdateAll updates all envs in a bulk write.
// Currently only field `services` is supported.
// Note: A bulk operation can have at most 1000 operations, but the client will do it for us.
// see https://stackoverflow.com/questions/24237887/what-is-mongodb-batch-operation-max-size
func (c *ProductColl) UpdateAll(envs []*models.Product) error {
	if len(envs) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, env := range envs {
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", env.ID}}).
				SetUpdate(bson.D{{"$set", bson.D{{"services", env.Services}}}}),
		)
	}
	_, err := c.BulkWrite(context.TODO(), ms)

	return err
}

type nsObject struct {
	ID        primitive.ObjectID `bson:"_id"`
	Namespace string             `bson:"namespace"`
}

func (c *ProductColl) ListExistedNamespace(clusterID string) ([]string, error) {
	nsList := make([]*nsObject, 0)
	resp := sets.NewString()
	selector := bson.D{
		{"namespace", 1},
	}
	query := bson.M{"is_existed": true}
	if clusterID != "" {
		query["cluster_id"] = clusterID
	}
	opt := options.Find()
	opt.SetProjection(selector)
	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &nsList)
	if err != nil {
		return nil, err
	}
	for _, obj := range nsList {
		resp.Insert(obj.Namespace)
	}
	return resp.List(), nil
}

func (c *ProductColl) ListProductionNamespace(clusterID string) ([]string, error) {
	nsList := make([]*nsObject, 0)
	resp := sets.NewString()
	selector := bson.D{
		{"namespace", 1},
	}
	query := bson.M{"production": true, "cluster_id": clusterID}
	opt := options.Find()
	opt.SetProjection(selector)
	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &nsList)
	if err != nil {
		return nil, err
	}
	for _, obj := range nsList {
		resp.Insert(obj.Namespace)
	}
	return resp.List(), nil
}

func (c *ProductColl) ListNamespace(clusterID string) ([]string, error) {
	nsList := make([]*nsObject, 0)
	resp := sets.NewString()
	selector := bson.D{
		{"namespace", 1},
	}
	query := bson.M{"cluster_id": clusterID}
	opt := options.Find()
	opt.SetProjection(selector)
	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &nsList)
	if err != nil {
		return nil, err
	}
	for _, obj := range nsList {
		resp.Insert(obj.Namespace)
	}
	return resp.List(), nil
}

func (c *ProductColl) ListEnvByNamespace(clusterID, namespace string) ([]*models.Product, error) {
	var resp []*models.Product
	query := bson.M{"namespace": namespace, "cluster_id": clusterID}
	cursor, err := c.Collection.Find(context.Background(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.Background(), &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ProductColl) UpdateConfigs(envName, productName string, analysisConfig *models.AnalysisConfig, notificationConfigs []*models.NotificationConfig, updateBy string) error {
	query := bson.M{"env_name": envName, "product_name": productName}

	change := bson.M{"$set": bson.M{
		"analysis_config":      analysisConfig,
		"notification_configs": notificationConfigs,
		"update_time":          time.Now().Unix(),
		"update_by":            updateBy,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}
