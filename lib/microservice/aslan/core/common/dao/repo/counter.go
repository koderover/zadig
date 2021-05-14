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
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type CounterColl struct {
	*mongo.Collection

	coll string
}

func NewCounterColl() *CounterColl {
	name := models.Counter{}.TableName()
	return &CounterColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *CounterColl) GetCollectionName() string {
	return c.coll
}

func (c *CounterColl) EnsureIndex(_ context.Context) error {
	return nil
}

func (c *CounterColl) GetNextSeq(counterName string) (int64, error) {
	ct := &models.Counter{}
	query := bson.M{"_id": counterName}

	opts := options.FindOneAndUpdate()
	opts.SetUpsert(true)
	opts.SetReturnDocument(options.After)

	res := c.FindOneAndUpdate(context.TODO(), query, bson.M{"$inc": bson.M{"seq": int64(1)}}, opts)
	if res.Err() != nil {
		return 0, res.Err()
	}
	err := res.Decode(ct)
	if err != nil {
		return 0, err
	}
	return ct.Seq, nil
}

func (c *CounterColl) Delete(counterName string) error {
	query := bson.M{"_id": counterName}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *CounterColl) Rename(oldName, newName string) error {
	old, err := c.Find(oldName)
	if err != nil {
		// Find().One() 没有找到数据会返回err not found
		// 旧的pipeline没有counter数据的话，直接返回即可
		if err == mongo.ErrNoDocuments {
			return nil
		}

		return fmt.Errorf("counter name %s error: %v", oldName, err)
	}

	if counter, _ := c.Find(newName); counter != nil {
		return fmt.Errorf("counter name %s already exsited", newName)
	}

	newCounter := &models.Counter{
		ID:  newName,
		Seq: old.Seq,
	}
	_, err = c.InsertOne(context.TODO(), newCounter)
	if err != nil {
		return err
	}

	return c.Delete(oldName)
}

func (c *CounterColl) Find(name string) (*models.Counter, error) {
	var counter *models.Counter
	query := bson.M{"_id": name}

	err := c.FindOne(context.TODO(), query).Decode(&counter)
	return counter, err
}

func (c *CounterColl) SetToNumber(counterName string, number int64) error {
	query := bson.M{"_id": counterName}

	var counter *models.Counter

	counter.Seq = number
	change := bson.M{"$set": counter}
	singleResult := c.FindOneAndUpdate(context.TODO(), query, change)
	if singleResult != nil && singleResult.Err() != nil {
		return fmt.Errorf(" [%s] ResetToNumber %v error: %v", counterName, number, singleResult.Err())
	}

	return nil
}
