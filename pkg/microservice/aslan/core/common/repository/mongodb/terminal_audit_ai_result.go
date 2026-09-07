package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type TerminalAuditAIResultColl struct {
	*mongo.Collection

	coll string
}

var ErrTerminalAuditAIAlreadyRunning = errors.New("terminal audit ai analysis is already running")

func NewTerminalAuditAIResultColl() *TerminalAuditAIResultColl {
	name := models.TerminalAuditAIResult{}.TableName()
	return &TerminalAuditAIResultColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *TerminalAuditAIResultColl) GetCollectionName() string { return c.coll }

func (c *TerminalAuditAIResultColl) EnsureIndex(ctx context.Context) error {
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: "session_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := c.Indexes().CreateOne(ctx, index, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *TerminalAuditAIResultColl) TryStart(sessionID, runID string, startedAt, leaseExpiresAt int64) (*models.TerminalAuditAIResult, error) {
	filter := bson.M{
		"session_id": sessionID,
		"$or": bson.A{
			bson.M{"status": bson.M{"$ne": models.TerminalAuditAIStatusRunning}},
			bson.M{"lease_expires_at": bson.M{"$lte": startedAt}},
			bson.M{"lease_expires_at": bson.M{"$exists": false}},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"status":                 models.TerminalAuditAIStatusRunning,
			"risk_level":             "",
			"summary":                "",
			"findings":               []models.TerminalAuditAIFinding{},
			"coverage":               "",
			"model":                  "",
			"token_num":              0,
			"analyzed_command_count": 0,
			"total_command_count":    0,
			"error_message":          "",
			"run_id":                 runID,
			"lease_expires_at":       leaseExpiresAt,
			"started_at":             startedAt,
			"finished_at":            0,
			"updated_at":             startedAt,
		},
		"$setOnInsert": bson.M{
			"session_id": sessionID,
			"created_at": startedAt,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	ctx, cancel := context.WithTimeout(context.Background(), terminalAuditMongoTimeout)
	defer cancel()
	result := new(models.TerminalAuditAIResult)
	// A running session with a valid lease does not match the filter, so the upsert
	// attempts an insert and hits the unique session_id index established above.
	err := c.FindOneAndUpdate(ctx, filter, update, opts).Decode(result)
	if mongo.IsDuplicateKeyError(err) {
		return nil, ErrTerminalAuditAIAlreadyRunning
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *TerminalAuditAIResultColl) UpdateLease(sessionID, runID string, leaseExpiresAt int64) error {
	now := time.Now().Unix()
	ctx, cancel := context.WithTimeout(context.Background(), terminalAuditMongoTimeout)
	defer cancel()
	result, err := c.UpdateOne(ctx, bson.M{
		"session_id": sessionID,
		"run_id":     runID,
		"status":     models.TerminalAuditAIStatusRunning,
	}, bson.M{
		"$max": bson.M{"lease_expires_at": leaseExpiresAt},
		"$set": bson.M{"updated_at": now},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("terminal audit ai run %s no longer owns session %s", runID, sessionID)
	}
	return nil
}

func (c *TerminalAuditAIResultColl) Finish(result *models.TerminalAuditAIResult) error {
	now := time.Now().Unix()
	result.UpdatedAt = now
	result.FinishedAt = now
	update := bson.M{"$set": bson.M{
		"status":                 result.Status,
		"risk_level":             result.RiskLevel,
		"summary":                result.Summary,
		"findings":               result.Findings,
		"coverage":               result.Coverage,
		"model":                  result.Model,
		"token_num":              result.TokenNum,
		"analyzed_command_count": result.AnalyzedCommandCount,
		"total_command_count":    result.TotalCommandCount,
		"error_message":          result.ErrorMessage,
		"lease_expires_at":       0,
		"finished_at":            result.FinishedAt,
		"updated_at":             result.UpdatedAt,
	}}
	ctx, cancel := context.WithTimeout(context.Background(), terminalAuditMongoTimeout)
	defer cancel()
	writeResult, err := c.UpdateOne(ctx, bson.M{"session_id": result.SessionID, "run_id": result.RunID}, update)
	if err != nil {
		return err
	}
	if writeResult.MatchedCount == 0 {
		return fmt.Errorf("terminal audit ai run %s no longer owns session %s", result.RunID, result.SessionID)
	}
	return nil
}

func (c *TerminalAuditAIResultColl) FindBySessionID(sessionID string) (*models.TerminalAuditAIResult, error) {
	resp := new(models.TerminalAuditAIResult)
	ctx, cancel := context.WithTimeout(context.Background(), terminalAuditMongoTimeout)
	defer cancel()
	err := c.FindOne(ctx, bson.M{"session_id": sessionID}).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
