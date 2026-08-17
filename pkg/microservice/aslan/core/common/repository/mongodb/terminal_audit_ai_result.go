package mongodb

import (
	"context"
	"errors"
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

func NewTerminalAuditAIResultColl() *TerminalAuditAIResultColl {
	name := models.TerminalAuditAIResult{}.TableName()
	return &TerminalAuditAIResultColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *TerminalAuditAIResultColl) GetCollectionName() string {
	return c.coll
}

func (c *TerminalAuditAIResultColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "session_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, indexes, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *TerminalAuditAIResultColl) Upsert(result *models.TerminalAuditAIResult) error {
	if result == nil {
		return errors.New("terminal audit ai result is nil")
	}
	if result.SessionID == "" {
		return errors.New("terminal audit ai result session id is empty")
	}

	now := time.Now().Unix()
	if result.CreatedAt == 0 {
		result.CreatedAt = now
	}
	result.UpdatedAt = now

	update := bson.M{
		"$set": bson.M{
			"status":        result.Status,
			"risk_level":    result.RiskLevel,
			"summary":       result.Summary,
			"findings":      result.Findings,
			"coverage":      result.Coverage,
			"prompt":        result.Prompt,
			"answer":        result.Answer,
			"token_num":     result.TokenNum,
			"error_message": result.ErrorMessage,
			"updated_at":    result.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"session_id": result.SessionID,
			"created_at": result.CreatedAt,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalAuditMongoTimeout)
	defer cancel()
	_, err := c.UpdateOne(ctx, bson.M{"session_id": result.SessionID}, update, options.Update().SetUpsert(true))
	return err
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
