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

type MsgQueuePipelineTaskColl struct {
	*mongo.Collection

	coll string
}

func NewMsgQueuePipelineTaskColl() *MsgQueuePipelineTaskColl {
	name := msg_queue.MsgQueuePipelineTask{}.TableName()
	return &MsgQueuePipelineTaskColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *MsgQueuePipelineTaskColl) GetCollectionName() string {
	return c.coll
}

func (c *MsgQueuePipelineTaskColl) EnsureIndex(ctx context.Context) error {
	return nil
}

type ListMsgQueuePipelineTaskOption struct {
	QueueType   string
	SortQueueID bool
}

func (c *MsgQueuePipelineTaskColl) List(opt *ListMsgQueuePipelineTaskOption) ([]*msg_queue.MsgQueuePipelineTask, error) {
	query := bson.M{}
	sort := bson.D{}
	if opt != nil {
		if opt.QueueType != "" {
			query["queue_type"] = opt.QueueType
		}
		if opt.SortQueueID {
			sort = append(sort, bson.E{Key: "queue_id", Value: 1})
		}
	}

	var resp []*msg_queue.MsgQueuePipelineTask
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

func (c *MsgQueuePipelineTaskColl) Delete(id primitive.ObjectID) error {
	query := bson.M{"_id": id}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *MsgQueuePipelineTaskColl) Create(args *msg_queue.MsgQueuePipelineTask) error {
	_, err := c.InsertOne(context.TODO(), args)
	return err
}
