/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type MsgQueueCommonColl struct {
	*mongo.Collection

	coll string
}

func NewMsgQueueCommonColl() *MsgQueueCommonColl {
	name := msg_queue.MsgQueueCommon{}.TableName()
	return &MsgQueueCommonColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *MsgQueueCommonColl) GetCollectionName() string {
	return c.coll
}

func (c *MsgQueueCommonColl) EnsureIndex(ctx context.Context) error {
	return nil
}

type ListMsgQueueCommonOption struct {
	QueueType string
}

func (c *MsgQueueCommonColl) List(opt *ListMsgQueueCommonOption) ([]*msg_queue.MsgQueueCommon, error) {
	query := bson.M{}
	sort := bson.D{}
	if opt != nil {
		if opt.QueueType != "" {
			query["queue_type"] = opt.QueueType
		}
	}

	var resp []*msg_queue.MsgQueueCommon
	ctx := context.Background()
	opts := options.Find().SetSort(sort)
	cursor, err := c.Collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *MsgQueueCommonColl) Delete(id primitive.ObjectID) error {
	query := bson.M{"_id": id}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *MsgQueueCommonColl) DeleteByQueueType(_type string) error {
	query := bson.M{"queue_type": _type}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

func (c *MsgQueueCommonColl) Create(args *msg_queue.MsgQueueCommon) error {
	_, err := c.InsertOne(context.TODO(), args)
	return err
}
