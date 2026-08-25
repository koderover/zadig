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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonoptions"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

const (
	releasePlanJobSpecsEncodingGZIPBSONV1 = "gzip-bson-v1"
	releasePlanJobSpecChunkSize           = 4 * 1024 * 1024
)

var releasePlanJobSpecsBSONRegistry = bson.NewRegistryBuilder().
	RegisterTypeMapEntry(bsontype.EmbeddedDocument, reflect.TypeOf(bson.M{})).
	RegisterDefaultEncoder(reflect.Slice, bsoncodec.NewSliceCodec(bsonoptions.SliceCodec().SetEncodeNilAsEmpty(true))).
	Build()

type releasePlanJobSpecEntry struct {
	JobID string      `bson:"job_id" json:"job_id"`
	Spec  interface{} `bson:"spec" json:"spec"`
}

type releasePlanJobSpecsPayload struct {
	Jobs []*releasePlanJobSpecEntry `bson:"jobs" json:"jobs"`
}

type encodedReleasePlanJobSpecs struct {
	compressed    []byte
	originalSize  int64
	sha256        string
	contentSHA256 string
}

type ReleasePlanJobSpecChunkColl struct {
	*mongo.Collection

	coll string
}

func NewReleasePlanJobSpecChunkColl() *ReleasePlanJobSpecChunkColl {
	name := models.ReleasePlanJobSpecChunk{}.TableName()
	return &ReleasePlanJobSpecChunkColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *ReleasePlanJobSpecChunkColl) GetCollectionName() string {
	return c.coll
}

func (c *ReleasePlanJobSpecChunkColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "storage_id", Value: 1}, {Key: "sequence", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "plan_id", Value: 1}}},
	}
	_, err := c.Indexes().CreateMany(ctx, indexes, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *ReleasePlanJobSpecChunkColl) Create(ctx context.Context, chunks []*models.ReleasePlanJobSpecChunk) error {
	if len(chunks) == 0 {
		return errors.New("empty release plan job spec chunks")
	}
	documents := make([]interface{}, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == nil {
			return errors.New("nil release plan job spec chunk")
		}
		documents = append(documents, chunk)
	}
	_, err := c.InsertMany(ctx, documents)
	return err
}

