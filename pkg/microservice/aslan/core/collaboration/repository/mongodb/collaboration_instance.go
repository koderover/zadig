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

type CollaborationInstanceFindOptions struct {
	ProjectName string
	Name        string
	UserUID     []string
}

type CollaborationInstanceListOptions struct {
	FindOpts []CollaborationInstanceFindOptions
}
type CollaborationInstanceColl struct {
	*mongo.Collection

	coll string
}

func NewCollaborationInstanceColl() *CollaborationInstanceColl {
	name := models.CollaborationInstance{}.TableName()
	return &CollaborationInstanceColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *CollaborationInstanceColl) GetCollectionName() string {
	return c.coll
}

func (c *CollaborationInstanceColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "collaboration_name", Value: 1},
				bson.E{Key: "user_uid", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

func (c *CollaborationInstanceColl) Find(opt *CollaborationInstanceFindOptions) (*models.CollaborationInstance, error) {
	res := &models.CollaborationInstance{}
	query := bson.M{}
	if len(opt.UserUID) > 0 {
		query["user_uid"] = bson.M{"$in": opt.UserUID}
	}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	err := c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}

func (c *CollaborationInstanceColl) Create(args *models.CollaborationInstance) error {
	// avoid panic issue
	if args == nil {
		return errors.New("nil CollaborationInstance")
	}
	now := time.Now().Unix()
	args.CreateTime = now
	args.UpdateTime = now
	args.Revision = 1
	_, err := c.InsertOne(context.TODO(), args)
	return err
}

// TODO: Update->BulkUpdate

func (c *CollaborationInstanceColl) Update(uid string, args *models.CollaborationInstance) error {
	if args == nil {
		return errors.New("nil CollaborationInstance")
	}

	query := bson.M{"collaboration_name": args.CollaborationName, "project_name": args.ProjectName, "user_uid": uid}
	change := bson.M{"$set": bson.M{
		"update_time":     time.Now().Unix(),
		"last_visit_time": args.LastVisitTime,
		"workflows":       args.Workflows,
		"revision":        args.Revision,
		"products":        args.Products,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *CollaborationInstanceColl) ResetRevision(modeName, projectName string) error {
	if len(modeName) == 0 || len(projectName) == 0 {
		return errors.New("nil modeName or projectName")
	}

	query := bson.M{"collaboration_name": modeName, "project_name": projectName}
	change := bson.M{"$set": bson.M{
		"update_time": time.Now().Unix(),
		"revision":    0,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)

	return err
}

func (c *CollaborationInstanceColl) BulkCreate(args []*models.CollaborationInstance) error {
	if len(args) == 0 {
		return nil
	}
	now := time.Now().Unix()
	for _, arg := range args {
		arg.CreateTime = now
		arg.UpdateTime = now
		arg.LastVisitTime = now
	}
	var ois []interface{}
	for _, obj := range args {
		ois = append(ois, obj)
	}
	_, err := c.InsertMany(context.TODO(), ois)
	return err
}

func (c *CollaborationInstanceColl) BulkDelete(opt CollaborationInstanceListOptions) error {
	condition := bson.A{}
	if len(opt.FindOpts) == 0 {
		return nil
	}
	for _, findOpt := range opt.FindOpts {
		condition = append(condition, bson.M{
			"project_name":       findOpt.ProjectName,
			"user_uid":           bson.M{"$in": findOpt.UserUID},
			"collaboration_name": findOpt.Name,
		})
	}
	filter := bson.D{{"$or", condition}}
	_, err := c.DeleteMany(context.TODO(), filter)
	return err
}

func (c *CollaborationInstanceColl) DeleteByProject(projectName string) error {
	query := bson.M{}
	query["project_name"] = projectName
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *CollaborationInstanceColl) List(opt *CollaborationInstanceFindOptions) ([]*models.CollaborationInstance, error) {
	var ret []*models.CollaborationInstance
	query := bson.M{}
	if len(opt.UserUID) > 0 {
		query["user_uid"] = bson.M{"$in": opt.UserUID}
	}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
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
