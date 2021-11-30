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

	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type EmailHostColl struct {
	*mongo.Collection

	coll string
}

func NewEmailHostColl() *EmailHostColl {
	name := models.EmailHost{}.TableName()
	coll := &EmailHostColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}

	return coll
}

func (c *EmailHostColl) GetCollectionName() string {
	return c.coll
}
func (c *EmailHostColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *EmailHostColl) Find() (*models.EmailHost, error) {
	emailHost := new(models.EmailHost)
	query := bson.M{"deleted_at": 0}

	ctx := context.Background()

	err := c.Collection.FindOne(ctx, query).Decode(emailHost)
	if err != nil {
		return nil, nil
	}
	return emailHost, nil
}

func (c *EmailHostColl) Update(emailHost *models.EmailHost) (*models.EmailHost, error) {
	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"name":       emailHost.Name,
		"port":       emailHost.Port,
		"username":   emailHost.Username,
		"password":   emailHost.Password,
		"is_tls":     emailHost.IsTLS,
		"updated_at": time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository Update EmailHostColl err : %v", err)
		return nil, err
	}
	return emailHost, nil
}

func (c *EmailHostColl) Delete() error {
	query := bson.M{"deleted_at": 0}
	change := bson.M{"$set": bson.M{
		"deleted_at": time.Now().Unix(),
	}}

	_, err := c.Collection.UpdateOne(context.TODO(), query, change)
	if err != nil {
		log.Error("repository Delete EmailHostColl err : %v", err)
		return err
	}
	return nil
}

func (c *EmailHostColl) Add(emailHost *models.EmailHost) (*models.EmailHost, error) {
	// TODO: tmp solution to avoid bug
	var res []*models.EmailHost
	query := bson.M{"deleted_at": 0}
	cur, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cur.All(context.TODO(), &res)
	if err != nil {
		return nil, err
	}
	if len(res) >= 1 {
		return nil, errors.New("cant add more than one emailhost")
	}

	_, err = c.Collection.InsertOne(context.TODO(), emailHost)
	if err != nil {
		log.Error("repository AddEmailHost err : %v", err)
		return nil, err
	}
	return emailHost, nil
}
