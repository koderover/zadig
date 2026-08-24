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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

const releasePlanVersionSnapshotStorageGZIPJSONV1 = "external-gzip-json-v1"

type ReleasePlanVersionSnapshotColl struct {
	*mongo.Collection

	coll string
}

func NewReleasePlanVersionSnapshotColl() *ReleasePlanVersionSnapshotColl {
	name := models.ReleasePlanVersionSnapshot{}.TableName()
	return &ReleasePlanVersionSnapshotColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ReleasePlanVersionSnapshotColl) GetCollectionName() string {
	return c.coll
}

func (c *ReleasePlanVersionSnapshotColl) EnsureIndex(ctx context.Context) error {
	_, err := c.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "plan_id", Value: 1},
			{Key: "version", Value: 1},
			{Key: "kind", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *ReleasePlanVersionSnapshotColl) Create(ctx context.Context, snapshots []*models.ReleasePlanVersionSnapshot) error {
	if len(snapshots) == 0 {
		return errors.New("empty release plan version snapshots")
	}
	documents := make([]interface{}, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			return errors.New("nil release plan version snapshot")
		}
		documents = append(documents, snapshot)
	}
	_, err := c.InsertMany(ctx, documents)
	return err
}

func (c *ReleasePlanVersionSnapshotColl) Upsert(ctx context.Context, snapshots []*models.ReleasePlanVersionSnapshot) error {
	if len(snapshots) == 0 {
		return errors.New("empty release plan version snapshots")
	}
	writes := make([]mongo.WriteModel, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			return errors.New("nil release plan version snapshot")
		}
		writes = append(writes, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"plan_id": snapshot.PlanID, "version": snapshot.Version, "kind": snapshot.Kind}).
			SetReplacement(snapshot).
			SetUpsert(true))
	}
	_, err := c.BulkWrite(ctx, writes)
	return err
}

func (c *ReleasePlanVersionSnapshotColl) Delete(ctx context.Context, planID string, version int64) error {
	_, err := c.DeleteMany(ctx, bson.M{"plan_id": planID, "version": version})
	return err
}

func (c *ReleasePlanVersionSnapshotColl) List(ctx context.Context, planID string, version int64) ([]*models.ReleasePlanVersionSnapshot, error) {
	cursor, err := c.Find(ctx, bson.M{"plan_id": planID, "version": version})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	resp := make([]*models.ReleasePlanVersionSnapshot, 0, 2)
	if err := cursor.All(ctx, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func prepareReleasePlanVersionForStorage(version *models.ReleasePlanVersion) (*models.ReleasePlanVersion, []*models.ReleasePlanVersionSnapshot, error) {
	if version == nil {
		return nil, nil, errors.New("nil ReleasePlanVersion")
	}
	if version.Snapshot == nil {
		return nil, nil, errors.New("nil release plan version snapshot")
	}

	storedVersion := *version
	storedVersion.SnapshotStorage = releasePlanVersionSnapshotStorageGZIPJSONV1
	storedVersion.HasBaseSnapshot = version.BaseSnapshot != nil
	storedVersion.BaseSnapshot = nil
	storedVersion.Snapshot = nil

	snapshots := make([]*models.ReleasePlanVersionSnapshot, 0, 2)
	if version.BaseSnapshot != nil {
		snapshot, err := newReleasePlanVersionSnapshot(version, models.ReleasePlanVersionSnapshotKindBase, version.BaseSnapshot)
		if err != nil {
			return nil, nil, errors.Wrap(err, "encode base snapshot")
		}
		snapshots = append(snapshots, snapshot)
	}
	currentSnapshot, err := newReleasePlanVersionSnapshot(version, models.ReleasePlanVersionSnapshotKindCurrent, version.Snapshot)
	if err != nil {
		return nil, nil, errors.Wrap(err, "encode current snapshot")
	}
	snapshots = append(snapshots, currentSnapshot)
	return &storedVersion, snapshots, nil
}

func newReleasePlanVersionSnapshot(version *models.ReleasePlanVersion, kind models.ReleasePlanVersionSnapshotKind, value interface{}) (*models.ReleasePlanVersionSnapshot, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, errors.Wrap(err, "marshal snapshot")
	}

	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		return nil, errors.Wrap(err, "create gzip writer")
	}
	if _, err := writer.Write(payload); err != nil {
		return nil, errors.Wrap(err, "compress snapshot")
	}
	if err := writer.Close(); err != nil {
		return nil, errors.Wrap(err, "finish compressing snapshot")
	}

	return &models.ReleasePlanVersionSnapshot{
		PlanID:       version.PlanID,
		Version:      version.Version,
		Kind:         kind,
		Data:         compressed.Bytes(),
		OriginalSize: int64(len(payload)),
		CreatedAt:    version.CreatedAt,
	}, nil
}

func restoreReleasePlanVersionSnapshots(version *models.ReleasePlanVersion, snapshots []*models.ReleasePlanVersionSnapshot) error {
	if version == nil {
		return errors.New("nil ReleasePlanVersion")
	}
	if version.SnapshotStorage == "" {
		return nil
	}
	if version.SnapshotStorage != releasePlanVersionSnapshotStorageGZIPJSONV1 {
		return fmt.Errorf("unsupported release plan version snapshot storage: %s", version.SnapshotStorage)
	}

	values := make(map[models.ReleasePlanVersionSnapshotKind]interface{}, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		if snapshot.PlanID != version.PlanID || snapshot.Version != version.Version {
			return errors.New("release plan version snapshot does not match version")
		}
		if _, exists := values[snapshot.Kind]; exists {
			return fmt.Errorf("duplicate %s snapshot", snapshot.Kind)
		}
		value, err := decodeReleasePlanVersionSnapshot(snapshot)
		if err != nil {
			return errors.Wrapf(err, "decode %s snapshot", snapshot.Kind)
		}
		values[snapshot.Kind] = value
	}

	currentSnapshot, exists := values[models.ReleasePlanVersionSnapshotKindCurrent]
	if !exists {
		return errors.New("missing current snapshot")
	}
	version.Snapshot = currentSnapshot
	if version.HasBaseSnapshot {
		baseSnapshot, exists := values[models.ReleasePlanVersionSnapshotKindBase]
		if !exists {
			return errors.New("missing base snapshot")
		}
		version.BaseSnapshot = baseSnapshot
	}
	return nil
}

func decodeReleasePlanVersionSnapshot(snapshot *models.ReleasePlanVersionSnapshot) (interface{}, error) {
	if snapshot == nil {
		return nil, errors.New("nil release plan version snapshot")
	}
	reader, err := gzip.NewReader(bytes.NewReader(snapshot.Data))
	if err != nil {
		return nil, errors.Wrap(err, "create gzip reader")
	}
	defer reader.Close()

	payload, err := io.ReadAll(io.LimitReader(reader, snapshot.OriginalSize+1))
	if err != nil {
		return nil, errors.Wrap(err, "decompress snapshot")
	}
	if int64(len(payload)) != snapshot.OriginalSize {
		return nil, errors.New("snapshot size mismatch")
	}
	var value interface{}
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, errors.Wrap(err, "unmarshal snapshot")
	}
	return value, nil
}
