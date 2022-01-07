/*
Copyright 2022 The KodeRover Authors.

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

	models "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type K8SClusterColl struct {
	*mongo.Collection

	coll string
}

func NewK8SClusterColl() *K8SClusterColl {
	name := models.K8SCluster{}.TableName()
	return &K8SClusterColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *K8SClusterColl) GetCollectionName() string {
	return c.coll
}

func (c *K8SClusterColl) List() ([]*models.K8SCluster, error) {
	var clusters []*models.K8SCluster

	query := bson.M{}
	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	err = cursor.All(context.TODO(), &clusters)
	if err != nil {
		return nil, err
	}

	return clusters, nil
}

func (c *K8SClusterColl) Create(cluster *models.K8SCluster, id string) error {
	if id != "" {
		cluster.ID, _ = primitive.ObjectIDFromHex(id)
		// If the local cluster already exists, do not insert，and return nil
		if _, err := c.Get(id); err == nil {
			return nil
		}
	}
	_, err := c.InsertOne(context.TODO(), cluster)
	return err
}

func (c *K8SClusterColl) Get(id string) (*models.K8SCluster, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	res := &models.K8SCluster{}
	err = c.FindOne(context.TODO(), query).Decode(res)

	return res, err
}
