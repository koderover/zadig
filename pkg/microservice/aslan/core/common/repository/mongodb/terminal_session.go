package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type TerminalSessionColl struct {
	*mongo.Collection

	coll string
}

type CloseSessionArgs struct {
	SessionID       string
	Status          models.TerminalSessionStatus
	EndedAt         int64
	DurationSeconds int64
	StorageID       string
	Bucket          string
	ObjectKey       string
	FileSize        int64
	ErrorMessage    string
}

func NewTerminalSessionColl() *TerminalSessionColl {
	name := models.TerminalSession{}.TableName()
	return &TerminalSessionColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *TerminalSessionColl) GetCollectionName() string {
	return c.coll
}

func (c *TerminalSessionColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "session_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "started_at", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "project_name", Value: 1}, {Key: "env_name", Value: 1}, {Key: "started_at", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "username", Value: 1}, {Key: "started_at", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "session_type", Value: 1}, {Key: "started_at", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "target_name", Value: 1}, {Key: "started_at", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, indexes, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *TerminalSessionColl) Create(session *models.TerminalSession) error {
	if session == nil {
		return fmt.Errorf("terminal session is nil")
	}
	now := time.Now().Unix()
	if session.CreatedAt == 0 {
		session.CreatedAt = now
	}
	if session.UpdatedAt == 0 {
		session.UpdatedAt = now
	}
	if session.LastActivityAt == 0 {
		session.LastActivityAt = session.StartedAt
	}
	_, err := c.InsertOne(context.TODO(), session)
	return err
}

func (c *TerminalSessionColl) FindBySessionID(sessionID string) (*models.TerminalSession, error) {
	resp := new(models.TerminalSession)
	err := c.FindOne(context.TODO(), bson.M{"session_id": sessionID}).Decode(resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *TerminalSessionColl) UpdateActivity(sessionID string, commandCountDelta int64, lastActivityAt int64) error {
	update := bson.M{
		"$set": bson.M{
			"last_activity_at": lastActivityAt,
			"updated_at":       time.Now().Unix(),
		},
	}
	if commandCountDelta != 0 {
		update["$inc"] = bson.M{"command_count": commandCountDelta}
	}
	_, err := c.UpdateOne(context.TODO(), bson.M{"session_id": sessionID}, update)
	return err
}

func (c *TerminalSessionColl) CloseSession(args *CloseSessionArgs) error {
	if args == nil {
		return fmt.Errorf("close terminal session arguments are nil")
	}
	update := bson.M{
		"$set": bson.M{
			"status":           args.Status,
			"ended_at":         args.EndedAt,
			"duration_seconds": args.DurationSeconds,
			"last_activity_at": args.EndedAt,
			"storage_id":       args.StorageID,
			"bucket":           args.Bucket,
			"object_key":       args.ObjectKey,
			"file_size":        args.FileSize,
			"error_message":    args.ErrorMessage,
			"updated_at":       time.Now().Unix(),
		},
	}
	_, err := c.UpdateOne(context.TODO(), bson.M{"session_id": args.SessionID}, update)
	return err
}

func (c *TerminalSessionColl) List(args *models.TerminalSessionListArgs) ([]*models.TerminalSession, int64, error) {
	resp := make([]*models.TerminalSession, 0)
	query := bson.M{}
	if args != nil {
		if args.Status != "" {
			query["status"] = args.Status
		}
		if args.SessionType != "" {
			query["session_type"] = args.SessionType
		}
		if args.ProjectName != "" {
			query["project_name"] = buildRegexQuery(args.ProjectName)
		}
		if args.EnvName != "" {
			query["env_name"] = buildRegexQuery(args.EnvName)
		}
		if args.ServiceName != "" {
			query["service_name"] = buildRegexQuery(args.ServiceName)
		}
		if args.Username != "" {
			query["username"] = buildRegexQuery(args.Username)
		}
		if args.TargetName != "" {
			query["target_name"] = buildRegexQuery(args.TargetName)
		}
		if args.RemoteAddr != "" {
			query["remote_addr"] = buildRegexQuery(args.RemoteAddr)
		}
		if args.StartTime > 0 || args.EndTime > 0 {
			timeQuery := bson.M{}
			if args.StartTime > 0 {
				timeQuery["$gte"] = args.StartTime
			}
			if args.EndTime > 0 {
				timeQuery["$lte"] = args.EndTime
			}
			query["started_at"] = timeQuery
		}
	}

	opts := options.Find().SetSort(bson.D{{Key: "started_at", Value: -1}})
	if args != nil && args.PageNum > 0 && args.PageSize > 0 {
		opts.SetSkip((args.PageNum - 1) * args.PageSize).SetLimit(args.PageSize)
	}
	cursor, err := c.Find(context.TODO(), query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.TODO())

	if err := cursor.All(context.TODO(), &resp); err != nil {
		return nil, 0, err
	}
	total, err := c.CountDocuments(context.TODO(), query)
	return resp, total, err
}
