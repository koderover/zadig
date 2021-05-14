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

package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/crypto"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
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

func (c *K8SClusterColl) EnsureIndex(_ context.Context) error {
	return nil
}

func (c *K8SClusterColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *K8SClusterColl) Create(cluster *models.K8SCluster) error {
	_, err := c.InsertOne(context.TODO(), cluster)

	return err
}

// Update ...
func (c *K8SClusterColl) Update(cluster *models.K8SCluster) error {
	_, err := c.UpdateOne(context.TODO(), bson.M{"_id": cluster.ID}, bson.M{"$set": cluster})
	return err
}

// Get ...
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

func (c *K8SClusterColl) HasDuplicateName(id, name string) (bool, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}

	query := bson.M{"_id": bson.M{"$ne": oid}, "name": name}

	count, err := c.CountDocuments(context.TODO(), query)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (c *K8SClusterColl) Find(clusterType string) ([]*models.K8SCluster, error) {
	var clusters []*models.K8SCluster

	query := bson.M{}
	if clusterType != "" {
		switch clusterType {
		case setting.ProdENV:
			query["production"] = true
		default:
			query["production"] = false
		}
	}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &clusters)
	if err != nil {
		return nil, err
	}

	return clusters, err
}

func (c *K8SClusterColl) FindByName(name string) (*models.K8SCluster, error) {
	res := &models.K8SCluster{}
	err := c.FindOne(context.TODO(), bson.M{"name": name}).Decode(res)

	return res, err
}

func (c *K8SClusterColl) UpdateMutableFields(cluster *models.K8SCluster) error {
	_, err := c.UpdateOne(context.TODO(),
		bson.M{"_id": cluster.ID}, bson.M{"$set": bson.M{
			"name":        cluster.Name,
			"description": cluster.Description,
			"tags":        cluster.Tags,
			"namespace":   cluster.Namespace,
			"production":  cluster.Production,
		}},
	)

	return err
}

func (c *K8SClusterColl) UpdateStatus(cluster *models.K8SCluster) error {
	_, err := c.UpdateOne(context.TODO(),
		bson.M{"_id": cluster.ID}, bson.M{"$set": bson.M{
			"status": cluster.Status,
		}},
	)

	return err
}

func (c *K8SClusterColl) FindConnectedClusters() ([]*models.K8SCluster, error) {
	var clusters []*models.K8SCluster
	cursor, err := c.Collection.Find(context.TODO(), bson.M{"disconnected": false})
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &clusters)

	return clusters, err
}

func (c *K8SClusterColl) UpdateConnectState(id string, disconnected bool) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	newState := bson.M{"disconnected": disconnected}

	if disconnected {
		newState["status"] = config.Disconnected
	} else {
		newState["status"] = config.Pending
	}

	_, err = c.UpdateMany(context.TODO(), bson.M{"_id": oid}, bson.M{"$set": newState})
	return err
}

func (c *K8SClusterColl) GetByToken(token string) (*models.K8SCluster, error) {
	id, err := crypto.AesDecrypt(token)
	if err != nil {
		return nil, err
	}

	return c.Get(id)
}
