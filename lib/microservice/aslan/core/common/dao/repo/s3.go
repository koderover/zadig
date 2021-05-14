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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/tool/crypto"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type S3StorageColl struct {
	*mongo.Collection

	coll string
}

func NewS3StorageColl() *S3StorageColl {
	name := models.S3Storage{}.TableName()
	return &S3StorageColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *S3StorageColl) EnsureIndex(ctx context.Context) error {
	return nil
}

func (c *S3StorageColl) GetCollectionName() string {
	return c.coll
}

func (c *S3StorageColl) FindDefault() (*models.S3Storage, error) {
	query := bson.M{"is_default": true}
	storage := new(models.S3Storage)
	err := c.FindOne(context.TODO(), query).Decode(storage)
	if err != nil {
		return nil, err
	}

	decryptedKey, err := crypto.AesDecrypt(storage.EncryptedSk)
	if err != nil {
		return nil, err
	}
	storage.Sk = decryptedKey

	return storage, nil
}

func (c *S3StorageColl) Find(id string) (*models.S3Storage, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	storage := new(models.S3Storage)
	query := bson.M{"_id": oid}
	err = c.FindOne(context.TODO(), query).Decode(storage)
	if err != nil {
		return nil, err
	}

	decryptedKey, err := crypto.AesDecrypt(storage.EncryptedSk)
	if err != nil {
		return nil, err
	}
	storage.Sk = decryptedKey

	return storage, nil
}

func (c *S3StorageColl) GetS3Storage() (*models.S3Storage, error) {
	storage := new(models.S3Storage)
	query := bson.M{
		"endpoint": bson.M{"$regex": primitive.Regex{Pattern: `qiniucs\.com`, Options: "i"}},
		"bucket":   "releases",
	}

	err := c.FindOne(context.TODO(), query).Decode(storage)
	if err != nil {
		return nil, err
	}

	decryptedKey, err := crypto.AesDecrypt(storage.EncryptedSk)
	if err != nil {
		return nil, err
	}
	storage.Sk = decryptedKey

	return storage, nil
}

func (c *S3StorageColl) unsetDefault() error {
	query := bson.M{"is_default": true}
	update := bson.M{"$set": bson.M{"is_default": false}}

	_, err := c.UpdateOne(context.TODO(), query, update)
	return err
}

// Upsert if the updated storage is default, all other default storage will be set as not default
func (c *S3StorageColl) Update(id string, args *models.S3Storage) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	args.ID = oid

	query := bson.M{"_id": args.ID}
	args.UpdateTime = time.Now().Unix()

	encryptedKey, err := crypto.AesEncrypt(args.Sk)
	if err != nil {
		return err
	}
	args.EncryptedSk = encryptedKey

	if args.IsDefault {
		if err := c.unsetDefault(); err != nil {
			return err
		}
	}

	change := bson.M{"$set": args}
	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *S3StorageColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	query := bson.M{"_id": oid}

	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

// Create if the crated storage is default, all other default storage will be set as not default
func (c *S3StorageColl) Create(args *models.S3Storage) error {
	args.UpdateTime = time.Now().Unix()
	encryptedKey, err := crypto.AesEncrypt(args.Sk)
	if err != nil {
		return err
	}
	args.EncryptedSk = encryptedKey

	if args.IsDefault {
		if err := c.unsetDefault(); err != nil {
			return err
		}
	}

	_, err = c.InsertOne(context.TODO(), args)
	return err
}

func (c *S3StorageColl) FindAll() ([]*models.S3Storage, error) {
	var storages []*models.S3Storage
	query := bson.M{}

	cursor, err := c.Collection.Find(context.TODO(), query)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &storages)
	if err != nil {
		return nil, err
	}

	for _, s := range storages {
		decryptedKey, err := crypto.AesDecrypt(s.EncryptedSk)
		if err != nil {
			return nil, err
		}
		s.Sk = decryptedKey
	}

	return storages, nil
}
