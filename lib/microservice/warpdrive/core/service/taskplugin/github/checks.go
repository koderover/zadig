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
	"time"

	"github.com/google/go-github/v35/github"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types"
)

const (
	// StatusQueued ...
	StatusQueued = "queued"
	// StatusInProgress ...
	StatusInProgress = "in_progress"
	// StatusCompleted ...
	StatusCompleted = "completed"
)

const (
	// CIStatusSuccess ...
	CIStatusSuccess = CIStatus("success")
	// CIStatusFailure ...
	CIStatusFailure = CIStatus("failure")
	// CIStatusNeutral ...
	CIStatusNeutral = CIStatus("neutral")
	// CIStatusCancelled ...
	CIStatusCancelled = CIStatus("cancelled")
	// CIStatusTimeout ...
	CIStatusTimeout = CIStatus("timed_out")
)

// CIStatus ...
type CIStatus string

// GitCheck ...
type GitCheck struct {
	Owner  string
	Repo   string
	Branch string // The name of the branch to perform a check against. (Required.)
	Ref    string // The SHA of the commit. (Required.)
	IsPr   bool

	AslanURL    string
	PipeName    string
	ProductName string
	PipeType    config.PipelineType
	TaskID      int64
	TestReports []*types.TestSuite
}

// CheckService ...
type CheckService struct {
	client *Client
}

// DetailsURL ...
func (gc *GitCheck) DetailsURL() string {
	url := GetTaskLink(
		gc.AslanURL,
		gc.ProductName,
		gc.PipeName,
		gc.PipeType,
		gc.TaskID,
	)

	return url
}

func GetTaskLink(baseUri, productName, pipelineName string, pipelineType config.PipelineType, taskID int64) string {
	return fmt.Sprintf(
		"%s/v1/projects/detail/%s/pipelines/%s/%s/%d",
		baseUri,
		productName,
		UiType(pipelineType),
		pipelineName,
		taskID,
	)
}

func UiType(pipelineType config.PipelineType) string {
	if pipelineType == config.SingleType {
		return "single"
	} else {
		return "multi"
	}
}

// StartGitCheck ...
// https://developer.github.com/v3/checks/runs/#create-a-check-run
func (s *CheckService) StartGitCheck(check *GitCheck) (int64, error) {

	opt := github.CreateCheckRunOptions{
		Name:       fmt.Sprintf("Aslan - %s", check.PipeName),
		HeadSHA:    check.Ref,
		DetailsURL: github.String(check.DetailsURL()),
		ExternalID: github.String(fmt.Sprintf("%s/%d", check.PipeName, check.TaskID)),
		StartedAt:  &github.Timestamp{Time: time.Now()},
		Status:     github.String(StatusQueued),
	}

	run, _, err := s.client.Git.Checks.CreateCheckRun(context.Background(), check.Owner, check.Repo, opt)
	if err != nil {
		return 0, err
	}

	fmt.Printf("StartGitCheck --> %+v\n", run)

	return run.GetID(), nil
}

// UpdateGitCheck ...
// https://developer.github.com/v3/checks/runs/#update-a-check-run
func (s *CheckService) UpdateGitCheck(gitCheckID int64, check *GitCheck) error {
	opt := github.UpdateCheckRunOptions{
		Name:       fmt.Sprintf("Aslan - %s", check.PipeName),
		DetailsURL: github.String(check.DetailsURL()),
		ExternalID: github.String(fmt.Sprintf("%s/%d", check.PipeName, check.TaskID)),
		Status:     github.String(StatusInProgress),
		Output: &github.CheckRunOutput{
			Title:   github.String("Pipeline started"),
			Summary: github.String(fmt.Sprintf("<a href='%s'> The **%s** pipeline</a> is currently running.", check.DetailsURL(), check.PipeName)),
		},
	}

	if check.PipeType == config.WorkflowType {
		opt.Output.Summary = github.String(fmt.Sprintf("<a href='%s'> The **%s** workflow</a> is currently running.", check.DetailsURL(), check.PipeName))
	}
	_, _, err := s.client.Git.Checks.UpdateCheckRun(context.Background(), check.Owner, check.Repo, gitCheckID, opt)
	return err
}

// CompleteGitCheck ...
// https://developer.github.com/v3/checks/runs/#update-a-check-run
func (s *CheckService) CompleteGitCheck(gitCheckID int64, status CIStatus, check *GitCheck) error {
	summary := fmt.Sprintf("<a href='%s'> The **%s** pipeline</a> is **%s**.", check.DetailsURL(), check.PipeName, status)
	if check.PipeType == config.WorkflowType {
		summary = fmt.Sprintf("<a href='%s'> The **%s** workflow</a> is **%s**.", check.DetailsURL(), check.PipeName, status)
	}
	if len(check.TestReports) != 0 {
		summary += "<br/> test result: <br/>"
		for _, testReport := range check.TestReports {
			summary += fmt.Sprintf("test name: %s, success/total: %d/%d <br/>", testReport.Name, testReport.Successes, testReport.Tests)
		}
	}

	opt := github.UpdateCheckRunOptions{
		Name:        fmt.Sprintf("Aslan - %s", check.PipeName),
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

	_, _, err := s.client.Git.Checks.UpdateCheckRun(context.Background(), check.Owner, check.Repo, gitCheckID, opt)
	return err
}
