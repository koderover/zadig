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
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

type WebHookColl struct {
	*mongo.Collection

	coll string
}

func NewWebHookColl() *WebHookColl {
	name := models.WebHook{}.TableName()
	return &WebHookColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *WebHookColl) GetCollectionName() string {
	return c.coll
}

func (c *WebHookColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "owner", Value: 1},
			bson.E{Key: "repo", Value: 1},
			bson.E{Key: "address", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := c.Indexes().CreateOne(ctx, mod)

	return err
}

// AddReferenceOrCreate updates the existing record by adding a new reference if there is a matching record,
// otherwise, it will create a new record and return true.
// It is an atomic operation and is safe cross goroutines (because of the unique index).
func (c *WebHookColl) AddReferenceOrCreate(owner, repo, address, ref string) (bool, error) {
	// the query must meet the unique index
	query := bson.M{"owner": owner, "repo": repo, "address": address}
	change := bson.M{
		"$set": bson.M{
			"owner":   owner,
			"repo":    repo,
			"address": address,
		},
		"$addToSet": bson.M{
			"references": ref,
		}}
	opts := options.FindOneAndUpdate().SetUpsert(true)
	err := c.FindOneAndUpdate(context.TODO(), query, change, opts).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return true, nil
		}

		return false, err
	}

	return false, nil
}

// RemoveReference removes a reference in the webhook record
func (c *WebHookColl) RemoveReference(owner, repo, address, ref string) (*models.WebHook, error) {
	// the query must meet the unique index
	query := bson.M{"owner": owner, "repo": repo, "address": address}
	change := bson.M{
		"$pull": bson.M{
			"references": ref,
		}}

	updated := &models.WebHook{}
	err := c.FindOneAndUpdate(context.TODO(), query, change, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(updated)

	// for compatibility
	if err != nil && err != mongo.ErrNoDocuments {
		return updated, err
	}

	return updated, nil
}

// Delete deletes the webhook record
func (c *WebHookColl) Delete(owner, repo, address string) error {
	// the query must meet the unique index
	query := bson.M{"owner": owner, "repo": repo, "address": address}

	_, err := c.DeleteOne(context.TODO(), query)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	return nil
}

// Find find webhook
func (c *WebHookColl) Find(owner, repo, address string) (*models.WebHook, error) {
	query := bson.M{"owner": owner, "repo": repo, "address": address}
	webhook := new(models.WebHook)
	err := c.FindOne(context.TODO(), query).Decode(webhook)
	return webhook, err
}

// Update update hookID
func (c *WebHookColl) Update(owner, repo, address, hookID string) error {
	query := bson.M{"owner": owner, "repo": repo, "address": address}
	change := bson.M{"$set": bson.M{
		"hook_id": hookID,
	}}

	_, err := c.UpdateOne(context.TODO(), query, change)
	return err
}
