package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type AIReviewStatColl struct{ *mongo.Collection }

func NewAIReviewStatColl() *AIReviewStatColl {
	return &AIReviewStatColl{mongotool.Database(config.MongoDatabase()).Collection(models.AIReviewStat{}.TableName())}
}

func (c *AIReviewStatColl) GetCollectionName() string { return models.AIReviewStat{}.TableName() }

func (c *AIReviewStatColl) EnsureIndex(ctx context.Context) error {
	_, err := c.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "project_name", Value: 1}, {Key: "codehost_id", Value: 1}, {Key: "repo_owner", Value: 1}, {Key: "repo_name", Value: 1}, {Key: "workflow_name", Value: 1}, {Key: "task_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "reviewed_at", Value: 1}, {Key: "project_name", Value: 1}}},
		{Keys: bson.D{{Key: "codehost_id", Value: 1}, {Key: "repo_owner", Value: 1}, {Key: "repo_name", Value: 1}, {Key: "reviewed_at", Value: -1}}},
	}, mongotool.CreateIndexOptions(ctx))
	return err
}

func (c *AIReviewStatColl) Upsert(ctx context.Context, stat *models.AIReviewStat) error {
	key := bson.M{"project_name": stat.ProjectName, "codehost_id": stat.CodehostID, "repo_owner": stat.RepoOwner, "repo_name": stat.RepoName, "workflow_name": stat.WorkflowName, "task_id": stat.TaskID}
	_, err := c.UpdateOne(ctx, key, bson.M{"$set": stat}, options.Update().SetUpsert(true))
	return err
}
