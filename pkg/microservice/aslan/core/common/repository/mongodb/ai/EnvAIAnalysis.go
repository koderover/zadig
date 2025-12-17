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

package ai

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/ai"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type EnvAIAnalysisColl struct {
	*mongo.Collection

	coll string
}

func NewEnvAIAnalysisColl() *EnvAIAnalysisColl {
	name := ai.EnvAIAnalysis{}.TableName()
	return &EnvAIAnalysisColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *EnvAIAnalysisColl) GetCollectionName() string {
	return c.coll
}

func (c *EnvAIAnalysisColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "env_name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

type EnvAIAnalysisListOption struct {
	ProjectName string
	EnvName     string
	Production  bool
	PageNum     int64
	PageSize    int64
}

func (c *EnvAIAnalysisColl) ListByOptions(opts EnvAIAnalysisListOption) ([]*ai.EnvAIAnalysis, int64, error) {
	query := bson.M{}
	if opts.ProjectName != "" {
		query["project_name"] = opts.ProjectName
	}
	if opts.EnvName != "" {
		query["env_name"] = opts.EnvName
	}

	if opts.PageNum == 0 {
		opts.PageNum = 1
	}
	if opts.PageSize == 0 {
		opts.PageSize = 10
	}

	var resp []*ai.EnvAIAnalysis
	opt := options.Find().
		SetSkip((opts.PageNum - 1) * opts.PageSize).
		SetLimit(opts.PageSize).
		SetSort(bson.D{{Key: "start_time", Value: -1}})

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, 0, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, 0, err
	}

	count, err := c.Collection.CountDocuments(context.TODO(), query)
	if err != nil {
		return nil, 0, err
	}

	return resp, count, nil
}

func (c *EnvAIAnalysisColl) Create(args *ai.EnvAIAnalysis) error {
	if args == nil {
		return errors.New("nil Workflow args")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}
