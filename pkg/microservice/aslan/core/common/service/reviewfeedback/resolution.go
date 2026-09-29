package reviewfeedback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"net/http"
	"strings"
	"time"

	githubapi "github.com/google/go-github/v35/github"
	"github.com/xanzy/go-gitlab"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
)

// ReadGitHubInlineThreads recovers the original comments of one published review.
func ReadGitHubInlineThreads(ctx context.Context, cli *githubapi.Client, owner, name string, number int, reviewID int64) ([]models.AIReviewInlineThread, error) {
	threads := make([]models.AIReviewInlineThread, 0)
	opts := &githubapi.ListOptions{PerPage: 100}
	for {
		items, resp, err := cli.PullRequests.ListReviewComments(ctx, owner, name, number, reviewID, opts)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item == nil || item.GetInReplyTo() != 0 || !strings.Contains(item.GetBody(), "<!-- zadig-ai-review -->") {
				continue
			}
			if item.GetID() <= 0 || item.GetNodeID() == "" {
				return nil, fmt.Errorf("GitHub inline comment is missing ID/node ID")
			}
			threads = append(threads, models.AIReviewInlineThread{CommentID: item.GetID(), CommentNodeID: item.GetNodeID(), ReviewID: reviewID})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return threads, nil
}

func inlineTotals(pr *models.AIReviewFeedback) (total, resolved int) {
	reviews := map[int64]int{}
	for _, thread := range pr.InlineThreads {
		if thread.ReviewID > 0 {
			reviews[thread.ReviewID]++
		} else {
			total++
		}
		if thread.Resolved && !thread.Deleted {
			resolved++
		}
	}
	for _, target := range pr.Comments {
		if target.Kind == "review" {
			reviews[target.ReviewID] = max(reviews[target.ReviewID], target.InlineTotal)
		}
	}
	for _, count := range reviews {
		total += count
	}
	return
}

func refreshRegisteredInlineTotals(ctx context.Context, key bson.M) error {
	coll := repo.NewAIReviewFeedbackColl()
	for ctx.Err() == nil {
		var pr models.AIReviewFeedback
		if err := coll.FindOne(ctx, key).Decode(&pr); err != nil {
			return err
		}
		total, resolved := inlineTotals(&pr)
		filter := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
		filter["revision"] = pr.Revision
		result, err := coll.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"inline_total": total, "inline_resolved": resolved}, "$inc": bson.M{"revision": int64(1)}})
		if err != nil {
			return err
		}
		if result.MatchedCount > 0 {
			return nil
		}
	}
	return ctx.Err()
}

