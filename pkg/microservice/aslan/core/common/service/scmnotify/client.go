/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scmnotify

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	giteeClient "gitee.com/openeuler/go-gitee/gitee"
	githubapi "github.com/google/go-github/v35/github"
	"github.com/pkg/errors"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitee"
	githubservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/reviewfeedback"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	gitlabtool "github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

type Client struct {
	logger *zap.SugaredLogger
}

func NewClient() *Client {
	return &Client{logger: log.SugaredLogger()}
}

func (c *Client) CreateAIReviewComment(codehostID int, projectID, repoOwner, repoName string, prID int, comment string) (reviewfeedback.PublishedComment, error) {
	if prID <= 0 {
		return reviewfeedback.PublishedComment{}, fmt.Errorf("invalid pull/merge request ID %d", prID)
	}
	codeHostDetail, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return reviewfeedback.PublishedComment{}, errors.Wrapf(err, "codehost %d not found to publish AI review result", codehostID)
	}

	switch strings.ToLower(codeHostDetail.Type) {
	case setting.SourceFromGitlab:
		cli, err := gitlabtool.NewClient(
			codeHostDetail.ID,
			codeHostDetail.Address,
			codeHostDetail.AccessToken,
			config.ProxyHTTPSAddr(),
			codeHostDetail.EnableProxy,
			codeHostDetail.DisableSSL,
		)
		if err != nil {
			return reviewfeedback.PublishedComment{}, fmt.Errorf("create gitlab client: %w", err)
		}
		item, _, err := cli.Notes.CreateMergeRequestNote(projectID, prID, &gitlab.CreateMergeRequestNoteOptions{Body: &comment})
		if err != nil {
			return reviewfeedback.PublishedComment{}, fmt.Errorf("create GitLab merge request note: %w", err)
		}
		if isAIReviewSummary(comment) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := archiveGitLabAIReviewSummaries(ctx, cli.Client, projectID, prID, int64(item.ID)); err != nil && c.logger != nil {
				c.logger.Warnf("failed to collapse previous GitLab AI review summaries for %s#%d: %v", projectID, prID, err)
			}
		}
		return reviewfeedback.PublishedComment{Kind: "note", CommentID: int64(item.ID)}, nil
	case setting.SourceFromGithub:
		cli, err := githubservice.GetGithubAppClientByOwner(repoOwner)
		if err != nil {
			return reviewfeedback.PublishedComment{}, fmt.Errorf("create github app client: %w", err)
		}
		if cli == nil {
			cli = githubservice.NewClient(codeHostDetail.AccessToken, config.ProxyHTTPSAddr(), codeHostDetail.EnableProxy)
		}
		item, _, err := cli.Issues.CreateComment(context.Background(), repoOwner, repoName, prID, &githubapi.IssueComment{Body: &comment})
		if err != nil {
			return reviewfeedback.PublishedComment{}, fmt.Errorf("create GitHub pull request comment: %w", err)
		}
		if isAIReviewSummary(comment) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := archiveGitHubAIReviewSummaries(ctx, cli.Client.Client, repoOwner, repoName, prID, item.GetID()); err != nil && c.logger != nil {
				c.logger.Warnf("failed to minimize previous GitHub AI review summaries for %s/%s#%d: %v", repoOwner, repoName, prID, err)
			}
		}
		return reviewfeedback.PublishedComment{Kind: "issue", CommentID: item.GetID()}, nil
	default:
		return reviewfeedback.PublishedComment{}, fmt.Errorf("codehost type %q does not support AI review comments", codeHostDetail.Type)
	}
}

