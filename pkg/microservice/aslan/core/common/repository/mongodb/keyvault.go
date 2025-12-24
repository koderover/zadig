/*
Copyright 2024 The KodeRover Authors.

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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type KeyVaultItemColl struct {
	*mongo.Collection

	coll string
}

func NewKeyVaultItemColl() *KeyVaultItemColl {
	name := models.KeyVaultItem{}.TableName()
	return &KeyVaultItemColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *KeyVaultItemColl) GetCollectionName() string {
	return c.coll
}

func (c *KeyVaultItemColl) EnsureIndex(ctx context.Context) error {
	mod := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "group", Value: 1},
			bson.E{Key: "key", Value: 1},
			bson.E{Key: "project_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, mod, options.CreateIndexes().SetCommitQuorumMajority())
	return err
}

type KeyVaultItemListOption struct {
	ProjectName  string
	Group        string
	IsSystemWide bool
}

// List returns items based on filter options
// If ProjectName is provided, returns items that are system-wide OR belong to that project
// If Group is provided, filters by group as well
func (c *KeyVaultItemColl) List(opt *KeyVaultItemListOption) ([]*models.KeyVaultItem, error) {
	query := bson.M{}

	if opt != nil {
		if opt.ProjectName != "" {
			query["project_name"] = opt.ProjectName
		}
		if opt.IsSystemWide {
			query["is_system_wide"] = true
		}
		if opt.Group != "" {
			query["group"] = opt.Group
		}
	}

	resp := make([]*models.KeyVaultItem, 0)
	ctx := context.Background()

	cursor, err := c.Collection.Find(ctx, query)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type KeyVaultGroupListOption struct {
	ProjectName  string
	IsSystemWide bool
}

// ListGroups returns distinct group names based on filter options
func (c *KeyVaultItemColl) ListGroups(opt *KeyVaultGroupListOption) ([]string, error) {
	query := bson.M{}

	if opt != nil {
		if opt.ProjectName != "" {
			query["project_name"] = opt.ProjectName
		}
		if opt.IsSystemWide {
			query["is_system_wide"] = true
		}
	}

	ctx := context.Background()
	groups, err := c.Collection.Distinct(ctx, "group", query)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(groups))
	for _, g := range groups {
		if str, ok := g.(string); ok {
			result = append(result, str)
		}
	}

	return result, nil
}

func (c *KeyVaultItemColl) FindByID(id string) (*models.KeyVaultItem, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	query := bson.M{"_id": oid}
	resp := &models.KeyVaultItem{}
	ctx := context.Background()

	err = c.Collection.FindOne(ctx, query).Decode(resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type KeyVaultItemFindOption struct {
	Group        string
	Key          string
	ProjectName  string
	IsSystemWide bool
}

func (c *KeyVaultItemColl) Find(opt *KeyVaultItemFindOption) (*models.KeyVaultItem, error) {
	if opt == nil || opt.Group == "" || opt.Key == "" {
		return nil, errors.New("nil or empty group or key")
	}

	if opt.ProjectName != "" && opt.IsSystemWide {
		return nil, errors.New("project name and system wide cannot be both set")
	}

	if opt.ProjectName == "" && !opt.IsSystemWide {
		return nil, errors.New("project name or system wide must be set")
	}

	query := bson.M{}
	if opt.ProjectName != "" {
		query["project_name"] = opt.ProjectName
	}
	if opt.IsSystemWide {
		query["is_system_wide"] = true
	}
	query["group"] = opt.Group
	query["key"] = opt.Key

	resp := &models.KeyVaultItem{}
	ctx := context.Background()

	err := c.Collection.FindOne(ctx, query).Decode(resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *KeyVaultItemColl) Create(args *models.KeyVaultItem) error {
	if args == nil {
		return errors.New("nil keyvault item")
	}

	now := time.Now().Unix()
	args.CreatedAt = now
	args.UpdatedAt = now

	_, err := c.InsertOne(context.TODO(), args)
	return err
}

func (c *KeyVaultItemColl) Update(id string, args *models.KeyVaultItem) error {
	if args == nil {
		return errors.New("nil keyvault item")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	change := bson.M{"$set": bson.M{
		"group":           args.Group,
		"key":             args.Key,
		"value":           args.Value,
		"is_sensitive":    args.IsSensitive,
		"description":     args.Description,
		"project_name":    args.ProjectName,
		"updated_by":      args.UpdatedBy,
		"updated_at":      time.Now().Unix(),
	}}

	_, err = c.UpdateOne(context.TODO(), query, change)
	return err
}

func (c *KeyVaultItemColl) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	query := bson.M{"_id": oid}
	_, err = c.DeleteOne(context.TODO(), query)
	return err
}

type KeyVaultItemDeleteOption struct {
	ProjectName  string
	IsSystemWide bool
}

// DeleteByGroup deletes all items in a group (optionally filtered by project)
func (c *KeyVaultItemColl) DeleteByGroup(group string, deleteOpt *KeyVaultItemDeleteOption) error {
	query := bson.M{"group": group}
	if deleteOpt != nil && deleteOpt.ProjectName != "" {
		query["project_name"] = deleteOpt.ProjectName
	}
	if deleteOpt != nil && deleteOpt.IsSystemWide {
		query["is_system_wide"] = true
	}

	_, err := c.DeleteMany(context.TODO(), query)
	return err
}

