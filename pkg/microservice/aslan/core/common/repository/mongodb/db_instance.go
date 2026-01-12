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
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DBInstanceColl struct {
	*mongo.Collection
}

type DBInstanceCollFindOption struct {
	Id   string
	Name string
}

func NewDBInstanceColl() *DBInstanceColl {
	coll := &DBInstanceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(models.DBInstance{}.TableName())}
	return coll
}

func (c *DBInstanceColl) GetCollectionName() string {
	return models.DBInstance{}.TableName()
}

func (c *DBInstanceColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *DBInstanceColl) List() ([]*models.DBInstance, error) {
	resp := make([]*models.DBInstance, 0)
	query := bson.M{}

	ctx := context.Background()
	opts := options.Find().SetSort(bson.D{{"created_at", -1}})

	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *DBInstanceColl) Find(opt *DBInstanceCollFindOption) (*models.DBInstance, error) {
	query := bson.M{}
	if len(opt.Id) > 0 {
		oid, err := primitive.ObjectIDFromHex(opt.Id)
		if err != nil {
			return nil, err
		}
		query["_id"] = oid
	}
	if opt.Name != "" {
		query["name"] = opt.Name
	}

	resp := &models.DBInstance{}
	err := c.FindOne(context.Background(), query).Decode(resp)
	return resp, err
}

func (c *DBInstanceColl) Create(args *models.DBInstance) error {
	if args == nil {
		return errors.New("nil db instance args")
	}

	args.CreatedAt = time.Now().Unix()
	args.UpdatedAt = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *DBInstanceColl) Update(id string, args *models.DBInstance) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	args.UpdatedAt = time.Now().Unix()
	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"name":       args.Name,
		"host":       args.Host,
		"port":       args.Port,
		"projects":   args.Projects,
		"username":   args.Username,
		"password":   args.Password,
		"update_by":  args.UpdateBy,
		"updated_at": time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change, options.Update().SetUpsert(true))
	return err
}

func (c *DBInstanceColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)

	return err
}

func (c *DBInstanceColl) ListByProject(projectName string) ([]*models.DBInstance, error) {
	resp := make([]*models.DBInstance, 0)
	query := bson.M{
		"projects": bson.M{
			"$elemMatch": bson.M{
				"$in": bson.A{projectName, setting.AllProjects},
			},
		},
	}

	ctx := context.Background()
	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
