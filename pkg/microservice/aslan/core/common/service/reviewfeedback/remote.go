package reviewfeedback

import (
	"context"
	"fmt"
	githubapi "github.com/google/go-github/v35/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/xanzy/go-gitlab"
	"net/http"
	"strings"
	"time"
)

type reactionCount struct{ Up, Down int }

func readGitHubTarget(ctx context.Context, cli *githubapi.Client, pr *models.AIReviewFeedback, target models.AIReviewFeedbackComment) (reactionCount, error) {
	if target.Kind == "issue" {
		item, resp, err := cli.Issues.GetComment(ctx, pr.RepoOwner, pr.RepoName, target.CommentID)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return reactionCount{}, nil
		}
		if err != nil {
			return reactionCount{}, err
		}
		return countGitHubReactions(item.Reactions)
	}
	if target.Kind != "review" || target.ReviewID <= 0 {
		return reactionCount{}, fmt.Errorf("invalid GitHub feedback target %s/%d", target.Kind, target.CommentID)
	}
	count := reactionCount{}
	opt := &githubapi.ListOptions{PerPage: 100}
	for {
		items, resp, err := cli.PullRequests.ListReviewComments(ctx, pr.RepoOwner, pr.RepoName, pr.PR, target.ReviewID, opt)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return reactionCount{}, nil
		}
		if err != nil {
			return reactionCount{}, err
		}
		for _, item := range items {
			if item == nil || item.GetInReplyTo() != 0 || !strings.Contains(item.GetBody(), "<!-- zadig-ai-review -->") {
				continue
			}
			itemCount, err := countGitHubReactions(item.Reactions)
			if err != nil {
				return reactionCount{}, err
			}
			count.Up += itemCount.Up
			count.Down += itemCount.Down
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return count, nil
}

func readGitLabTarget(ctx context.Context, cli *gitlab.Client, pr *models.AIReviewFeedback, target models.AIReviewFeedbackComment) (reactionCount, error) {
	project := strings.TrimLeft(pr.RepoOwner+"/"+pr.RepoName, "/")
	if pr.ProjectID > 0 {
		project = fmt.Sprint(pr.ProjectID)
	}
	opt := &gitlab.ListAwardEmojiOptions{PerPage: 100}
	count := reactionCount{}
	for {
		items, resp, err := cli.AwardEmoji.ListMergeRequestAwardEmojiOnNote(project, pr.PR, int(target.CommentID), opt, gitlab.WithContext(ctx))
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// Confirm that the note was deleted rather than silently treating an API failure as zero.
			_, noteResp, noteErr := cli.Notes.GetMergeRequestNote(project, pr.PR, int(target.CommentID), gitlab.WithContext(ctx))
			if noteResp != nil && noteResp.StatusCode == http.StatusNotFound {
				return reactionCount{}, nil
			}
			if noteErr != nil {
				return reactionCount{}, noteErr
			}
		}
		if err != nil {
			return reactionCount{}, err
		}
		for _, item := range items {
			if item != nil {
				switch item.Name {
				case "thumbsup":
					count.Up++
				case "thumbsdown":
					count.Down++
				}
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return count, nil
}

func needsRefresh(target models.AIReviewFeedbackComment, cycle time.Time) bool {
	return target.SyncedAt.IsZero() || (!target.DirtyAt.IsZero() && !target.DirtyAt.Before(target.SyncedAt)) || (!cycle.IsZero() && target.SyncedAt.Before(cycle))
}

func countGitHubReactions(r *githubapi.Reactions) (reactionCount, error) {
	if r == nil || r.PlusOne == nil || r.MinusOne == nil {
		return reactionCount{}, fmt.Errorf("GitHub comment response is missing reaction counts")
	}
	return reactionCount{Up: r.GetPlusOne(), Down: r.GetMinusOne()}, nil
}
