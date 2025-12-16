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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ProjectClusterRelationColl struct {
	*mongo.Collection

	coll string
}

func NewProjectClusterRelationColl() *ProjectClusterRelationColl {
	name := models.ProjectClusterRelation{}.TableName()
	return &ProjectClusterRelationColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ProjectClusterRelationColl) GetCollectionName() string {
	return c.coll
}

func (c *ProjectClusterRelationColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "project_name", Value: 1},
			bson.E{Key: "cluster_id", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *ProjectClusterRelationColl) Create(args *models.ProjectClusterRelation) error {
	if args == nil {
		return errors.New("nil projectClusterRelation info")
	}

	args.CreatedAt = time.Now().Unix()
	_, err := c.InsertOne(context.TODO(), args)

	return err
}

type ProjectClusterRelationOption struct {
	ProjectName string
	ClusterID   string
}

func (c *ProjectClusterRelationColl) List(opt *ProjectClusterRelationOption) ([]*models.ProjectClusterRelation, error) {
	query := bson.M{}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}

	if opt.ClusterID != "" {
		query["cluster_id"] = opt.ClusterID
	}

	var resp []*models.ProjectClusterRelation
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

func (c *ProjectClusterRelationColl) Delete(opt *ProjectClusterRelationOption) error {
	query := bson.M{}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}

	if opt.ClusterID != "" {
		query["cluster_id"] = opt.ClusterID
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}
