package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type AIReviewFeedbackColl struct{ *mongo.Collection }

func (c *AIReviewFeedbackColl) Finish(ctx context.Context, pr *models.AIReviewFeedback, update bson.M) error {
	key := bson.M{"codehost_id": pr.CodehostID, "repo_owner": pr.RepoOwner, "repo_name": pr.RepoName, "pr": pr.PR, "lease_token": pr.LeaseToken, "revision": pr.Revision}
	update["lease_until"] = time.Time{}
	update["lease_token"] = ""
	result, err := c.UpdateOne(ctx, key, bson.M{"$set": update})
	if err != nil {
		return err
	}
	if result.MatchedCount > 0 {
		return nil
	}
	delete(key, "revision")
	_, err = c.UpdateOne(ctx, key, bson.M{"$set": bson.M{"lease_until": time.Time{}, "lease_token": ""}})
	return err
}

func NewAIReviewFeedbackColl() *AIReviewFeedbackColl {
	return &AIReviewFeedbackColl{mongotool.Database(config.MongoDatabase()).Collection(models.AIReviewFeedback{}.TableName())}
}

func (c *AIReviewFeedbackColl) GetCollectionName() string {
	return models.AIReviewFeedback{}.TableName()
}
func (c *AIReviewFeedbackColl) EnsureIndex(ctx context.Context) error {
	_, err := c.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "codehost_id", Value: 1}, {Key: "repo_owner", Value: 1}, {Key: "repo_name", Value: 1}, {Key: "pr", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "closed", Value: 1}, {Key: "next_sync_at", Value: 1}, {Key: "lease_until", Value: 1}}},
		{Keys: bson.D{{Key: "source_host", Value: 1}, {Key: "project_id", Value: 1}, {Key: "pr", Value: 1}, {Key: "comments.kind", Value: 1}, {Key: "comments.comment_id", Value: 1}}},
	}, mongotool.CreateIndexOptions(ctx))
	return err
}

// AddComments atomically appends only new kind/comment-ID pairs. Existing API
// snapshots and progress survive repeated registration.
func (c *AIReviewFeedbackColl) AddComments(ctx context.Context, key bson.M, comments []models.AIReviewFeedbackComment) error {
	for start := 0; start < len(comments); start += 100 {
		end := min(start+100, len(comments))
		writes := make([]mongo.WriteModel, 0, end-start)
		for _, comment := range comments[start:end] {
			filter := bson.M{}
			for field, value := range key {
				filter[field] = value
			}
			filter["comments"] = bson.M{"$not": bson.M{"$elemMatch": bson.M{"kind": comment.Kind, "comment_id": comment.CommentID}}}
			writes = append(writes, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(bson.M{
				"$push": bson.M{"comments": comment}, "$inc": bson.M{"revision": int64(1)},
			}))
		}
		_, err := c.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *AIReviewFeedbackColl) AddInlineThreads(ctx context.Context, key bson.M, threads []models.AIReviewInlineThread) error {
	for _, thread := range threads {
		if thread.CommentID <= 0 {
			continue
		}
		filter := bson.M{}
		for field, value := range key {
			filter[field] = value
		}
		filter["inline_threads.comment_id"] = bson.M{"$ne": thread.CommentID}
		if _, err := c.UpdateOne(ctx, filter, bson.M{"$push": bson.M{"inline_threads": thread}, "$inc": bson.M{"revision": int64(1)}}); err != nil {
			return err
		}
	}
	return nil
}
