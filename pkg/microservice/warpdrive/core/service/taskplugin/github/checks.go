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

package github

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/go-github/v35/github"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
)

const (
	StatusQueued     = "queued"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

type CIStatus string

const (
	CIStatusSuccess   CIStatus = "success"
	CIStatusFailure   CIStatus = "failure"
	CIStatusNeutral   CIStatus = "neutral"
	CIStatusCancelled CIStatus = "cancelled"
	CIStatusTimeout   CIStatus = "timed_out"
	CIStatusError     CIStatus = "error"
)

type GitCheck struct {
	Owner  string
	Repo   string
	Branch string // The name of the branch to perform a check against. (Required.)
	Ref    string // The SHA of the commit. (Required.)
	IsPr   bool

	AslanURL    string
	PipeName    string
	DisplayName string
	ProductName string
	PipeType    config.PipelineType
	TaskID      int64
	TestReports []*types.TestSuite
}

// DetailsURL ...
func (gc *GitCheck) DetailsURL() string {
	url := GetTaskLink(
		gc.AslanURL,
		gc.ProductName,
		gc.PipeName,
		gc.DisplayName,
		gc.PipeType,
		gc.TaskID,
	)

	return url
}

func GetTaskLink(baseURI, productName, pipelineName, displayName string, pipelineType config.PipelineType, taskID int64) string {
	disPlayNameEncoded := url.QueryEscape(displayName)
	return fmt.Sprintf(
		"%s/v1/projects/detail/%s/pipelines/%s/%s/%d?display_name=%s",
		baseURI,
		productName,
		UIType(pipelineType),
		pipelineName,
		taskID,
		disPlayNameEncoded,
	)
}

func UIType(pipelineType config.PipelineType) string {
	if pipelineType == config.SingleType {
		return "single"
	}
	return "multi"
}

// https://developer.github.com/v3/checks/runs/#create-a-check-run
func (c *Client) StartGitCheck(check *GitCheck) (int64, error) {

	opt := github.CreateCheckRunOptions{
		Name:       fmt.Sprintf("Aslan - %s", check.DisplayName),
		HeadSHA:    check.Ref,
		DetailsURL: github.String(check.DetailsURL()),
		ExternalID: github.String(fmt.Sprintf("%s/%d", check.PipeName, check.TaskID)),
		StartedAt:  &github.Timestamp{Time: time.Now()},
		Status:     github.String(StatusQueued),
	}

	run, err := c.CreateCheckRun(context.TODO(), check.Owner, check.Repo, opt)
	if err != nil {
		return 0, err
	}

	fmt.Printf("StartGitCheck --> %+v\n", run)

	return run.GetID(), nil
}

// https://developer.github.com/v3/checks/runs/#update-a-check-run
func (c *Client) UpdateGitCheck(gitCheckID int64, check *GitCheck) error {
	opt := github.UpdateCheckRunOptions{
		Name:       fmt.Sprintf("Aslan - %s", check.DisplayName),
		DetailsURL: github.String(check.DetailsURL()),
		ExternalID: github.String(fmt.Sprintf("%s/%d", check.PipeName, check.TaskID)),
		Status:     github.String(StatusInProgress),
		Output: &github.CheckRunOutput{
			Title:   github.String("Pipeline started"),
			Summary: github.String(fmt.Sprintf("<a href='%s'> The **%s** pipeline</a> is currently running.", check.DetailsURL(), check.PipeName)),
		},
	}

	if check.PipeType == config.WorkflowType {
		opt.Output.Summary = github.String(fmt.Sprintf("<a href='%s'> The **%s** workflow</a> is currently running.", check.DetailsURL(), check.DisplayName))
	}
	_, err := c.UpdateCheckRun(context.TODO(), check.Owner, check.Repo, gitCheckID, opt)
	return err
}

// https://developer.github.com/v3/checks/runs/#update-a-check-run
func (c *Client) CompleteGitCheck(gitCheckID int64, status CIStatus, check *GitCheck) error {
	summary := fmt.Sprintf("<a href='%s'> The **%s** pipeline</a> is **%s**.", check.DetailsURL(), check.PipeName, status)
	if check.PipeType == config.WorkflowType {
		summary = fmt.Sprintf("<a href='%s'> The **%s** workflow</a> is **%s**.", check.DetailsURL(), check.DisplayName, status)
	}
	if len(check.TestReports) != 0 {
		summary += "<br/> test result: <br/>"
		for _, testReport := range check.TestReports {
			summary += fmt.Sprintf("test name: %s, success/total: %d/%d <br/>", testReport.Name, testReport.Successes, testReport.Tests)
		}
	}

	opt := github.UpdateCheckRunOptions{
		Name:        fmt.Sprintf("Aslan - %s", check.DisplayName),
		DetailsURL:  github.String(check.DetailsURL()),
		ExternalID:  github.String(fmt.Sprintf("%s/%d", check.PipeName, check.TaskID)),
		Status:      github.String(StatusCompleted),
		CompletedAt: &github.Timestamp{Time: time.Now()},
		Conclusion:  github.String(string(status)),
		Output: &github.CheckRunOutput{
			Title:   github.String(fmt.Sprintf("Pipeline %s", status)),
			Summary: github.String(summary),
		},
	}

	_, err := c.UpdateCheckRun(context.TODO(), check.Owner, check.Repo, gitCheckID, opt)
	return err
}
