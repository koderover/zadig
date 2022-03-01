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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type CollaborationModeFindOptions struct {
	ProjectName string
	Name        string
	Members     string
	IsDeleted   bool
}

type CollaborationModeListOptions struct {
	Projects  []string
	Name      string
	Members   []string
	IsDeleted bool
}

type CollaborationModeColl struct {
	*mongo.Collection

	coll string
}

func NewCollaborationModeColl() *CollaborationModeColl {
	name := models.CollaborationMode{}.TableName()
	return &CollaborationModeColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *CollaborationModeColl) GetCollectionName() string {
	return c.coll
}

func (c *CollaborationModeColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "members", Value: 1},
				bson.E{Key: "is_deleted", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "name", Value: 1},
				bson.E{Key: "is_deleted", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *CollaborationModeColl) Update(username string, args *models.CollaborationMode) error {
	if args == nil {
		return errors.New("nil CollaborationMode")
	}
	res, err := c.Find(&CollaborationModeFindOptions{
		Name:        args.Name,
		ProjectName: args.ProjectName,
		IsDeleted:   false,
	})
	if err == nil {
		query := bson.M{"name": args.Name, "project_name": args.ProjectName, "is_deleted": false}
		change := bson.M{"$set": bson.M{
			"update_time": time.Now().Unix(),
			"update_by":   username,
			"members":     args.Members,
			"workflows":   args.Workflows,
			"recycle_day": args.RecycleDay,
			"revision":    res.Revision + 1,
			"products":    args.Products,
		}}

		_, err = c.UpdateOne(context.TODO(), query, change)

		return err
	}
	return nil
}

func (c *CollaborationModeColl) Create(userName string, args *models.CollaborationMode) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil CollaborationMode")
	}
	_, err := c.Find(&CollaborationModeFindOptions{
		Name:        args.Name,
		ProjectName: args.ProjectName,
		IsDeleted:   false,
	})

	if err == nil {
		return errors.New("the collaborationMode is already exist in this project")
	}
	now := time.Now().Unix()
	args.CreateBy = userName
	args.UpdateBy = userName
	args.CreateTime = now
	args.UpdateTime = now
	args.Revision = 1
	args.IsDeleted = false
	_, err = c.InsertOne(context.TODO(), args)
	return err
}

func (c *CollaborationModeColl) DeleteByProject(project string) error {
	query := bson.M{"project_name": project}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *CollaborationModeColl) Delete(username, projectName, name string) error {
	_, err := c.Find(&CollaborationModeFindOptions{
		Name:        name,
		ProjectName: projectName,
		IsDeleted:   false,
	})
	if err == nil {
		deleteQuery := bson.M{"name": name, "project_name": projectName, "is_deleted": true}
		_, err = c.DeleteOne(context.TODO(), deleteQuery)
		if err != nil {
			return err
		}
	}
	query := bson.M{"name": name, "project_name": projectName, "is_deleted": false}
	change := bson.M{"$set": bson.M{
		"delete_time": time.Now().Unix(),
		"delete_by":   username,
		"is_deleted":  true,
	}}
	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *CollaborationModeColl) Find(opt *CollaborationModeFindOptions) (*models.CollaborationMode, error) {
	res := &models.CollaborationMode{}
	query := bson.M{}
	if opt.Name != "" {
		query["name"] = opt.Name
	}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	query["is_deleted"] = false
	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *CollaborationModeColl) List(opt *CollaborationModeListOptions) ([]*models.CollaborationMode, error) {
	var ret []*models.CollaborationMode
	query := bson.M{}
	if opt.Name != "" {
		query["name"] = opt.Name
	}
	if len(opt.Projects) > 0 {
		query["project_name"] = bson.M{"$in": opt.Projects}
	}
	if len(opt.Members) > 0 {
		opt.Members = append(opt.Members, "*")
		query["members"] = bson.M{"$in": opt.Members}
	}
	query["is_deleted"] = false

	ctx := context.Background()
	opts := options.Find()
	opts.SetSort(bson.D{{"create_time", -1}})
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
