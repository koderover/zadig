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

package ilyshin

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type MergeRequest struct {
	ID              int        `json:"id"`
	IID             int        `json:"iid"`
	ProjectID       int        `json:"project_id"`
	Title           string     `json:"title"`
	State           string     `json:"state"`
	CreatedAt       *time.Time `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
	SourceBranch    string     `json:"source_branch"`
	TargetBranch    string     `json:"target_branch"`
	SourceProjectID int        `json:"source_project_id"`
	TargetProjectID int        `json:"target_project_id"`
	Description     string     `json:"description"`
	Author          *BasicUser `json:"author"`
	Changes         []Changes  `json:"changes"`
}

type Changes struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	AMode       string `json:"a_mode"`
	BMode       string `json:"b_mode"`
	Diff        string `json:"diff"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

type BasicUser struct {
	ID        int        `json:"id"`
	Username  string     `json:"username"`
	Name      string     `json:"name"`
	State     string     `json:"state"`
	CreatedAt *time.Time `json:"created_at"`
	AvatarURL string     `json:"avatar_url"`
	WebURL    string     `json:"web_url"`
}

func (c *Client) ListOpenedProjectMergeRequests(owner, repo, targetBranch string, log *zap.SugaredLogger) ([]*MergeRequest, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/isource/merge_requests", generateProjectName(owner, repo))
	qs := map[string]string{
		"state":    "opened",
		"per_page": "100",
	}
	if targetBranch != "" {
		qs["target_branch"] = targetBranch
	}

	var err error
	var mergeRequests []*MergeRequest
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&mergeRequests)); err != nil {
		log.Errorf("Failed to list project mergeRequests, error: %s", err)
		return mergeRequests, err
	}

	return mergeRequests, nil
}

func (c *Client) ListChangedFiles(event *MergeEvent) ([]string, error) {
	files := make([]string, 0)

	url := fmt.Sprintf("/api/v4/projects/%d/merge_requests/%d/changes", event.ObjectAttributes.TargetProjectID, event.ObjectAttributes.IID)
	var err error
	var mergeRequest *MergeRequest
	if _, err = c.Get(url, httpclient.SetResult(&mergeRequest)); err != nil {
		log.Errorf("Failed to get project mergeRequest, error: %s", err)
		return files, err
	}

	for _, change := range mergeRequest.Changes {
		files = append(files, change.NewPath)
		files = append(files, change.OldPath)
	}

	return files, nil
}

func (c *Client) GetLatestPRCommitList(projectID string, pr int, log *zap.SugaredLogger) (*Commit, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/commits", projectID, pr)

	var commit *Commit
	if _, err := c.Get(url, httpclient.SetResult(&commit)); err != nil {
		log.Errorf("Failed to get project mergeRequest commits, error: %s", err)
		return commit, err
	}
	return commit, nil
}
