package reviewfeedback

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	codehostmodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

func TestFeedbackRegistrationPartialFailureRemainsScheduled(t *testing.T) {
	uri := os.Getenv("AI_REVIEW_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set AI_REVIEW_TEST_MONGO_URI for local integration test")
	}
	log.Init(&log.Config{Level: "error"})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("ai_review_registration_test_%d", time.Now().UnixNano())
	oldDB := viper.Get(setting.ENVAslanDBName)
	viper.Set(setting.ENVAslanDBName, dbName)
	defer viper.Set(setting.ENVAslanDBName, oldDB)
	mongotool.Init(ctx, uri)
	db := mongotool.Database(dbName)
	defer db.Drop(context.Background())
	coll := repo.NewAIReviewFeedbackColl()
	// Reject thread appends while allowing the initial document, metadata and comments.
	validator := bson.M{"$jsonSchema": bson.M{"properties": bson.M{"inline_threads": bson.M{"bsonType": "array", "maxItems": 0}}}}
	if err := db.CreateCollection(ctx, coll.GetCollectionName(), options.CreateCollection().SetValidator(validator)); err != nil {
		t.Fatal(err)
	}
	for i, provider := range []string{setting.SourceFromGithub, setting.SourceFromGitlab} {
		id := i + 1
		if _, err := db.Collection(codehostmodels.CodeHost{}.TableName()).InsertOne(ctx, codehostmodels.CodeHost{ID: id, Type: provider, Address: "https://git.example"}); err != nil {
			t.Fatal(err)
		}
		comments := []PublishedComment{{Kind: "note", CommentID: 1}}
		threads := []models.AIReviewInlineThread{{CommentID: 1, ThreadID: "thread-1"}}
		if provider == setting.SourceFromGithub {
			comments = []PublishedComment{{Kind: "review", CommentID: 2, ReviewID: 2, InlineTotal: 1}}
			threads = []models.AIReviewInlineThread{{CommentID: 1, CommentNodeID: "comment-1", ReviewID: 2}}
		}
		if err := Register(ctx, "project", "title", id, 10, "org", "repo", 7, comments, threads); err == nil {
			t.Fatal("expected thread write failure")
		}
		var pr models.AIReviewFeedback
		if err := coll.FindOne(ctx, prKey(id, "org", "repo", 7)).Decode(&pr); err != nil {
			t.Fatal(err)
		}
		if len(pr.Comments) != 1 || len(pr.InlineThreads) != 0 || pr.Provider != provider || pr.Closed || pr.NextSyncAt.IsZero() || pr.NextSyncAt.After(time.Now()) || pr.FullSyncAt.IsZero() || pr.SourceHost != "git.example" || pr.ProjectID != 10 {
			t.Fatalf("partial registration lost scheduling metadata: %+v", pr)
		}
		filter := prKey(id, "org", "repo", 7)
		filter["closed"], filter["provider"] = false, provider
		filter["comments.0"] = bson.M{"$exists": true}
		filter["next_sync_at"] = bson.M{"$lte": time.Now()}
		if count, err := coll.CountDocuments(ctx, filter); err != nil || count != 1 {
			t.Fatalf("partial registration is not selectable: count=%d err=%v", count, err)
		}
	}
}