func (c *ReleasePlanJobSpecChunkColl) List(ctx context.Context, planID string, storageID primitive.ObjectID) ([]*models.ReleasePlanJobSpecChunk, error) {
	cursor, err := c.Find(ctx, bson.M{"plan_id": planID, "storage_id": storageID}, options.Find().SetSort(bson.D{{Key: "sequence", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var chunks []*models.ReleasePlanJobSpecChunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

func (c *ReleasePlanJobSpecChunkColl) Delete(ctx context.Context, storageID primitive.ObjectID) error {
	_, err := c.DeleteMany(ctx, bson.M{"storage_id": storageID})
	return err
}

func prepareReleasePlanForStorage(plan *models.ReleasePlan, existingRef *models.ReleasePlanJobSpecsRef) (*models.ReleasePlan, []*models.ReleasePlanJobSpecChunk, error) {
	storedPlan, payload, err := buildReleasePlanJobSpecsPayload(plan)
	if err != nil {
		return nil, nil, err
	}
	if payload == nil {
		return storedPlan, nil, nil
	}

	encoded, err := encodeReleasePlanJobSpecs(payload)
	if err != nil {
		return nil, nil, err
	}
	if existingRef != nil && existingRef.Encoding == releasePlanJobSpecsEncodingGZIPBSONV1 && existingRef.ContentSHA256 == encoded.contentSHA256 {
		ref := *existingRef
		storedPlan.JobSpecsRef = &ref
		return storedPlan, nil, nil
	}

	storageID := primitive.NewObjectID()
	chunks := splitReleasePlanJobSpecPayload(plan.ID.Hex(), storageID, encoded.compressed, releasePlanJobSpecChunkSize)
	storedPlan.JobSpecsRef = &models.ReleasePlanJobSpecsRef{
		StorageID:      storageID,
		Encoding:       releasePlanJobSpecsEncodingGZIPBSONV1,
		ChunkCount:     int32(len(chunks)),
		OriginalSize:   encoded.originalSize,
		CompressedSize: int64(len(encoded.compressed)),
		SHA256:         encoded.sha256,
		ContentSHA256:  encoded.contentSHA256,
	}
	return storedPlan, chunks, nil
}

func buildReleasePlanJobSpecsPayload(plan *models.ReleasePlan) (*models.ReleasePlan, *releasePlanJobSpecsPayload, error) {
	if plan == nil {
		return nil, nil, errors.New("nil ReleasePlan")
	}
	storedPlan := *plan
	storedPlan.Jobs = make([]*models.ReleaseJob, len(plan.Jobs))
	if len(plan.Jobs) == 0 {
		storedPlan.JobSpecsRef = nil
		return &storedPlan, nil, nil
	}

	payload := &releasePlanJobSpecsPayload{Jobs: make([]*releasePlanJobSpecEntry, 0, len(plan.Jobs))}
	jobIDs := make(map[string]struct{}, len(plan.Jobs))
	for index, job := range plan.Jobs {
		if job == nil {
			return nil, nil, errors.New("nil release plan job")
		}
		if _, exists := jobIDs[job.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate release plan job ID: %s", job.ID)
		}
		jobIDs[job.ID] = struct{}{}
		storedJob := *job
		storedJob.Spec = nil
		storedPlan.Jobs[index] = &storedJob
		payload.Jobs = append(payload.Jobs, &releasePlanJobSpecEntry{JobID: job.ID, Spec: job.Spec})
	}
	return &storedPlan, payload, nil
}

func encodeReleasePlanJobSpecs(payload *releasePlanJobSpecsPayload) (*encodedReleasePlanJobSpecs, error) {
	bsonPayload, err := bson.MarshalWithRegistry(releasePlanJobSpecsBSONRegistry, payload)
	if err != nil {
		return nil, errors.Wrap(err, "marshal release plan job specs")
	}
	normalizedPayload, err := decodeReleasePlanJobSpecsPayload(bsonPayload)
	if err != nil {
		return nil, err
	}
	digestPayload, err := json.Marshal(normalizedPayload)
	if err != nil {
		return nil, errors.Wrap(err, "marshal release plan job specs digest")
	}
	contentDigest := sha256.Sum256(digestPayload)
	digest := sha256.Sum256(bsonPayload)

	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		return nil, errors.Wrap(err, "create release plan job specs gzip writer")
	}
	if _, err := writer.Write(bsonPayload); err != nil {
		return nil, errors.Wrap(err, "compress release plan job specs")
	}
	if err := writer.Close(); err != nil {
		return nil, errors.Wrap(err, "finish compressing release plan job specs")
	}

	return &encodedReleasePlanJobSpecs{
		compressed:    compressed.Bytes(),
		originalSize:  int64(len(bsonPayload)),
		sha256:        hex.EncodeToString(digest[:]),
		contentSHA256: hex.EncodeToString(contentDigest[:]),
	}, nil
}

func splitReleasePlanJobSpecPayload(planID string, storageID primitive.ObjectID, payload []byte, chunkSize int) []*models.ReleasePlanJobSpecChunk {
	chunkCount := (len(payload) + chunkSize - 1) / chunkSize
	chunks := make([]*models.ReleasePlanJobSpecChunk, 0, chunkCount)
	createdAt := time.Now().Unix()
	for offset := 0; offset < len(payload); offset += chunkSize {
		end := offset + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		chunks = append(chunks, &models.ReleasePlanJobSpecChunk{
			StorageID: storageID,
			PlanID:    planID,
			Sequence:  int32(len(chunks)),
			Data:      payload[offset:end],
			CreatedAt: createdAt,
		})
	}
	return chunks
}

func restoreReleasePlanJobSpecs(plan *models.ReleasePlan, chunks []*models.ReleasePlanJobSpecChunk) error {
	if plan == nil {
		return errors.New("nil ReleasePlan")
	}
	if plan.JobSpecsRef == nil {
		return nil
	}
	payload, err := decodeReleasePlanJobSpecChunks(plan.ID.Hex(), plan.JobSpecsRef, chunks)
	if err != nil {
		return err
	}
	return restoreReleasePlanJobSpecEntries(plan.Jobs, payload.Jobs)
}

func decodeReleasePlanJobSpecChunks(planID string, ref *models.ReleasePlanJobSpecsRef, chunks []*models.ReleasePlanJobSpecChunk) (*releasePlanJobSpecsPayload, error) {
	if ref.Encoding != releasePlanJobSpecsEncodingGZIPBSONV1 {
		return nil, fmt.Errorf("unsupported release plan job specs encoding: %s", ref.Encoding)
	}
	if ref.StorageID.IsZero() || ref.ChunkCount <= 0 || ref.OriginalSize < 0 || ref.OriginalSize == math.MaxInt64 || ref.CompressedSize < 0 {
		return nil, errors.New("invalid release plan job specs reference")
	}
	if len(chunks) != int(ref.ChunkCount) {
		return nil, errors.New("release plan job spec chunk count mismatch")
	}

	var compressed bytes.Buffer
	for index, chunk := range chunks {
		if chunk == nil || chunk.StorageID != ref.StorageID || chunk.PlanID != planID || chunk.Sequence != int32(index) {
			return nil, errors.New("release plan job spec chunk does not match reference")
		}
		compressed.Write(chunk.Data)
	}
	if int64(compressed.Len()) != ref.CompressedSize {
		return nil, errors.New("release plan job specs compressed size mismatch")
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed.Bytes()))
	if err != nil {
		return nil, errors.Wrap(err, "create release plan job specs gzip reader")
	}
	defer reader.Close()
	bsonPayload, err := io.ReadAll(io.LimitReader(reader, ref.OriginalSize+1))
	if err != nil {
		return nil, errors.Wrap(err, "decompress release plan job specs")
	}
	if int64(len(bsonPayload)) != ref.OriginalSize {
		return nil, errors.New("release plan job specs original size mismatch")
	}
	digest := sha256.Sum256(bsonPayload)
	if hex.EncodeToString(digest[:]) != ref.SHA256 {
		return nil, errors.New("release plan job specs checksum mismatch")
	}
	return decodeReleasePlanJobSpecsPayload(bsonPayload)
}

func restoreReleasePlanJobSpecEntries(jobs []*models.ReleaseJob, entries []*releasePlanJobSpecEntry) error {
	if len(entries) != len(jobs) {
		return errors.New("release plan job specs does not match release plan jobs")
	}

	specs := make(map[string]interface{}, len(entries))
	for _, entry := range entries {
		if entry == nil {
			return errors.New("nil release plan job spec entry")
		}
		if _, exists := specs[entry.JobID]; exists {
			return fmt.Errorf("duplicate release plan job spec: %s", entry.JobID)
		}
		specs[entry.JobID] = entry.Spec
	}

	resolvedSpecs := make([]interface{}, len(jobs))
	for index, job := range jobs {
		if job == nil {
			return errors.New("nil release plan job")
		}
		spec, exists := specs[job.ID]
		if !exists {
			return errors.New("release plan job specs does not match release plan jobs")
		}
		resolvedSpecs[index] = spec
		delete(specs, job.ID)
	}
	if len(specs) != 0 {
		return errors.New("release plan job specs does not match release plan jobs")
	}
	for index, job := range jobs {
		job.Spec = resolvedSpecs[index]
	}
	return nil
}

func decodeReleasePlanJobSpecsPayload(data []byte) (*releasePlanJobSpecsPayload, error) {
	payload := new(releasePlanJobSpecsPayload)
	decoder, err := bson.NewDecoder(bsonrw.NewBSONDocumentReader(data))
	if err != nil {
		return nil, errors.Wrap(err, "create release plan job specs BSON decoder")
	}
	if err := decoder.SetRegistry(releasePlanJobSpecsBSONRegistry); err != nil {
		return nil, errors.Wrap(err, "set release plan job specs BSON registry")
	}
	if err := decoder.Decode(payload); err != nil {
		return nil, errors.Wrap(err, "unmarshal release plan job specs")
	}
	return payload, nil
}
