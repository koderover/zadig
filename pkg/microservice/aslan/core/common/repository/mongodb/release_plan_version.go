/*
 * Copyright 2026 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mongodb

import (
	"context"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type ReleasePlanVersionColl struct {
	*mongo.Collection

	coll string
}

func NewReleasePlanVersionColl() *ReleasePlanVersionColl {
	name := models.ReleasePlanVersion{}.TableName()
	return &ReleasePlanVersionColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ReleasePlanVersionColl) GetCollectionName() string {
	return c.coll
}

func (c *ReleasePlanVersionColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "plan_id", Value: 1}, {Key: "version", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "plan_id", Value: 1}, {Key: "created_at", Value: -1}},
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *ReleasePlanVersionColl) Create(ctx context.Context, args *models.ReleasePlanVersion) error {
	if args == nil {
		return errors.New("nil ReleasePlanVersion")
	}

	storedVersion, snapshots, err := prepareReleasePlanVersionForStorage(args)
	if err != nil {
		return err
	}
	if err := NewReleasePlanVersionSnapshotColl().Create(ctx, snapshots); err != nil {
		return errors.Wrap(err, "create release plan version snapshots")
	}
	_, err = c.InsertOne(ctx, storedVersion)
	return err
}

func (c *ReleasePlanVersionColl) Upsert(ctx context.Context, args *models.ReleasePlanVersion) error {
	if args == nil {
		return errors.New("nil ReleasePlanVersion")
	}

	storedVersion, snapshots, err := prepareReleasePlanVersionForStorage(args)
	if err != nil {
		return err
	}
	if err := NewReleasePlanVersionSnapshotColl().Upsert(ctx, snapshots); err != nil {
		return errors.Wrap(err, "upsert release plan version snapshots")
	}
	_, err = c.ReplaceOne(ctx, bson.M{
		"plan_id": args.PlanID,
		"version": args.Version,
	}, storedVersion, options.Replace().SetUpsert(true))
	return err
}

func (c *ReleasePlanVersionColl) Delete(ctx context.Context, planID string, version int64) error {
	_, err := c.DeleteOne(ctx, bson.M{
		"plan_id": planID,
		"version": version,
	})
	if err != nil {
		return err
	}
	return NewReleasePlanVersionSnapshotColl().Delete(ctx, planID, version)
}

func (c *ReleasePlanVersionColl) Get(planID string, version int64) (*models.ReleasePlanVersion, error) {
	resp := new(models.ReleasePlanVersion)
	err := c.FindOne(context.Background(), bson.M{
		"plan_id": planID,
		"version": version,
	}).Decode(resp)
	if err != nil {
		return nil, err
	}
	if err := loadReleasePlanVersionSnapshots(context.Background(), resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ReleasePlanVersionColl) GetLatestBySectionsBefore(planID string, sectionKeys []string, beforeVersion int64) (*models.ReleasePlanVersion, error) {
	resp := new(models.ReleasePlanVersion)
	err := c.FindOne(context.Background(), bson.M{
		"plan_id": planID,
		"version": bson.M{"$lt": beforeVersion},
		"section_key": bson.M{
			"$in": sectionKeys,
		},
	}, options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})).Decode(resp)
	if err != nil {
		return nil, err
	}
	if err := loadReleasePlanVersionSnapshots(context.Background(), resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func loadReleasePlanVersionSnapshots(ctx context.Context, version *models.ReleasePlanVersion) error {
	if version.SnapshotStorage == "" {
		return nil
	}
	snapshots, err := NewReleasePlanVersionSnapshotColl().List(ctx, version.PlanID, version.Version)
	if err != nil {
		return errors.Wrap(err, "list release plan version snapshots")
	}
	return restoreReleasePlanVersionSnapshots(version, snapshots)
}
