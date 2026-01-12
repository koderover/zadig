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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ProjectInfo struct {
	Name           string                         `bson:"product_name"`
	Alias          string                         `bson:"project_name"`
	Desc           string                         `bson:"description"`
	UpdatedAt      int64                          `bson:"update_time"`
	UpdatedBy      string                         `bson:"update_by"`
	OnboardStatus  int                            `bson:"onboarding_status"`
	Public         bool                           `bson:"public"`
	ProductFeature *templatemodels.ProductFeature `bson:"product_feature"`
}

type ProductColl struct {
	*mongo.Collection
	mongo.Session
	coll string
}

func NewProductColl() *ProductColl {
	name := template.Product{}.TableName()
	return &ProductColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func NewProductCollWithSess(session mongo.Session) *ProductColl {
	name := template.Product{}.TableName()
	return &ProductColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), Session: session, coll: name}
}

func (c *ProductColl) GetCollectionName() string {
	return c.coll
}

func (c *ProductColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"product_name": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))

	return err
}

func (c *ProductColl) Find(productName string) (*template.Product, error) {
	res := &template.Product{}
	query := bson.M{"product_name": productName}
	err := c.FindOne(mongotool.SessionContext(context.TODO(), c.Session), query).Decode(res)
	return res, err
}

func (c *ProductColl) FindProjectName(project string) (*template.Product, error) {
	resp := &template.Product{}
	query := bson.M{"project_name": project}
	err := c.FindOne(context.TODO(), query).Decode(resp)
	return resp, err
}

func (c *ProductColl) ListNames(inNames []string) ([]string, error) {
	res, err := c.listProjects(inNames, bson.M{
		"product_name": "$product_name",
	})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, r := range res {
		names = append(names, r.Name)
	}

	return names, nil
}

