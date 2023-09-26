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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type EnvVersionFindOptions struct {
	Name              string
	EnvName           string
	Namespace         string
	Production        *bool
	IgnoreNotFoundErr bool
}

// ClusterId is a primitive.ObjectID{}.Hex()
type EnvVersionListOptions struct {
	EnvName             string
	Name                string
	Namespace           string
	IsSortByUpdateTime  bool
	IsSortByProductName bool
	InProjects          []string
	InEnvs              []string
	InIDs               []string
	Production          *bool
}

type EnvVersionColl struct {
	*mongo.Collection

	coll string
}

func NewEnvVersionColl() *EnvVersionColl {
	name := models.EnvVersion{}.TableName()
	return &EnvVersionColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *EnvVersionColl) GetCollectionName() string {
	return c.coll
}

func (c *EnvVersionColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "product_name", Value: 1},
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "production", Value: 1},
				bson.E{Key: "service", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *EnvVersionColl) Find(opt *ProductFindOptions) (*models.EnvVersion, error) {
	res := &models.EnvVersion{}
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

	err := c.FindOne(context.TODO(), query).Decode(res)
	if err != nil && mongo.ErrNoDocuments == err && opt.IgnoreNotFoundErr {
		return nil, nil
	}
	return res, err
}

func (c *EnvVersionColl) List(opt *EnvVersionListOptions) ([]*models.EnvVersion, error) {
	var ret []*models.EnvVersion
	query := bson.M{}

	if opt == nil {
		opt = &EnvVersionListOptions{}
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

func (c *EnvVersionColl) Delete(owner, productName string) error {
	query := bson.M{"env_name": owner, "product_name": productName}
	_, err := c.DeleteOne(context.TODO(), query)

	return err
}

// Update  Cannot update owner & product name
func (c *EnvVersionColl) Update(args *models.EnvVersion) error {
	query := bson.M{"env_name": args.EnvName, "product_name": args.ProductName}
	changePayload := bson.M{
		"update_time":      time.Now().Unix(),
		"service":          args.Service,
		"revision":         args.Revision,
		"global_variables": args.GlobalVariables,
		"default_values":   args.DefaultValues,
		"yaml_data":        args.YamlData,
	}
	change := bson.M{"$set": changePayload}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *EnvVersionColl) Create(args *models.EnvVersion) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil EnvVersion")
	}

	now := time.Now().Unix()
	args.CreateTime = now
	args.UpdateTime = now
	_, err := c.InsertOne(context.TODO(), args)

	return err
}
