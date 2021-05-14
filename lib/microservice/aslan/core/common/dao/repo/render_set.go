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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type RenderSetListOption struct {
	// if Revision == 0 then search max revision of RenderSet
	Revision    int64
	ProductTmpl string
}

// RenderSetFindOption ...
type RenderSetFindOption struct {
	// if Revision == 0 then search max revision of RenderSet
	Revision int64
	Name     string
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

func (c *RenderSetColl) EnsureIndex(ctx context.Context) error {
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

func (c *RenderSetColl) List(opt *RenderSetListOption) ([]*models.RenderSet, error) {
	var pipeResp []*RenderSetPipeResp
	var pipeline []bson.M
	if opt.ProductTmpl != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"product_tmpl": opt.ProductTmpl}})
	}
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"name": "$name",
			},
			"revision": bson.M{"$max": "$revision"},
		},
	})

	cursor, err := c.Aggregate(context.TODO(), pipeline)
	if err != nil {
		return nil, err
	}

	if err := cursor.All(context.TODO(), &pipeResp); err != nil {
		return nil, err
	}

	var resp []*models.RenderSet
	for _, pipe := range pipeResp {
		optRender := &RenderSetFindOption{
			Revision: pipe.Revision,
			Name:     pipe.RenderSet.Name,
		}
		if pipe.Revision == 0 && pipe.RenderSet.Name == "" {
			continue
		}
		rs, err := c.Find(optRender)
		if err != nil {
			return nil, err
		}
		resp = append(resp, rs)
	}
	return resp, nil
}

func (c *RenderSetColl) Find(opt *RenderSetFindOption) (*models.RenderSet, error) {
	if opt == nil {
		return nil, errors.New("RenderSetFindOption cannot be ni")
	}

	query := bson.M{"name": opt.Name}
	opts := options.FindOne()
	if opt.Revision > 0 {
		query["revision"] = opt.Revision
	} else {
		opts.SetSort(bson.D{{"revision", -1}})
	}

	rs := &models.RenderSet{}
	err := c.FindOne(context.TODO(), query, opts).Decode(&rs)
	if err != nil {
		return nil, err
	}

	return rs, err
}

func (c *RenderSetColl) Create(args *models.RenderSet) error {
	if args == nil {
		return errors.New("RenderSet cannot be nil")
	}
	args.UpdateTime = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *RenderSetColl) Update(args *models.RenderSet) error {
	query := bson.M{"name": args.Name, "revision": args.Revision}
	change := bson.M{"$set": bson.M{
		"chart_infos": args.ChartInfos,
		"update_time": time.Now().Unix(),
		"update_by":   args.UpdateBy,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *RenderSetColl) SetDefault(renderTmplName, productTmplName string) error {
	query := bson.M{"name": renderTmplName, "product_tmpl": productTmplName}
	change := bson.M{"$set": bson.M{
		"is_default": true,
	}}

	if _, err := c.UpdateMany(context.TODO(), query, change); err != nil {
		return err
	}

	notEq := bson.M{"name": bson.M{"$ne": renderTmplName}, "product_tmpl": productTmplName}
	falseChange := bson.M{"$set": bson.M{
		"is_default": false,
	}}

	_, err := c.UpdateMany(context.TODO(), notEq, falseChange)
	return err
}

// Delete 根据项目名称删除renderset
func (c *RenderSetColl) Delete(productName string) error {
	query := bson.M{"product_tmpl": productName}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *RenderSetColl) ListAllRenders() ([]*models.RenderSet, error) {
	resp := make([]*models.RenderSet, 0)
	query := bson.M{}

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
