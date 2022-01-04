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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func NewEmailHostColl() *EmailHostColl {
	name := models.EmailHost{}.TableName()
	coll := &EmailHostColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

type EmailHostColl struct {
	*mongo.Collection

	coll string
}

func (c *EmailHostColl) List() ([]*models.EmailHost, error) {
	emailhosts := make([]*models.EmailHost, 0)
	query := bson.M{"deleted_at": 0}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &emailhosts)
	if err != nil {
		return nil, err
	}

	return emailhosts, nil
}

func (c *EmailHostColl) ChangeType(ID primitive.ObjectID, isTLS string) error {
	query := bson.M{"_id": ID}
	changedTLS := false
	if isTLS == "2" {
		changedTLS = true
	}
	change := bson.M{"$set": bson.M{
		"is_tls": changedTLS,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Errorf("emailhost update fail,err:%s", err)
		return err
	}
	log.Infof("success to change id:%v isTLS:%s to %v", ID, isTLS, changedTLS)
	return nil
}

func (c *EmailHostColl) RollbackType(ID primitive.ObjectID, isTLS bool) error {
	query := bson.M{"_id": ID}
	changedTLS := "1"
	if isTLS == true {
		changedTLS = "2"
	}
	change := bson.M{"$set": bson.M{
		"is_tls": changedTLS,
	}}
	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Errorf("emailhost update fail,err:%s", err)
		return err
	}
	log.Infof("success to change id:%v isTLS:%v to %s", ID, isTLS, changedTLS)
	return nil
}
