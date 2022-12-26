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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type RenderSetListOption struct {
	// if Revision == 0 then search max revision of RenderSet
	ProductTmpl   string
	Revisions     []int64
	RendersetName string
	FindOpts      []RenderSetFindOption
}

// RenderSetFindOption ...
type RenderSetFindOption struct {
	// if Revision == 0 then search max revision of RenderSet
	ProductTmpl       string
	EnvName           string
	IsDefault         bool
	Revision          int64
	Name              string
	YamlVariableSetID string
}

type RenderSetPipeResp struct {
	RenderSet struct {
		Name        string `bson:"name"                     json:"name"`
		ProductTmpl string `bson:"product_tmpl"             json:"product_tmpl"`
	} `bson:"_id"      json:"render_set"`
	Revision int64 `bson:"revision"     json:"revision"`
}

type RenderSetColl struct {
	*mongo.Collection

	coll string
}

func NewRenderSetColl() *RenderSetColl {
	name := models.RenderSet{}.TableName()
	return &RenderSetColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *RenderSetColl) GetCollectionName() string {
	return c.coll
}

func (c *RenderSetColl) FindRenderSet(opt *RenderSetFindOption) (*models.RenderSet, bool, error) {
	res, err := c.Find(opt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false, nil
		}
		return nil, false, err
	}

	return res, true, nil
}

func (c *RenderSetColl) Find(opt *RenderSetFindOption) (*models.RenderSet, error) {
	if opt == nil {
		return nil, errors.New("RenderSetFindOption cannot be nil")
	}

	query := bson.M{"name": opt.Name}
	opts := options.FindOne()
	if opt.Revision > 0 {
		// revisionName + revision are enough to locate the target record
		// there is no need to set other query condition
		// Note. the query logic has been rolled back to 1.12.0
		query["revision"] = opt.Revision
	} else {
		opts.SetSort(bson.D{{"revision", -1}})

		if len(opt.EnvName) > 0 {
			query["env_name"] = opt.EnvName
		}

		if len(opt.ProductTmpl) > 0 {
			query["product_tmpl"] = opt.ProductTmpl
		}

		if opt.IsDefault {
			query["is_default"] = opt.IsDefault
		}
	}

	rs := &models.RenderSet{}
	err := c.FindOne(context.TODO(), query, opts).Decode(&rs)
	if err != nil {
		return nil, err
	}

	return rs, err
}

func (c *RenderSetColl) Update(args *models.RenderSet) error {
	query := bson.M{"name": args.Name, "revision": args.Revision}
	change := bson.M{"$set": bson.M{
		"chart_infos": args.ChartInfos,
		"update_time": time.Now().Unix(),
		"update_by":   args.UpdateBy,
		//"kvs":            args.KVs,
		"default_values": args.DefaultValues,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *RenderSetColl) UpdateDefaultValues(renderSet *models.RenderSet) error {
	query := bson.M{"name": renderSet.Name, "revision": renderSet.Revision}
	change := bson.M{"$set": bson.M{
		"update_time":    time.Now().Unix(),
		"update_by":      "ua",
		"default_values": renderSet.DefaultValues,
	}}
	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
