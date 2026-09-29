package reviewfeedback

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/xanzy/go-gitlab"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/time/rate"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	gitlabtool "github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
)

const leaseDuration = 5 * time.Minute

type PublishedComment struct {
	Kind        string
	CommentID   int64
	ReviewID    int64
	InlineTotal int
}

func Host(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Host)
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
		host = strings.ToLower(u.Hostname())
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	}
	return host
}

func prKey(codehostID int, owner, name string, pr int) bson.M {
	return bson.M{"codehost_id": codehostID, "repo_owner": owner, "repo_name": name, "pr": pr}
}

// Register records only comments that were actually published by Zadig.
func Register(ctx context.Context, projectName, title string, codehostID, projectID int, owner, name string, pr int, comments []PublishedComment, threads []models.AIReviewInlineThread) error {
	if len(comments) == 0 {
		return nil
	}
	host, err := systemconfig.New().GetCodeHost(codehostID, ctx)
	if err != nil {
		return err
	}
	provider := strings.ToLower(host.Type)
	now := time.Now()
	sourceHost := Host(host.Address)
	if provider == setting.SourceFromGithub && (sourceHost == "" || sourceHost == "api.github.com") {
		sourceHost = "github.com"
	}
	key := prKey(codehostID, owner, name, pr)
	coll := repo.NewAIReviewFeedbackColl()
	initial := prKey(codehostID, owner, name, pr)
	initial["up"], initial["down"], initial["comments"] = 0, 0, bson.A{}
	initial["inline_threads"], initial["inline_total"], initial["inline_resolved"], initial["pr_title"] = bson.A{}, 0, 0, ""
	if _, err = coll.UpdateOne(ctx, key, bson.M{"$setOnInsert": initial}, options.Update().SetUpsert(true)); err != nil {
		return err
	}
	targets := make([]models.AIReviewFeedbackComment, 0, len(comments))
	for _, comment := range comments {
		if comment.CommentID > 0 {
			targets = append(targets, models.AIReviewFeedbackComment{Kind: comment.Kind, CommentID: comment.CommentID, ReviewID: comment.ReviewID, InlineTotal: comment.InlineTotal, DirtyAt: now})
		}
	}
	if err = coll.AddComments(ctx, key, targets); err != nil {
		return err
	}
	if err = coll.AddInlineThreads(ctx, key, threads); err != nil {
		return err
	}
	update := bson.M{
		"$set": bson.M{"project_name": projectName, "provider": provider, "source_host": sourceHost, "closed": false, "updated_at": now},
		"$inc": bson.M{"revision": int64(1)},
	}
	if projectID > 0 {
		update["$set"].(bson.M)["project_id"] = projectID
	}
	if title != "" {
		update["$set"].(bson.M)["pr_title"] = title
	}
	if provider == setting.SourceFromGithub || provider == setting.SourceFromGitlab {
		update["$min"] = bson.M{"next_sync_at": now, "full_sync_at": now}
	}
	_, err = coll.UpdateOne(ctx, key, update)
	if err != nil {
		return err
	}
	return refreshRegisteredInlineTotals(ctx, key)
}

// SchedulePR schedules final reconciliation on close/merge, or resumes feedback on reopen.
// Callers must validate the provider's webhook secret before calling this.
func SchedulePR(ctx context.Context, provider, sourceHost, owner, name string, number int, action string) error {
	if sourceHost == "" || number <= 0 {
		return nil
	}
	now := time.Now()
	if provider == setting.SourceFromGitlab {
		closing := action == "close" || action == "merge"
		update := bson.M{"$set": bson.M{"closed": false, "final_sync": closing, "updated_at": now, "cycle_started_at": time.Time{}}, "$inc": bson.M{"revision": int64(1)}}
		update["$min"] = bson.M{"next_sync_at": now, "full_sync_at": now}
		_, err := repo.NewAIReviewFeedbackColl().UpdateMany(ctx, bson.M{"provider": provider, "source_host": sourceHost, "repo_owner": owner, "repo_name": name, "pr": number}, update)
		return err
	}
	_, err := repo.NewAIReviewFeedbackColl().UpdateMany(ctx, bson.M{"provider": provider, "source_host": sourceHost, "repo_owner": owner, "repo_name": name, "pr": number}, bson.M{
		"$set": bson.M{"closed": false, "updated_at": now, "cycle_started_at": time.Time{}}, "$min": bson.M{"next_sync_at": now, "full_sync_at": now}, "$inc": bson.M{"revision": int64(1)},
	})
	return err
}

var workerMu sync.Mutex

