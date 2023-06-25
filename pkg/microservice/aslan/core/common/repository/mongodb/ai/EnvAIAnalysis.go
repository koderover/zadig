package ai

import (
	"context"
	"errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/ai"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EnvAIAnalysisColl struct {
	*mongo.Collection

	coll string
}

func NewEnvAIAnalysisColl() *EnvAIAnalysisColl {
	name := ai.EnvAIAnalysis{}.TableName()
	return &EnvAIAnalysisColl{Collection: mongotool.Database(config.MongoDatabase()).Collection(name), coll: name}
}

func (c *EnvAIAnalysisColl) GetCollectionName() string {
	return c.coll
}

func (c *EnvAIAnalysisColl) EnsureIndex(ctx context.Context) error {
	mod := []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "project_name", Value: 1},
				bson.E{Key: "env_name", Value: 1},
				bson.E{Key: "production", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := c.Indexes().CreateMany(ctx, mod)

	return err
}

type EnvAIAnalysisListOption struct {
	ProjectName string
	EnvName     string
	Production  bool
	PageNum     int64
	PageSize    int64
}

func (c *EnvAIAnalysisColl) ListByOptions(opts EnvAIAnalysisListOption) ([]*ai.EnvAIAnalysis, error) {
	query := bson.M{}
	if opts.ProjectName != "" {
		query["project_name"] = opts.ProjectName
	}
	if opts.EnvName != "" {
		query["env_name"] = opts.EnvName
	}
	if opts.Production {
		query["production"] = opts.Production
	}

	if opts.PageNum == 0 {
		opts.PageNum = 1
	}
	if opts.PageSize == 0 {
		opts.PageSize = 10
	}

	var resp []*ai.EnvAIAnalysis
	opt := options.Find().
		SetSkip(int64((opts.PageNum - 1) * opts.PageSize)).
		SetLimit(int64(opts.PageSize)).
		SetSort(bson.D{{Key: "create_time", Value: -1}})

	cursor, err := c.Collection.Find(context.TODO(), query, opt)
	if err != nil {
		return nil, err
	}
	err = cursor.All(context.TODO(), &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *EnvAIAnalysisColl) Create(args *ai.EnvAIAnalysis) error {
	if args == nil {
		return errors.New("nil Workflow args")
	}

	_, err := c.InsertOne(context.TODO(), args)
	return err
}
