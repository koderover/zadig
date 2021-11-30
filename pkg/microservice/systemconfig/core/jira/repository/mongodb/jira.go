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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type JiraColl struct {
	*mongo.Collection

	coll string
}

func NewJiraColl() *JiraColl {
	name := models.Jira{}.TableName()
	coll := &JiraColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *JiraColl) GetCollectionName() string {
	return c.coll
}
func (c *JiraColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *JiraColl) AddJira(iJira *models.Jira) (*models.Jira, error) {
	_, err := c.Collection.InsertOne(context.TODO(), iJira)
	if err != nil {
		log.Error("repository AddJira err : %v", err)
		return nil, err
	}
	return iJira, nil
}

func (c *JiraColl) UpdateJira(iJira *models.Jira) (*models.Jira, error) {

	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"host":         iJira.Host,
		"user":         iJira.User,
		"access_token": iJira.AccessToken,
		"updated_at":   time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository UpdateJira err : %v", err)
		return nil, err
	}
	return iJira, nil
}

func (c *JiraColl) DeleteJira() error {

	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository DeleteJira err : %v", err)
		return err
	}
	return nil
}

func (c *JiraColl) GetJira() (*models.Jira, error) {
	jira := &models.Jira{}
	query := bson.M{"deleted_at": 0}

	err := c.Collection.FindOne(context.TODO(), query).Decode(jira)
	if err != nil {
		return nil, nil
	}
	return jira, nil
}