func (c *Client) getAIReviewPRMetadata(codehostID int, project, owner, name string, prID int) (int, AIReviewPRMetadata, error) {
	host, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return 0, AIReviewPRMetadata{}, err
	}
	if strings.EqualFold(host.Type, setting.SourceFromGitlab) {
		cli, err := gitlabtool.NewClient(host.ID, host.Address, host.AccessToken, config.ProxyHTTPSAddr(), host.EnableProxy, host.DisableSSL)
		if err != nil {
			return 0, AIReviewPRMetadata{}, err
		}
		mr, _, err := cli.MergeRequests.GetMergeRequest(project, prID, nil)
		if err != nil {
			return 0, AIReviewPRMetadata{}, err
		}
		metadata := AIReviewPRMetadata{Title: mr.Title, URL: mr.WebURL}
		if mr.Author != nil {
			metadata.Author = mr.Author.Username
		}
		return mr.ProjectID, metadata, nil
	}
	cli, err := githubservice.GetGithubAppClientByOwner(owner)
	if err != nil {
		return 0, AIReviewPRMetadata{}, err
	}
	if cli == nil {
		cli = githubservice.NewClient(host.AccessToken, config.ProxyHTTPSAddr(), host.EnableProxy)
	}
	pull, _, err := cli.PullRequests.Get(context.Background(), owner, name, prID)
	if err != nil {
		return 0, AIReviewPRMetadata{}, err
	}
	return 0, AIReviewPRMetadata{Title: pull.GetTitle(), Author: pull.GetUser().GetLogin(), URL: pull.GetHTMLURL()}, nil
}

func (c *Client) createAIReviewInlineComments(ctx context.Context, codehostID int, projectID, repoOwner, repoName string, prID int, report *stepspec.AIReviewReport) (aiReviewInlinePublishResult, error) {
	if prID <= 0 {
		return aiReviewInlinePublishResult{}, fmt.Errorf("invalid pull/merge request ID %d", prID)
	}
	codeHostDetail, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return aiReviewInlinePublishResult{}, errors.Wrapf(err, "codehost %d not found to publish inline AI review comments", codehostID)
	}

	switch strings.ToLower(codeHostDetail.Type) {
	case setting.SourceFromGitlab:
		cli, err := gitlabtool.NewClient(
			codeHostDetail.ID,
			codeHostDetail.Address,
			codeHostDetail.AccessToken,
			config.ProxyHTTPSAddr(),
			codeHostDetail.EnableProxy,
			codeHostDetail.DisableSSL,
		)
		if err != nil {
			return aiReviewInlinePublishResult{}, fmt.Errorf("create gitlab client: %w", err)
		}
		return c.createGitLabAIReviewInlineComments(ctx, cli.Client, projectID, prID, report)
	case setting.SourceFromGithub:
		cli, err := githubservice.GetGithubAppClientByOwner(repoOwner)
		if err != nil {
			return aiReviewInlinePublishResult{}, fmt.Errorf("create github app client: %w", err)
		}
		if cli == nil {
			cli = githubservice.NewClient(codeHostDetail.AccessToken, config.ProxyHTTPSAddr(), codeHostDetail.EnableProxy)
		}
		return c.createGitHubAIReviewInlineComments(ctx, cli.Client.Client, repoOwner, repoName, prID, report)
	default:
		return aiReviewInlinePublishResult{}, fmt.Errorf("codehost type %q does not support inline AI review comments", codeHostDetail.Type)
	}
}

