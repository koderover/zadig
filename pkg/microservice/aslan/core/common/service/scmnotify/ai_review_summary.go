package scmnotify

import (
	"context"
	"fmt"
	"strings"

	githubapi "github.com/google/go-github/v35/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/reviewfeedback"
	"github.com/xanzy/go-gitlab"
)

func isAIReviewSummary(body string) bool {
	return strings.Contains(body, aiReviewCommentMarker) &&
		!strings.Contains(body, "<!-- zadig-ai-review-fingerprint:") &&
		(strings.HasPrefix(body, "## Zadig AI Review\n") || strings.HasPrefix(body, "## Zadig AI 代码审查\n") || strings.HasPrefix(body, "## 🤖 Zadig AI Review\n") || strings.HasPrefix(body, "### 🤖 Zadig AI Review\n"))
}

func archiveGitHubAIReviewSummaries(ctx context.Context, cli *githubapi.Client, owner, name string, pr int, currentID int64) error {
	opts := &githubapi.IssueListCommentsOptions{Sort: githubapi.String("created"), Direction: githubapi.String("asc"), ListOptions: githubapi.ListOptions{PerPage: 100}}
	for {
		comments, resp, err := cli.Issues.ListComments(ctx, owner, name, pr, opts)
		if err != nil {
			return err
		}
		for _, comment := range comments {
			if comment == nil || comment.GetID() <= 0 || comment.GetID() >= currentID || !isAIReviewSummary(comment.GetBody()) {
				continue
			}
			if comment.GetNodeID() == "" {
				return fmt.Errorf("GitHub summary comment %d is missing node ID", comment.GetID())
			}
			if err := reviewfeedback.MinimizeGitHubComment(ctx, cli, comment.GetNodeID()); err != nil {
				return fmt.Errorf("minimize GitHub summary comment %d: %w", comment.GetID(), err)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			return nil
		}
		opts.Page = resp.NextPage
	}
}

func archiveGitLabAIReviewSummaries(ctx context.Context, cli *gitlab.Client, project string, pr int, currentID int64) error {
	opts := &gitlab.ListMergeRequestNotesOptions{OrderBy: gitlab.String("created_at"), Sort: gitlab.String("asc"), ListOptions: gitlab.ListOptions{PerPage: 100}}
	for {
		notes, resp, err := cli.Notes.ListMergeRequestNotes(project, pr, opts, gitlab.WithContext(ctx))
		if err != nil {
			return err
		}
		for _, note := range notes {
			if note == nil || note.ID <= 0 || int64(note.ID) >= currentID || note.System || note.Position != nil || note.Type == "DiffNote" || !isAIReviewSummary(note.Body) {
				continue
			}
			body := "<details>\n<summary>点击展开 Zadig AI Review 详情</summary>\n\n" + note.Body + "\n\n</details>"
			if _, _, err := cli.Notes.UpdateMergeRequestNote(project, pr, note.ID, &gitlab.UpdateMergeRequestNoteOptions{Body: &body}, gitlab.WithContext(ctx)); err != nil {
				return fmt.Errorf("collapse GitLab summary note %d: %w", note.ID, err)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			return nil
		}
		opts.Page = resp.NextPage
	}
}
