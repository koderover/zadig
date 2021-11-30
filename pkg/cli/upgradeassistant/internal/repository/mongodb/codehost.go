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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func (c *CodehostColl) ChangeType(ID int, sourceType string) error {
	query := bson.M{"id": ID}

	preType := sourceType
	if sourceType == "1" {
		sourceType = "gitlab"
	} else if sourceType == "2" {
		sourceType = "github"
	} else if sourceType == "3" {
		sourceType = "gerrit"
	} else if sourceType == "4" {
		sourceType = "codehub"
	} else {
		return nil
	}

	change := bson.M{"$set": bson.M{
		"type": sourceType,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Errorf("repository update fail,err:%s", err)
		return err
	}
	log.Infof("success to change id:%d type:%s to type:%s", ID, preType, sourceType)
	return nil
}

func (c *CodehostColl) RollbackType(ID int, sourceType string) error {
	query := bson.M{"id": ID}

	if sourceType == "gitlab" {
		sourceType = "1"
	} else if sourceType == "github" {
		sourceType = "2"
	} else if sourceType == "gerrit" {
		sourceType = "3"
	} else if sourceType == "codehub" {
		sourceType = "4"
	} else {
		return nil
	}

	change := bson.M{"$set": bson.M{
		"type": sourceType,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Errorf("repository update fail,err:%s", err)
		return err
	}
	log.Infof("success to rollback id:%d to type:%s", ID, sourceType)
	return nil
}

func (c *CodehostColl) List() ([]*models.CodeHost, error) {
	codeHosts := make([]*models.CodeHost, 0)
	query := bson.M{"deleted_at": 0}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &codeHosts)
	if err != nil {
		return nil, err
	}

	return codeHosts, nil
}

type CodehostColl struct {
	*mongo.Collection

	coll string
}

type ListArgs struct {
	Owner   string
	Address string
	Source  string
}

func NewCodehostColl() *CodehostColl {
	name := models.CodeHost{}.TableName()
	coll := &CodehostColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}
