/*
Copyright 2025 The KodeRover Authors.

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
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type TemporaryFileColl struct {
	*mongo.Collection

	coll string
}

func NewTemporaryFileColl() *TemporaryFileColl {
	name := models.TemporaryFile{}.TableName()
	return &TemporaryFileColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *TemporaryFileColl) GetCollectionName() string {
	return c.coll
}

func (c *TemporaryFileColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "session_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "expires_at", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "instance_id", Value: 1},
			},
		},
	}

	_, err := c.Indexes().CreateMany(ctx, indexes, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

func (c *TemporaryFileColl) Create(temporaryFile *models.TemporaryFile) error {
	if temporaryFile == nil {
		return nil
	}

	now := time.Now().Unix()
	if temporaryFile.CreatedAt == 0 {
		temporaryFile.CreatedAt = now
	}
	if temporaryFile.UpdatedAt == 0 {
		temporaryFile.UpdatedAt = now
	}

	// Set expiration to 24 hours from now if not set
	if temporaryFile.ExpiresAt == 0 {
		temporaryFile.ExpiresAt = now + (24 * 60 * 60) // 24 hours in seconds
	}

	_, err := c.InsertOne(context.TODO(), temporaryFile)
	return err
}

func (c *TemporaryFileColl) GetBySessionID(sessionID string) (*models.TemporaryFile, error) {
	temporaryFile := &models.TemporaryFile{}
	query := bson.M{"session_id": sessionID}
	err := c.FindOne(context.TODO(), query).Decode(temporaryFile)
	if err != nil {
		return nil, err
	}
	return temporaryFile, nil
}

func (c *TemporaryFileColl) GetByID(id string) (*models.TemporaryFile, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	temporaryFile := &models.TemporaryFile{}
	query := bson.M{"_id": oid}
	err = c.FindOne(context.TODO(), query).Decode(temporaryFile)
	if err != nil {
		return nil, err
	}
	return temporaryFile, nil
}

func (c *TemporaryFileColl) UpdateStatus(sessionID string, status string) error {
	query := bson.M{"session_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now().Unix(),
		},
	}
	_, err := c.UpdateOne(context.TODO(), query, update)
	return err
}

func (c *TemporaryFileColl) UpdateUploadedParts(sessionID string, uploadedParts []int) error {
	query := bson.M{"session_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"uploaded_parts": uploadedParts,
			"updated_at":     time.Now().Unix(),
		},
	}
	_, err := c.UpdateOne(context.TODO(), query, update)
	return err
}

func (c *TemporaryFileColl) UpdateFileInfo(sessionID string, storageID, filePath, fileHash string) error {
	query := bson.M{"session_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"storage_id": storageID,
			"file_path":  filePath,
			"file_hash":  fileHash,
			"status":     models.TemporaryFileStatusCompleted,
			"updated_at": time.Now().Unix(),
		},
	}
	_, err := c.UpdateOne(context.TODO(), query, update)
	return err
}

func (c *TemporaryFileColl) Delete(sessionID string) error {
	query := bson.M{"session_id": sessionID}
	_, err := c.DeleteOne(context.TODO(), query)
	return err
}

func (c *TemporaryFileColl) DeleteByID(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

func (c *TemporaryFileColl) ListExpired() ([]*models.TemporaryFile, error) {
	var temporaryFiles []*models.TemporaryFile
	query := bson.M{
		"expires_at": bson.M{"$lt": time.Now().Unix()},
		"status":     bson.M{"$ne": models.TemporaryFileStatusCompleted},
	}
	cursor, err := c.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	err = cursor.All(context.TODO(), &temporaryFiles)
	return temporaryFiles, err
}

func (c *TemporaryFileColl) ListByInstanceID(instanceID string) ([]*models.TemporaryFile, error) {
	var temporaryFiles []*models.TemporaryFile
	query := bson.M{"instance_id": instanceID}
	cursor, err := c.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	err = cursor.All(context.TODO(), &temporaryFiles)
	return temporaryFiles, err
}

func (c *TemporaryFileColl) CleanupExpired() error {
	query := bson.M{
		"expires_at": bson.M{"$lt": time.Now().Unix()},
		"status":     bson.M{"$ne": models.TemporaryFileStatusCompleted},
	}
	_, err := c.DeleteMany(context.TODO(), query)
	return err
}
