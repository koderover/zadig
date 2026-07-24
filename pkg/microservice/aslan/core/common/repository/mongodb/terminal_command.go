package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type TerminalCommandColl struct {
	*mongo.Collection

	coll string
}

func NewTerminalCommandColl() *TerminalCommandColl {
	name := models.TerminalCommand{}.TableName()
	return &TerminalCommandColl{
		Collection: mongotool.Database(config.MongoDatabase()).Collection(name),
		coll:       name,
	}
}

func (c *TerminalCommandColl) GetCollectionName() string {
	return c.coll
}

func (c *TerminalCommandColl) EnsureIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "session_id", Value: 1}, {Key: "seq", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "project_name", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys:    bson.D{{Key: "username", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetUnique(false),
		},
	}
	_, err := c.Indexes().CreateMany(ctx, indexes, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *TerminalCommandColl) CreateMany(commands []*models.TerminalCommand) error {
	if len(commands) == 0 {
		return nil
	}
	docs := make([]interface{}, 0, len(commands))
	for _, command := range commands {
		if command == nil {
			return fmt.Errorf("terminal command is nil")
		}
		docs = append(docs, command)
	}
	_, err := c.InsertMany(context.TODO(), docs)
	return err
}

func (c *TerminalCommandColl) List(args *models.TerminalCommandListArgs) ([]*models.TerminalCommand, int64, error) {
	resp := make([]*models.TerminalCommand, 0)
	query := bson.M{}
	if args != nil {
		if args.SessionID != "" {
			query["session_id"] = args.SessionID
		}
		if args.ProjectName != "" {
			query["project_name"] = buildRegexQuery(args.ProjectName)
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
		if args.Command != "" {
			query["command"] = buildRegexQuery(args.Command)
		}
		if args.StartTime > 0 || args.EndTime > 0 {
			timeQuery := bson.M{}
			if args.StartTime > 0 {
				timeQuery["$gte"] = args.StartTime
			}
			if args.EndTime > 0 {
				timeQuery["$lte"] = args.EndTime
			}
			query["created_at"] = timeQuery
		}
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "seq", Value: -1}})
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
