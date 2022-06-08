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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/microservice/hubserver/config"
	"github.com/koderover/zadig/pkg/microservice/hubserver/core/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type K8sClusterColl struct {
	*mongo.Collection

	coll string
}

func NewK8sClusterColl() *K8sClusterColl {
	name := models.K8SCluster{}.TableName()
	coll := &K8sClusterColl{Collection: mongotool.Database(config.AslanDBName()).Collection(name), coll: name}

	return coll
}

func (c *K8sClusterColl) GetCollectionName() string {
	return c.coll
}

func (c *K8sClusterColl) FindConnectedClusters() ([]*models.K8SCluster, error) {
	query := bson.M{"disconnected": false}

	ctx := context.Background()
	resp := make([]*models.K8SCluster, 0)

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *K8sClusterColl) UpdateStatus(cluster *models.K8SCluster) error {
	query := bson.M{"_id": cluster.ID}

	update := bson.M{"$set": bson.M{
		"last_connection_time": cluster.LastConnectionTime,
		"status":               cluster.Status,
	}}

	_, err := c.UpdateOne(context.TODO(), query, update)

	return err
}

func (c *K8sClusterColl) UpdateConnectState(id string, disconnected bool) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"disconnected": disconnected}

	if disconnected {
		change["status"] = config.Disconnected
	} else {
		change["last_connection_time"] = time.Now().Unix()
		change["status"] = config.Pending
	}
	update := bson.M{"$set": change}
	_, err = c.UpdateOne(context.TODO(), query, update)

	return err
}

func (c *K8sClusterColl) Get(id string) (*models.K8SCluster, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	res := &models.K8SCluster{}

	err = c.FindOne(context.TODO(), query).Decode(res)
	return res, err
}