func (c *Client) createGitHubAIReviewInlineComments(ctx context.Context, cli *githubapi.Client, repoOwner, repoName string, prID int, report *stepspec.AIReviewReport) (aiReviewInlinePublishResult, error) {
	pullRequest, _, err := cli.PullRequests.Get(ctx, repoOwner, repoName, prID)
	if err != nil {
		return aiReviewInlinePublishResult{}, fmt.Errorf("get GitHub pull request: %w", err)
	}
	result := aiReviewInlinePublishResult{Fallback: make([]stepspec.AIReviewFinding, 0), Title: pullRequest.GetTitle(), Author: pullRequest.GetUser().GetLogin(), URL: pullRequest.GetHTMLURL()}
	existing, err := reviewfeedback.CurrentGitHubInlineThreads(ctx, cli, repoOwner, repoName, prID)
	if err != nil {
		return result, fmt.Errorf("read current GitHub AI review threads: %w", err)
	}
	result.Threads = append(result.Threads, existing...)
	reviews := make(map[int64]int)
	for _, thread := range existing {
		if thread.ReviewID > 0 {
			reviews[thread.ReviewID]++
		}
	}
	for id, count := range reviews {
		result.Comments = append(result.Comments, reviewfeedback.PublishedComment{Kind: "review", CommentID: id, ReviewID: id, InlineTotal: count})
	}
	findings, skipped := filterAIReviewFindings(report.Findings, existing)
	result.Skipped = skipped
	headSHA := pullRequest.GetHead().GetSHA()
	if headSHA == "" {
		return result, fmt.Errorf("GitHub pull request head SHA is empty")
	}

	patches := make(map[string]string)
	options := &githubapi.ListOptions{PerPage: 100}
	for {
		files, resp, err := cli.PullRequests.ListFiles(ctx, repoOwner, repoName, prID, options)
		if err != nil {
			return result, fmt.Errorf("list GitHub pull request files: %w", err)
		}
		for _, file := range files {
			if file == nil {
				continue
			}
			patches[file.GetFilename()] = file.GetPatch()
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		options.Page = resp.NextPage
	}

	comments := make([]*githubapi.DraftReviewComment, 0, len(findings))
	publishedFindings := make([]stepspec.AIReviewFinding, 0, len(findings))
	for _, finding := range findings {
		anchor, ok := findAIReviewAddedLine(patches[finding.File], finding.StartLine, finding.EndLine)
		if !ok {
			result.Fallback = append(result.Fallback, finding)
			continue
		}
		publishedFindings = append(publishedFindings, finding)
		comments = append(comments, &githubapi.DraftReviewComment{
			Path: githubapi.String(finding.File),
			Body: githubapi.String(formatAIReviewInlineComment(finding)),
			Side: githubapi.String("RIGHT"),
			Line: githubapi.Int(anchor),
		})
	}
	if len(comments) == 0 {
		return result, nil
	}
	review, _, err := cli.PullRequests.CreateReview(ctx, repoOwner, repoName, prID, &githubapi.PullRequestReviewRequest{
		CommitID: githubapi.String(headSHA),
		Event:    githubapi.String("COMMENT"),
		Comments: comments,
	})
	if err != nil {
		return result, fmt.Errorf("create GitHub pull request review: %w", err)
	}
	result.Published = len(comments)
	result.Comments = append(result.Comments, reviewfeedback.PublishedComment{Kind: "review", CommentID: review.GetID(), ReviewID: review.GetID(), InlineTotal: len(comments)})
	newThreads, publishedComments, err := reviewfeedback.ReadGitHubInlineThreads(ctx, cli, repoOwner, repoName, prID, review.GetID())
	if err != nil && c.logger != nil {
		c.logger.Warnf("failed to register GitHub inline comment IDs for review %d: %v", review.GetID(), err)
	}
	for _, item := range publishedComments {
		if item.GetHTMLURL() == "" {
			continue
		}
		finding := stepspec.AIReviewFinding{Title: "行内评论", File: item.GetPath()}
		for i, draft := range comments {
			if item.GetPath() == draft.GetPath() && item.GetLine() == draft.GetLine() && item.GetBody() == draft.GetBody() {
				finding = publishedFindings[i]
				break
			}
		}
		result.Links = append(result.Links, aiReviewInlineLink{URL: item.GetHTMLURL(), Finding: finding, Line: item.GetLine()})
	}
	result.Threads = append(result.Threads, newThreads...)
	return result, nil
}

func (c *Client) createGitLabAIReviewInlineComments(ctx context.Context, cli *gitlab.Client, projectID string, prID int, report *stepspec.AIReviewReport) (aiReviewInlinePublishResult, error) {
	mergeRequest, _, err := cli.MergeRequests.GetMergeRequestChanges(projectID, prID, nil, gitlab.WithContext(ctx))
	if err != nil {
		return aiReviewInlinePublishResult{}, fmt.Errorf("get GitLab merge request changes: %w", err)
	}
	result := aiReviewInlinePublishResult{Fallback: make([]stepspec.AIReviewFinding, 0), Title: mergeRequest.Title, URL: mergeRequest.WebURL, ProjectID: mergeRequest.ProjectID}
	if mergeRequest.Author != nil {
		result.Author = mergeRequest.Author.Username
	}
	existing, err := reviewfeedback.CurrentGitLabInlineThreads(ctx, cli, projectID, prID)
	if err != nil {
		return result, fmt.Errorf("read current GitLab AI review threads: %w", err)
	}
	result.Threads = append(result.Threads, existing...)
	for _, thread := range existing {
		result.Comments = append(result.Comments, reviewfeedback.PublishedComment{Kind: "note", CommentID: thread.CommentID})
	}
	findings, skipped := filterAIReviewFindings(report.Findings, existing)
	result.Skipped = skipped
	if mergeRequest.DiffRefs.HeadSha == "" {
		return result, fmt.Errorf("GitLab merge request head SHA is empty")
	}

	type gitLabPatch struct {
		oldPath string
		patch   string
	}
	patches := make(map[string]gitLabPatch, len(mergeRequest.Changes))
	for _, change := range mergeRequest.Changes {
		patches[change.NewPath] = gitLabPatch{oldPath: change.OldPath, patch: change.Diff}
	}

	for _, finding := range findings {
		filePatch, ok := patches[finding.File]
		if !ok {
			result.Fallback = append(result.Fallback, finding)
			continue
		}
		anchor, ok := findAIReviewAddedLine(filePatch.patch, finding.StartLine, finding.EndLine)
		if !ok {
			result.Fallback = append(result.Fallback, finding)
			continue
		}
		discussion, _, err := cli.Discussions.CreateMergeRequestDiscussion(
			projectID,
			prID,
			&gitlab.CreateMergeRequestDiscussionOptions{
				Body: gitlab.String(formatAIReviewInlineComment(finding)),
				Position: &gitlab.NotePosition{
					BaseSHA:      mergeRequest.DiffRefs.BaseSha,
					StartSHA:     mergeRequest.DiffRefs.StartSha,
					HeadSHA:      mergeRequest.DiffRefs.HeadSha,
					PositionType: "text",
					OldPath:      filePatch.oldPath,
					NewPath:      finding.File,
					NewLine:      anchor,
				},
			}, gitlab.WithContext(ctx),
		)
		if err != nil {
			result.Fallback = append(result.Fallback, finding)
			if c.logger != nil {
				c.logger.Warnf("failed to create GitLab inline AI review comment for %s:%d: %v", finding.File, anchor, err)
			}
			continue
		}
		result.Published++
		if len(discussion.Notes) > 0 {
			if result.URL != "" && discussion.Notes[0].ID > 0 {
				result.Links = append(result.Links, aiReviewInlineLink{URL: fmt.Sprintf("%s#note_%d", result.URL, discussion.Notes[0].ID), Finding: finding, Line: anchor})
			}
			result.Comments = append(result.Comments, reviewfeedback.PublishedComment{Kind: "note", CommentID: int64(discussion.Notes[0].ID)})
			result.Threads = append(result.Threads, models.AIReviewInlineThread{CommentID: int64(discussion.Notes[0].ID), ThreadID: discussion.ID, Resolved: discussion.Notes[0].Resolved, Fingerprint: finding.Fingerprint})
		}
	}
	return result, nil
}

// Comment send comment to gitlab and set comment id in notify
func (c *Client) Comment(notify *models.Notification) error {
	if notify.PrID == 0 {
		return fmt.Errorf("non pr notification not supported yet")
	}

	var err error
	comment := notify.ErrInfo
	if comment == "" {
		if comment, err = notify.CreateCommentBody(); err != nil {
			return fmt.Errorf("failed to create comment body %v", err)
		}
	}

	codeHostDetail, err := systemconfig.New().GetCodeHost(notify.CodehostID)
	if err != nil {
		return errors.Wrapf(err, "codehost %d not found to comment", notify.CodehostID)
	}
	if strings.ToLower(codeHostDetail.Type) == setting.SourceFromGitlab {
		var note *gitlab.Note
		cli, err := gitlabtool.NewClient(codeHostDetail.ID, codeHostDetail.Address, codeHostDetail.AccessToken, config.ProxyHTTPSAddr(), codeHostDetail.EnableProxy, codeHostDetail.DisableSSL)
		if err != nil {
			c.logger.Errorf("create gitlab client failed err: %v", err)
			return fmt.Errorf("create gitlab client failed err: %v", err)
		}
		if notify.CommentID == "" {
			// create comment
			note, _, err = cli.Notes.CreateMergeRequestNote(
				notify.ProjectID, notify.PrID, &gitlab.CreateMergeRequestNoteOptions{
					Body: &comment,
				},
			)

			if err == nil {
				notify.CommentID = strconv.Itoa(note.ID)
			}
		} else {
			// update comment
			noteID, _ := strconv.Atoi(notify.CommentID)
			_, _, err = cli.Notes.UpdateMergeRequestNote(
				notify.ProjectID, notify.PrID, noteID, &gitlab.UpdateMergeRequestNoteOptions{
					Body: &comment,
				})
		}

		if err != nil {
			return fmt.Errorf("failed to comment gitlab due to %s/%d %v", notify.ProjectID, notify.PrID, err)
		}
	} else if strings.ToLower(codeHostDetail.Type) == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(codeHostDetail.Address, codeHostDetail.AccessToken, config.ProxyHTTPSAddr(), codeHostDetail.EnableProxy)
		for _, task := range notify.Tasks {
			// create task created comment
			encodedDisplayName := url.PathEscape(task.WorkflowDisplayName)
			workflowURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/multi/%s/%d?display_name=%s", notify.BaseURI, task.ProductName, task.WorkflowName, task.ID, encodedDisplayName)
			if notify.IsWorkflowV4 {
				workflowURL = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", notify.BaseURI, task.ProductName, task.WorkflowName, task.ID, encodedDisplayName)
			}
			if !task.FirstCommented && task.Status == config.TaskStatusReady {
				if e := cli.SetReview(
					notify.RepoName,
					notify.PrID,
					fmt.Sprintf(""+
						"%s ⏱️ %s",
						strings.ToUpper(string(task.Status)),
						workflowURL,
					),
					notify.Label,
					"0",
					notify.Revision,
				); e != nil {
					c.logger.Warnf("failed to set review %v %v %v", task, notify, e)
				}

				task.FirstCommented = true
				continue
			}

			/* set review score*/
			var emoji, score string
			var skip bool
			switch task.Status {
			case config.TaskStatusPass:
				emoji = "✅"
				score = "+1"
			case config.TaskStatusCancelled:
				emoji = "✖️"
				score = "0"
			case config.TaskStatusTimeout, config.TaskStatusFailed:
				emoji = "❌"
				score = "-1"
			default:
				skip = true
			}

			if !skip {
				if e := cli.SetReview(
					notify.RepoName,
					notify.PrID,
					fmt.Sprintf(""+
						"%s %s %s",
						strings.ToUpper(string(task.Status)),
						emoji,
						workflowURL,
					),
					notify.Label,
					score,
					notify.Revision,
				); e != nil {
					c.logger.Warnf("failed to set review %v %v %v", task, notify, e)
				}
			}
		}
	} else if strings.ToLower(codeHostDetail.Type) == setting.SourceFromGitee || strings.ToLower(codeHostDetail.Type) == setting.SourceFromGiteeEE {
		cli := gitee.NewClient(codeHostDetail.ID, codeHostDetail.AccessToken, config.ProxyHTTPSAddr(), codeHostDetail.EnableProxy, codeHostDetail.Address)
		var pullRequestComments giteeClient.PullRequestComments
		if notify.CommentID == "" {
			// create comment
			pullRequestComments, err = cli.CreateMergeRequestComment(context.Background(),
				notify.RepoOwner, notify.RepoName, int32(notify.PrID), giteeClient.PullRequestCommentPostParam{
					Body: comment,
				},
			)

			if err == nil {
				notify.CommentID = strconv.Itoa(int(pullRequestComments.Id))
			}
		} else {
			// update comment
			commentID, err := strconv.Atoi(notify.CommentID)
			if err != nil {
				return fmt.Errorf("failed to atoi commentID %v,err: %s", notify.CommentID, err)
			}
			err = cli.UpdateMergeRequestComment(context.Background(),
				notify.RepoOwner, notify.RepoName, int32(commentID), giteeClient.PullRequestCommentPatchParam{
					Body: comment,
				})
		}

		if err != nil {
			return fmt.Errorf("failed to comment gitee due to %s/%d %v", notify.ProjectID, notify.PrID, err)
		}
	} else {
		return fmt.Errorf("non gitlab source not supported to comment")
	}

	return nil
}
