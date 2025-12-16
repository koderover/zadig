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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type DiffNoteFindOpt struct {
	CodehostID     int
	ProjectID      string
	MergeRequestID int
}

type DiffNoteColl struct {
	*mongo.Collection

	coll string
}

func NewDiffNoteColl() *DiffNoteColl {
	name := models.DiffNote{}.TableName()
	return &DiffNoteColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *DiffNoteColl) GetCollectionName() string {
	return c.coll
}

func (c *DiffNoteColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "repo.codehost_id", Value: 1},
			bson.E{Key: "repo.project_id", Value: 1},
			bson.E{Key: "merge_request_id", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}

	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *DiffNoteColl) Find(opt *DiffNoteFindOpt) (*models.DiffNote, error) {
	query := bson.M{}
	diffNote := &models.DiffNote{}
	if opt.CodehostID != 0 {
		query["repo.codehost_id"] = opt.CodehostID
	}
	if opt.ProjectID != "" {
		query["repo.project_id"] = opt.ProjectID
	}
	if opt.MergeRequestID != 0 {
		query["merge_request_id"] = opt.MergeRequestID
	}

	err := c.FindOne(context.TODO(), query).Decode(diffNote)
	return diffNote, err
}

func (c *DiffNoteColl) Create(args *models.DiffNote) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil diff_note info")
	}

	args.CreateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *DiffNoteColl) Update(id, commitID, body string, resolved bool) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}

	set := bson.M{
		"body":        body,
		"resolved":    resolved,
		"update_time": time.Now().Unix(),
	}
	if commitID != "" {
		set["commit_id"] = commitID
	}

	change := bson.M{"$set": set}

	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}
