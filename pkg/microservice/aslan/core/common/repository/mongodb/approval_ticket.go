/*
Copyright 2024 The KodeRover Authors.

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
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ApprovalTicketColl struct {
	*mongo.Collection

	coll string
}

func NewApprovalTicketColl() *ApprovalTicketColl {
	name := models.ApprovalTicket{}.TableName()
	return &ApprovalTicketColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *ApprovalTicketColl) GetCollectionName() string {
	return c.coll
}

func (c *ApprovalTicketColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "approval_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				bson.E{Key: "execution_window_start", Value: 1},
				bson.E{Key: "execution_window_end", Value: 1},
				bson.E{Key: "users", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())

	return err
}

func (c *ApprovalTicketColl) Create(ticket *models.ApprovalTicket) error {
	if ticket == nil {
		return fmt.Errorf("nil Module args")
	}

	ticket.CreateTime = time.Now().Unix()

	_, err := c.Collection.InsertOne(context.TODO(), ticket)

	return err
}

type ApprovalTicketListOption struct {
	UserEmail  *string
	ProjectKey *string
	Query      *string
	Status     *int32
}

func (c *ApprovalTicketColl) List(opt *ApprovalTicketListOption) ([]*models.ApprovalTicket, error) {
	if opt == nil {
		return nil, fmt.Errorf("nil FindOption")
	}

	query := bson.M{}

	if opt.ProjectKey != nil {
		query["project_key"] = opt.ProjectKey
	}

	if opt.Status != nil {
		query["status"] = opt.Status
	}

	if opt.UserEmail != nil {
		cond := make([]bson.M, 0)
		cond = append(cond, bson.M{
			"users": bson.M{
				"$elemMatch": bson.M{
					"email": opt.UserEmail,
				},
			},
		})
		cond = append(cond, bson.M{
			"users": bson.M{
				"$size": 0,
			},
		})

		query["$or"] = cond
	}

	if opt.Query != nil {
		query["approval_id"] = bson.M{
			"$regex":   opt.Query,
			"$options": "i",
		}
	}

	ctx := context.Background()
	cursor, err := c.Collection.Find(ctx, query, options.Find().SetSort(bson.D{{"create_time", -1}}))
	if err != nil {
		return nil, err
	}

	var resp []*models.ApprovalTicket
	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *ApprovalTicketColl) GetByID(idstring string) (*models.ApprovalTicket, error) {
	resp := new(models.ApprovalTicket)
	id, err := primitive.ObjectIDFromHex(idstring)
	if err != nil {
		return nil, err
	}
	query := bson.M{"_id": id}

	err = c.FindOne(context.TODO(), query).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