func queryGitHubThreads(ctx context.Context, cli *githubapi.Client, query string, variables map[string]interface{}, result interface{}) error {
	endpoint := *cli.BaseURL
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/") + "/graphql"
	if strings.HasSuffix(cli.BaseURL.Path, "/api/v3/") {
		endpoint.Path = strings.TrimSuffix(cli.BaseURL.Path, "/api/v3/") + "/api/graphql"
	}
	req, err := cli.NewRequest(http.MethodPost, endpoint.String(), map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if _, err := cli.Do(ctx, req, &raw); err != nil {
		return err
	}
	var status struct{ Errors []struct{ Message string } }
	if err := json.Unmarshal(raw, &status); err != nil {
		return err
	}
	if len(status.Errors) > 0 {
		return fmt.Errorf("GitHub review threads: %s", status.Errors[0].Message)
	}
	return json.Unmarshal(raw, result)
}

func readGitHubResolutions(ctx context.Context, cli *githubapi.Client, pr *models.AIReviewFeedback) ([]models.AIReviewInlineThread, error) {
	threads := append([]models.AIReviewInlineThread{}, pr.InlineThreads...)
	known := map[int64]bool{}
	counts := map[int64]int{}
	for _, thread := range threads {
		known[thread.CommentID] = true
		counts[thread.ReviewID]++
	}
	for _, target := range pr.Comments {
		if target.Kind != "review" || (target.InlineTotal > 0 && counts[target.ReviewID] >= target.InlineTotal) {
			continue
		}
		items, err := ReadGitHubInlineThreads(ctx, cli, pr.RepoOwner, pr.RepoName, pr.PR, target.ReviewID)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !known[item.CommentID] {
				threads = append(threads, item)
				known[item.CommentID] = true
			}
		}
	}
	if len(threads) == 0 {
		return threads, nil
	}
	// Fetch one root comment per thread; replies are irrelevant to AI attribution.
	query := `query($owner:String!,$name:String!,$number:Int!,$after:String){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100,after:$after){nodes{id isResolved comments(first:1){nodes{id}}} pageInfo{hasNextPage endCursor}}}}}`
	started := time.Now()
	matched := map[string]bool{}
	indices := map[string]int{}
	for i, thread := range threads {
		indices[thread.CommentNodeID] = i
	}
	after := interface{}(nil)
	for {
		var response struct {
			Data struct {
				Repository *struct {
					PullRequest *struct {
						ReviewThreads *struct {
							Nodes []struct {
								ID         string
								IsResolved *bool
								Comments   struct{ Nodes []struct{ ID string } }
							}
							PageInfo struct {
								HasNextPage bool
								EndCursor   string
							}
						}
					}
				}
			}
		}
		if err := queryGitHubThreads(ctx, cli, query, map[string]interface{}{"owner": pr.RepoOwner, "name": pr.RepoName, "number": pr.PR, "after": after}, &response); err != nil {
			return nil, err
		}
		if response.Data.Repository == nil || response.Data.Repository.PullRequest == nil || response.Data.Repository.PullRequest.ReviewThreads == nil {
			return nil, fmt.Errorf("GitHub review threads response is missing pull request")
		}
		page := response.Data.Repository.PullRequest.ReviewThreads
		for _, item := range page.Nodes {
			if len(item.Comments.Nodes) == 0 {
				continue
			}
			root := item.Comments.Nodes[0].ID
			if item.IsResolved == nil || root == "" {
				return nil, fmt.Errorf("GitHub review thread response is missing state/comment ID")
			}
			if i, ok := indices[root]; ok {
				threads[i].ThreadID, threads[i].Resolved, threads[i].Deleted, threads[i].UpdatedAt = item.ID, *item.IsResolved, false, started
				matched[root] = true
			}
		}
		if !page.PageInfo.HasNextPage {
			break
		}
		if page.PageInfo.EndCursor == "" || page.PageInfo.EndCursor == after {
			return nil, fmt.Errorf("GitHub review threads pagination did not advance")
		}
		after = page.PageInfo.EndCursor
	}
	for i := range threads {
		if !matched[threads[i].CommentNodeID] {
			threads[i].Deleted, threads[i].Resolved, threads[i].UpdatedAt = true, false, started
		}
	}
	return threads, nil
}