// SyncDue bounds each invocation. Leases prevent duplicate work across replicas.
func SyncDue(ctx context.Context, limit int) error {
	if !workerMu.TryLock() {
		return nil
	}
	defer workerMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	coll := repo.NewAIReviewFeedbackColl()
	var firstErr error
	for i := 0; i < limit && ctx.Err() == nil; i++ {
		now := time.Now()
		var pr models.AIReviewFeedback
		err := coll.FindOneAndUpdate(ctx, bson.M{"closed": false, "comments.0": bson.M{"$exists": true}, "next_sync_at": bson.M{"$lte": now}, "provider": bson.M{"$in": bson.A{setting.SourceFromGithub, setting.SourceFromGitlab}}, "$or": bson.A{bson.M{"lease_until": bson.M{"$lte": now}}, bson.M{"lease_until": bson.M{"$exists": false}}}},
			bson.M{"$set": bson.M{"lease_until": now.Add(leaseDuration), "lease_token": primitive.NewObjectID().Hex()}}, options.FindOneAndUpdate().SetSort(bson.D{{Key: "next_sync_at", Value: 1}}).SetReturnDocument(options.After)).Decode(&pr)
		if errors.Is(err, mongo.ErrNoDocuments) {
			break
		}
		if err != nil {
			return err
		}
		closed, syncErr := syncPR(ctx, &pr)
		now = time.Now()
		update := syncResultUpdate(&pr, now, closed, syncErr)
		if syncErr != nil && firstErr == nil {
			firstErr = syncErr
		}
		// A canceled HTTP request must still release its lease; use a short independent DB context.
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = coll.Finish(finishCtx, &pr, update)
		finishCancel()
		if err != nil {
			return err
		}
	}
	return firstErr
}

