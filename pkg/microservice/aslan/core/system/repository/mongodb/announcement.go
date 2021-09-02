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
	models2 "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type AnnouncementColl struct {
	*mongo.Collection

	coll string
}

func NewAnnouncementColl() *AnnouncementColl {
	name := models2.Announcement{}.TableName()
	return &AnnouncementColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *AnnouncementColl) GetCollectionName() string {
	return c.coll
}

func (c *AnnouncementColl) EnsureIndex(_ context.Context) error {
	return nil
}

func (c *AnnouncementColl) Create(args *models2.Announcement) error {
	args.CreateTime = time.Now().Unix()

	_, err := c.InsertOne(context.TODO(), args)

	return err
}

func (c *AnnouncementColl) Update(id string, args *models2.Announcement) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = c.UpdateByID(context.TODO(), oid, bson.M{"$set": args})

	return err
}

func (c *AnnouncementColl) List(receiver string) ([]*models2.Announcement, error) {
	var res []*models2.Announcement

	query := bson.M{"receiver": receiver}
	opts := options.Find().SetSort(bson.D{{"create_time", -1}}).SetLimit(100)
	cursor, err := c.Collection.Find(context.TODO(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *AnnouncementColl) ListValidAnnouncements(receiver string) ([]*models2.Announcement, error) {
	var res []*models2.Announcement

	query := bson.M{"receiver": receiver}
	now := time.Now().Unix()
	query["content.start_time"] = bson.M{"$lt": now}
	query["content.end_time"] = bson.M{"$gt": now}

	opts := options.Find().SetSort(bson.D{{"create_time", -1}}).SetLimit(100)
	cursor, err := c.Collection.Find(context.TODO(), query, opts)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

type AnnouncementDeleteArgs struct {
	ID string
}

func (c *AnnouncementColl) DeleteAnnouncement(args *AnnouncementDeleteArgs) error {
	query := bson.M{}

	if args.ID != "" {
		ID, err := primitive.ObjectIDFromHex(args.ID)
		if err != nil {
			return errors.New("invalid id to delete")
		}
		query["_id"] = ID
	}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}