func (c *ProductColl) ListNonPMProject() ([]*ProjectInfo, error) {
	filter := bson.M{}
	filter["product_feature.basic_facility"] = bson.M{"$ne": "cloud_host"}

	pipeline := []bson.M{
		{
			"$match": filter,
		},
		{
			"$project": bson.M{
				"product_name": "$product_name",
				"project_name": "$project_name",
			},
		},
	}

	cursor, err := c.Collection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	var res []*ProjectInfo
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type ProductListByFilterOpt struct {
	Names  []string
	Limit  int64
	Skip   int64
	Filter string
}

// @note pinyin search
func (c *ProductColl) PageListProjectByFilter(opt ProductListByFilterOpt) ([]*ProjectInfo, int, error) {
	findOption := bson.M{}

	if len(opt.Names) > 0 {
		findOption = bson.M{"product_name": bson.M{"$in": opt.Names}}
	} else {
		findOption = bson.M{"product_name": bson.M{"$ne": ""}}
	}

	finalSearchCondition := []bson.M{
		findOption,
	}

	if opt.Filter != "" {
		finalSearchCondition = append(finalSearchCondition, bson.M{
			"$or": bson.A{
				bson.M{"project_name": bson.M{"$regex": opt.Filter, "$options": "i"}},
				bson.M{"product_name": bson.M{"$regex": opt.Filter, "$options": "i"}},
				bson.M{"project_name_pinyin": bson.M{"$regex": opt.Filter, "$options": "i"}},
				bson.M{"project_name_pinyin_first_letter": bson.M{"$regex": opt.Filter, "$options": "i"}},
			},
		})
	}

	filter := bson.M{
		"$and": finalSearchCondition,
	}

	projection := bson.M{
		"product_name":      "$product_name",
		"project_name":      "$project_name",
		"description":       "$description",
		"update_time":       "$update_time",
		"update_by":         "$update_by",
		"onboarding_status": "$onboarding_status",
		"public":            "$public",
		"product_feature":   "$product_feature",
	}
	pipeline := []bson.M{
		{
			"$match": filter,
		},
		{
			"$sort": bson.M{"update_time": -1}, // 按更新时间降序排序
		},
		{
			"$facet": bson.M{
				"data": []bson.M{
					{
						"$project": projection,
					},
					{
						"$skip": opt.Skip,
					},
					{
						"$limit": opt.Limit,
					},
				},
				"total": []bson.M{
					{
						"$count": "total",
					},
				},
			},
		},
	}

	result := make([]struct {
		Data  []*ProjectInfo `bson:"data"`
		Total []struct {
			Total int `bson:"total"`
		} `bson:"total"`
	}, 0)

	cursor, err := c.Collection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(context.TODO(), &result)
	if err != nil {
		return nil, 0, err
	}

	if len(result) == 0 {
		return nil, 0, nil
	}

	var res []*ProjectInfo
	if len(result[0].Data) > 0 {
		res = result[0].Data
	}

	total := 0
	if len(result[0].Total) > 0 {
		total = result[0].Total[0].Total
	}

	return res, total, nil
}

func (c *ProductColl) ListProjectBriefs(inNames []string) ([]*ProjectInfo, error) {
	return c.listProjects(inNames, bson.M{
		"product_name":      "$product_name",
		"project_name":      "$project_name",
		"description":       "$description",
		"update_time":       "$update_time",
		"update_by":         "$update_by",
		"onboarding_status": "$onboarding_status",
		"public":            "$public",
		"product_feature":   "$product_feature",
	})
}

func (c *ProductColl) listProjects(inNames []string, projection bson.M) ([]*ProjectInfo, error) {
	query := bson.M{}
	if len(inNames) > 0 {
		query["product_name"] = bson.M{"$in": inNames}
	} else {
		query["product_name"] = bson.M{"$ne": ""}
	}

	pipeline := []bson.M{
		{
			"$match": query,
		},
		{
			"$project": projection,
		},
	}

	cursor, err := c.Collection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	var res []*ProjectInfo
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ProductColl) Count() (int64, error) {
	query := bson.M{}

	ctx := context.Background()
	count, err := c.Collection.CountDocuments(ctx, query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (c *ProductColl) List() ([]*template.Product, error) {
	var resp []*template.Product

	cursor, err := c.Collection.Find(context.TODO(), bson.M{})
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
	IsOpensource          string
	ContainSharedServices []*template.ServiceInfo
	BasicFacility         string
	DeployType            string
}

// ListWithOption ...
func (c *ProductColl) ListWithOption(opt *ProductListOpt) ([]*template.Product, error) {
	var resp []*template.Product

	query := bson.M{}
	if opt.IsOpensource != "" {
		query["is_opensource"] = stringToBool(opt.IsOpensource)
	}
	if len(opt.ContainSharedServices) > 0 {
		query["shared_services"] = bson.M{"$in": opt.ContainSharedServices}
	}
	if opt.BasicFacility != "" {
		query["product_feature.basic_facility"] = opt.BasicFacility
	}
	if opt.DeployType != "" {
		query["product_feature.deploy_type"] = bson.M{"$in": strings.Split(opt.DeployType, ",")}
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

func (c *ProductColl) UpdateServiceOrchestration(productName string, services [][]string, updateBy string) error {
	query := bson.M{"product_name": productName}
	change := bson.M{"$set": bson.M{
		"services":    services,
		"update_time": time.Now().Unix(),
		"update_by":   updateBy,
	}}

	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	return err
}

func (c *ProductColl) UpdateProductionServiceOrchestration(productName string, services [][]string, updateBy string) error {
	query := bson.M{"product_name": productName}
	change := bson.M{"$set": bson.M{
		"production_services": services,
		"update_time":         time.Now().Unix(),
		"update_by":           updateBy,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductColl) ListAllName() ([]string, error) {
	projects, err := c.List()
	if err != nil {
		return nil, err
	}

	resp := make([]string, 0, len(projects))
	for _, project := range projects {
		resp = append(resp, project.ProductName)
	}
	return resp, nil
}

func (c *ProductColl) UpdateProductFeatureAndServices(productName string, productFeature *template.ProductFeature, services, productionSvcs [][]string, updateBy string) error {
	query := bson.M{"product_name": productName}
	change := bson.M{"$set": bson.M{
		"update_time":                     time.Now().Unix(),
		"update_by":                       updateBy,
		"services":                        services,
		"production_services":             productionSvcs,
		"product_feature.deploy_type":     productFeature.DeployType,
		"product_feature.create_env_type": productFeature.CreateEnvType,
		"product_feature.basic_facility":  productFeature.BasicFacility,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
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
		"project_name":                     strings.TrimSpace(args.ProjectName),
		"project_name_pinyin":              args.ProjectNamePinyin,
		"project_name_pinyin_first_letter": args.ProjectNamePinyinFirstLetter,
		"revision":                         args.Revision,
		"services":                         args.Services,
		"production_services":              args.ProductionServices,
		"update_time":                      time.Now().Unix(),
		"update_by":                        args.UpdateBy,
		"enabled":                          args.Enabled,
		"description":                      args.Description,
		"timeout":                          args.Timeout,
		"release_max_history":              args.ReleaseMaxHistory,
		"auto_deploy":                      args.AutoDeploy,
		"image_searching_rules":            args.ImageSearchingRules,
		"custom_tar_rule":                  args.CustomTarRule,
		"custom_image_rule":                args.CustomImageRule,
		"delivery_version_hook":            args.DeliveryVersionHook,
		"global_variables":                 args.GlobalVariables,
		"production_global_variables":      args.ProductionGlobalVariables,
		"public":                           args.Public,
	}}

	_, err := c.UpdateOne(mongotool.SessionContext(context.TODO(), c.Session), query, change)
	return err
}

type ProductArgs struct {
	ProductName string     `json:"product_name"`
	Services    [][]string `json:"services"`
	UpdateBy    string     `json:"update_by"`
}

// AddService adds a service to services[0] if it is not there.
func (c *ProductColl) AddService(productName, serviceName string) error {

	query := bson.M{"product_name": productName}
	serviceUniqueFilter := bson.M{
		"$elemMatch": bson.M{
			"$elemMatch": bson.M{
				"$eq": serviceName,
			},
		},
	}
	query["services"] = bson.M{"$not": serviceUniqueFilter}
	change := bson.M{"$addToSet": bson.M{
		"services.1": serviceName,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

// AddProductionService adds a service to services[0] if it is not there.
func (c *ProductColl) AddProductionService(productName, serviceName string) error {

	query := bson.M{"product_name": productName}
	serviceUniqueFilter := bson.M{
		"$elemMatch": bson.M{
			"$elemMatch": bson.M{
				"$eq": serviceName,
			},
		},
	}
	query["production_services"] = bson.M{"$not": serviceUniqueFilter}
	change := bson.M{"$addToSet": bson.M{
		"production_services.1": serviceName,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

// UpdateAll updates all projects in a bulk write.
// Currently, only field `shared_services` is supported.
// Note: A bulk operation can have at most 1000 operations, but the client will do it for us.
// see https://stackoverflow.com/questions/24237887/what-is-mongodb-batch-operation-max-size
// Depreated
// This function is only used in migration of old versions, please use Update instead.
func (c *ProductColl) UpdateAll(projects []*template.Product) error {
	if len(projects) == 0 {
		return nil
	}

	var ms []mongo.WriteModel
	for _, p := range projects {
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"product_name", p.ProductName}}).
				SetUpdate(bson.D{{"$set", bson.D{{"shared_services", p.SharedServices}}}}),
		)
	}
	_, err := c.BulkWrite(context.TODO(), ms)

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

func (c *ProductColl) UpdateGlobalVars(productName string, serviceVars []*types.ServiceVariableKV) error {
	query := bson.M{"product_name": productName}
	change := bson.M{"$set": bson.M{
		"global_variables": serviceVars,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *ProductColl) Delete(productName string) error {
	query := bson.M{"product_name": productName}

	_, err := c.DeleteOne(context.TODO(), query)

	return err
}