func syncPR(ctx context.Context, pr *models.AIReviewFeedback) (bool, error) {
	host, err := systemconfig.New().GetCodeHost(pr.CodehostID, ctx)
	if err != nil {
		return false, err
	}
	var read func(models.AIReviewFeedbackComment) (reactionCount, error)
	var readResolutions func() ([]models.AIReviewInlineThread, error)
	closed := false
	switch pr.Provider {
	case setting.SourceFromGithub:
		cli, err := newGitHubFeedbackClient(ctx, pr, host)
		if err != nil {
			return false, err
		}
		pull, _, err := cli.PullRequests.Get(ctx, pr.RepoOwner, pr.RepoName, pr.PR)
		if err != nil {
			discardGitHubFeedbackClient(cli)
			return false, err
		}
		if pull.GetState() != "open" && pull.GetState() != "closed" {
			return false, fmt.Errorf("GitHub PR response has invalid state %q", pull.GetState())
		}
		closed = pull.GetState() == "closed"
		pr.PRTitle = pull.GetTitle()
		readResolutions = func() ([]models.AIReviewInlineThread, error) { return readGitHubResolutions(ctx, cli, pr) }
		read = func(target models.AIReviewFeedbackComment) (reactionCount, error) {
			count, err := readGitHubTarget(ctx, cli, pr, target)
			if err != nil {
				discardGitHubFeedbackClient(cli)
			}
			return count, err
		}
	case setting.SourceFromGitlab:
		cli, err := gitlabtool.NewClientWithContext(ctx, host.ID, host.Address, host.AccessToken, config.ProxyHTTPSAddr(), host.EnableProxy, host.DisableSSL, gitlab.WithoutRetries(), gitlab.WithCustomLimiter(rate.NewLimiter(rate.Inf, 1)))
		if err != nil {
			return false, err
		}
		defer cli.CloseIdleConnections()
		project := strings.TrimLeft(pr.RepoOwner+"/"+pr.RepoName, "/")
		mr, _, err := cli.MergeRequests.GetMergeRequest(project, pr.PR, nil, gitlab.WithContext(ctx))
		if err != nil {
			return false, err
		}
		if mr.ProjectID <= 0 || (mr.State != "opened" && mr.State != "closed" && mr.State != "merged" && mr.State != "locked") {
			return false, fmt.Errorf("GitLab MR response has invalid project/state")
		}
		closed = mr.State == "closed" || mr.State == "merged"
		pr.PRTitle = mr.Title
		readResolutions = func() ([]models.AIReviewInlineThread, error) { return readGitLabResolutions(ctx, cli.Client, pr) }
		if pr.ProjectID != mr.ProjectID {
			key := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
			key["lease_token"] = pr.LeaseToken
			key["revision"] = pr.Revision
			if _, err = repo.NewAIReviewFeedbackColl().UpdateOne(ctx, key, bson.M{"$set": bson.M{"project_id": mr.ProjectID}}); err != nil {
				return false, err
			}
		}
		pr.ProjectID = mr.ProjectID
		read = func(target models.AIReviewFeedbackComment) (reactionCount, error) {
			return readGitLabTarget(ctx, cli.Client, pr, target)
		}
	default:
		return false, fmt.Errorf("unsupported AI review feedback provider %q", pr.Provider)
	}
	// Metadata uses the same revision fence as the collected snapshots.
	metadataKey := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
	metadataKey["lease_token"], metadataKey["revision"] = pr.LeaseToken, pr.Revision
	metadata := bson.M{}
	if pr.PRTitle != "" {
		metadata["pr_title"] = pr.PRTitle
	}
	// A missed close webhook must still enter final reconciliation and retain
	// its retry schedule if an API request fails later in this cycle.
	if pr.Provider == setting.SourceFromGitlab && closed {
		pr.FinalSync = true
		metadata["final_sync"] = true
	}
	if len(metadata) > 0 {
		result, err := repo.NewAIReviewFeedbackColl().UpdateOne(ctx, metadataKey, bson.M{"$set": metadata})
		if err != nil {
			return false, err
		}
		if result.MatchedCount == 0 {
			return false, fmt.Errorf("AI review feedback changed during synchronization")
		}
	}
	threads, err := readResolutions()
	if err != nil {
		return false, err
	}
	// A persisted cycle lets large PRs resume without re-fetching completed targets.
	if pr.CycleStartedAt.IsZero() && (closed || !pr.FullSyncAt.After(time.Now())) {
		pr.CycleStartedAt = time.Now()
		key := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
		key["lease_token"] = pr.LeaseToken
		key["revision"] = pr.Revision
		_, err = repo.NewAIReviewFeedbackColl().UpdateOne(ctx, key, bson.M{"$set": bson.M{"cycle_started_at": pr.CycleStartedAt}})
		if err != nil {
			return false, err
		}
	}
	coll := repo.NewAIReviewFeedbackColl()
	for i, target := range pr.Comments {
		if pr.Provider == setting.SourceFromGitlab && !closed && !pr.FinalSync {
			continue
		}
		if !needsRefresh(target, pr.CycleStartedAt) {
			continue
		}
		started := time.Now()
		count, err := read(target)
		if err != nil {
			return false, err
		}
		key := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
		key["lease_token"], key["revision"] = pr.LeaseToken, pr.Revision
		key["comments"] = bson.M{"$elemMatch": bson.M{"kind": target.Kind, "comment_id": target.CommentID}}
		result, err := coll.UpdateOne(ctx, key, bson.M{"$set": bson.M{"comments.$.up": count.Up, "comments.$.down": count.Down, "comments.$.synced_at": started}})
		if err != nil {
			return false, err
		}
		if result.MatchedCount == 0 {
			return false, fmt.Errorf("AI review feedback changed during synchronization")
		}
		pr.Comments[i].Up, pr.Comments[i].Down, pr.Comments[i].SyncedAt = count.Up, count.Down, started
	}
	// Only registered AI targets contribute to this PR's aggregate.
	total := feedbackTotals(pr.Comments)
	key := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
	key["lease_token"], key["revision"] = pr.LeaseToken, pr.Revision
	pr.InlineThreads = threads
	inlineTotal, inlineResolved := inlineTotals(pr)
	snapshot := bson.M{"inline_threads": threads, "inline_total": inlineTotal, "inline_resolved": inlineResolved, "resolution_synced_at": time.Now(), "updated_at": time.Now()}
	if pr.Provider != setting.SourceFromGitlab || closed || pr.FinalSync {
		snapshot["up"], snapshot["down"] = total.Up, total.Down
	}
	result, err := coll.UpdateOne(ctx, key, bson.M{"$set": snapshot, "$inc": bson.M{"revision": int64(1)}})
	if err == nil && result.MatchedCount == 0 {
		err = fmt.Errorf("AI review feedback changed during synchronization")
	}
	if err == nil {
		pr.Revision++
	}
	return closed, err
}

// Failed attempts retain the existing snapshots and use the normal polling interval.
func syncResultUpdate(pr *models.AIReviewFeedback, now time.Time, closed bool, syncErr error) bson.M {
	if pr.Provider == setting.SourceFromGitlab && pr.FinalSync && syncErr != nil {
		return bson.M{"next_sync_at": now.Add(time.Minute), "last_error": syncErr.Error()}
	}
	interval := config.AIReviewGitPollInterval()
	if syncErr != nil {
		return bson.M{"next_sync_at": now.Add(interval), "last_error": syncErr.Error()}
	}
	nextFull := now.Add(interval)
	return bson.M{"next_sync_at": nextFull, "full_sync_at": nextFull, "cycle_started_at": time.Time{}, "synced_at": now, "closed": closed, "final_sync": false, "last_error": ""}
}

func feedbackTotals(comments []models.AIReviewFeedbackComment) reactionCount {
	total := reactionCount{}
	for _, comment := range comments {
		total.Up += comment.Up
		total.Down += comment.Down
	}
	return total
}
