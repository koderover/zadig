package reviewfeedback

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
)

// ApplyGitLabReaction is called only after the webhook token has been checked.
// Counts and reaction state are committed together in the MR document.
func ApplyGitLabReaction(ctx context.Context, sourceHost string, projectID, number int, noteID, emojiID int64, name, action string, log *zap.SugaredLogger) (err error) {
	start := time.Now()
	defer func() {
		if err != nil {
			log.Errorw("failed to process gitlab emoji webhook", "error", err, "duration", time.Since(start))
		}
	}()
	if number <= 0 || projectID <= 0 || noteID <= 0 || emojiID <= 0 || (name != "thumbsup" && name != "thumbsdown") || (action != "award" && action != "revoke") {
		log.Infow("ignored gitlab emoji webhook", "reason", "invalid reaction")
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	key := bson.M{"source_host": sourceHost, "project_id": projectID, "pr": number, "provider": setting.SourceFromGitlab, "closed": false,
		"comments": bson.M{"$elemMatch": bson.M{"kind": "note", "comment_id": noteID}}}
	coll := repo.NewAIReviewFeedbackColl()
	for ctx.Err() == nil {
		var pr models.AIReviewFeedback
		if err = coll.FindOne(ctx, key).Decode(&pr); errors.Is(err, mongo.ErrNoDocuments) {
			log.Infow("ignored gitlab emoji webhook", "reason", "AI comment is not registered or MR is frozen/missing")
			return nil
		} else if err != nil {
			return err
		}
		log = log.With("codehost_id", pr.CodehostID, "repo_owner", pr.RepoOwner, "repo_name", pr.RepoName, "pr", pr.PR)
		if !applyGitLabReaction(&pr, emojiID, name, action) {
			log.Infow("ignored gitlab emoji webhook", "reason", "duplicate or obsolete event", "up", pr.Up, "down", pr.Down)
			return nil
		}
		filter := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
		filter["provider"], filter["closed"], filter["revision"] = setting.SourceFromGitlab, false, pr.Revision
		now := time.Now()
		result, err := coll.UpdateOne(ctx, filter, bson.M{
			"$set": bson.M{"up": pr.Up, "down": pr.Down, "gitlab_reactions." + strconv.FormatInt(emojiID, 10): action, "synced_at": now, "updated_at": now, "cycle_started_at": pr.CycleStartedAt},
			"$inc": bson.M{"revision": int64(1)},
		})
		if err != nil {
			return err
		}
		if result.MatchedCount > 0 {
			log.Infow("updated gitlab AI review feedback", "up", pr.Up, "down", pr.Down, "duration", time.Since(start))
			return nil
		}
		// Another event changed the MR; reload before applying this event.
	}
	return ctx.Err()
}

func applyGitLabReaction(pr *models.AIReviewFeedback, emojiID int64, name, action string) bool {
	id := strconv.FormatInt(emojiID, 10)
	previous := pr.GitLabReactions[id]
	if previous == action || previous == "revoke" {
		return false
	}
	count := &pr.Up
	if name == "thumbsdown" {
		count = &pr.Down
	}
	if action == "award" {
		*count++
	} else if previous == "award" && *count > 0 {
		*count--
	}
	if pr.GitLabReactions == nil {
		pr.GitLabReactions = make(map[string]string)
	}
	pr.GitLabReactions[id] = action
	// A concurrent final reconciliation must re-read comments already collected.
	pr.CycleStartedAt = time.Time{}
	return true
}

// Keep revoked IDs as tombstones so delayed awards cannot undo reconciliation.
func reconciledGitLabReactions(pr *models.AIReviewFeedback) map[string]string {
	states := make(map[string]string, len(pr.GitLabReactions))
	for id := range pr.GitLabReactions {
		states[id] = "revoke"
	}
	for _, comment := range pr.Comments {
		for _, id := range comment.ReactionIDs {
			states[strconv.FormatInt(id, 10)] = "award"
		}
	}
	return states
}