func readGitLabResolutions(ctx context.Context, cli *gitlab.Client, pr *models.AIReviewFeedback) ([]models.AIReviewInlineThread, error) {
	threads := append([]models.AIReviewInlineThread{}, pr.InlineThreads...)
	registered := map[int64]bool{}
	indices := map[int64]int{}
	for _, comment := range pr.Comments {
		if comment.Kind == "note" {
			registered[comment.CommentID] = true
		}
	}
	for i, thread := range threads {
		indices[thread.CommentID] = i
	}
	project := strings.TrimLeft(pr.RepoOwner+"/"+pr.RepoName, "/")
	if pr.ProjectID > 0 {
		project = fmt.Sprint(pr.ProjectID)
	}
	started := time.Now()
	found := map[int64]bool{}
	opts := &gitlab.ListMergeRequestDiscussionsOptions{PerPage: 100}
	for {
		discussions, resp, err := cli.Discussions.ListMergeRequestDiscussions(project, pr.PR, opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		for _, discussion := range discussions {
			if discussion == nil || len(discussion.Notes) == 0 {
				continue
			}
			note := discussion.Notes[0]
			if note == nil || !registered[int64(note.ID)] || note.Position == nil || !note.Resolvable {
				continue
			}
			id := int64(note.ID)
			thread := models.AIReviewInlineThread{CommentID: id, ThreadID: discussion.ID, Resolved: note.Resolved, UpdatedAt: started}
			if i, ok := indices[id]; ok {
				threads[i] = thread
			} else {
				indices[id] = len(threads)
				threads = append(threads, thread)
			}
			found[id] = true
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	for i := range threads {
		if !found[threads[i].CommentID] {
			threads[i].Deleted, threads[i].Resolved, threads[i].UpdatedAt = true, false, started
		}
	}
	return threads, nil
}

// applyThreadState rejects duplicate and older deliveries. Polling observations
// also carry a timestamp, so delayed webhooks cannot overwrite a newer snapshot.
func applyThreadState(pr *models.AIReviewFeedback, commentIDs []int64, threadID string, resolved bool, updatedAt time.Time) bool {
	changed := false
	for i := range pr.InlineThreads {
		thread := &pr.InlineThreads[i]
		for _, id := range commentIDs {
			if thread.CommentID != id || thread.Deleted || !updatedAt.After(thread.UpdatedAt) {
				continue
			}
			thread.Resolved, thread.UpdatedAt = resolved, updatedAt
			if threadID != "" {
				thread.ThreadID = threadID
			}
			changed = true
			break
		}
	}
	return changed
}

func ApplyGitHubThread(ctx context.Context, sourceHost, owner, name string, number int, commentIDs []int64, threadID string, log *zap.SugaredLogger) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	key := bson.M{"provider": setting.SourceFromGithub, "source_host": sourceHost, "repo_owner": owner, "repo_name": name, "pr": number, "closed": false}
	coll := repo.NewAIReviewFeedbackColl()
	for ctx.Err() == nil {
		var pr models.AIReviewFeedback
		if err := coll.FindOne(ctx, key).Decode(&pr); errors.Is(err, mongo.ErrNoDocuments) {
			log.Infow("ignored github review thread webhook", "reason", "PR is not registered or frozen")
			return nil
		} else if err != nil {
			return err
		}
		eligible := false
		for _, thread := range pr.InlineThreads {
			for _, id := range commentIDs {
				if thread.CommentID == id && !thread.Deleted {
					eligible = true
					break
				}
			}
		}
		if !eligible {
			log.Infow("ignored github review thread webhook", "reason", "AI thread is not registered")
			return nil
		}
		host, err := systemconfig.New().GetCodeHost(pr.CodehostID, ctx)
		if err != nil {
			return err
		}
		cli, err := newGitHubFeedbackClient(ctx, &pr, host)
		if err != nil {
			return err
		}
		updatedAt := time.Now()
		var response struct {
			Data struct{ Node *struct{ IsResolved *bool } }
		}
		if err := queryGitHubThreads(ctx, cli, `query($id:ID!){node(id:$id){... on PullRequestReviewThread{isResolved}}}`, map[string]interface{}{"id": threadID}, &response); err != nil {
			return err
		}
		if response.Data.Node == nil || response.Data.Node.IsResolved == nil {
			return fmt.Errorf("GitHub review thread is missing")
		}
		if !applyThreadState(&pr, commentIDs, threadID, *response.Data.Node.IsResolved, updatedAt) {
			log.Infow("ignored github review thread webhook", "reason", "obsolete observation")
			return nil
		}
		total, count := inlineTotals(&pr)
		filter := prKey(pr.CodehostID, pr.RepoOwner, pr.RepoName, pr.PR)
		filter["revision"], filter["closed"] = pr.Revision, false
		result, err := coll.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"inline_threads": pr.InlineThreads, "inline_total": total, "inline_resolved": count, "updated_at": time.Now()}, "$inc": bson.M{"revision": int64(1)}})
		if err != nil {
			return err
		}
		if result.MatchedCount > 0 {
			log.Infow("updated github AI review thread", "pr", number, "inline_total", total, "inline_resolved", count)
			return nil
		}
	}
	return ctx.Err()
}
