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

type ProjectHook struct {
	ID                       int        `json:"id"`
	URL                      string     `json:"url"`
	ConfidentialNoteEvents   bool       `json:"confidential_note_events"`
	ProjectID                int        `json:"project_id"`
	PushEvents               bool       `json:"push_events"`
	PushEventsBranchFilter   string     `json:"push_events_branch_filter"`
	IssuesEvents             bool       `json:"issues_events"`
	ConfidentialIssuesEvents bool       `json:"confidential_issues_events"`
	MergeRequestsEvents      bool       `json:"merge_requests_events"`
	TagPushEvents            bool       `json:"tag_push_events"`
	NoteEvents               bool       `json:"note_events"`
	JobEvents                bool       `json:"job_events"`
	PipelineEvents           bool       `json:"pipeline_events"`
	WikiPageEvents           bool       `json:"wiki_page_events"`
	DeploymentEvents         bool       `json:"deployment_events"`
	ReleasesEvents           bool       `json:"releases_events"`
	EnableSSLVerification    bool       `json:"enable_ssl_verification"`
	CreatedAt                *time.Time `json:"created_at"`
}

type Hook struct {
	URL                   string `url:"url,omitempty" json:"url,omitempty"`
	Name                  string `url:"name,omitempty" json:"name,omitempty"`
	Token                 string `url:"token,omitempty" json:"token,omitempty"`
	PushEvents            bool   `url:"push_events,omitempty" json:"push_events,omitempty"`
	MergeRequestsEvents   bool   `url:"merge_requests_events,omitempty" json:"merge_requests_events,omitempty"`
	TagPushEvents         bool   `url:"tag_push_events,omitempty" json:"tag_push_events,omitempty"`
	EnableSSLVerification bool   `url:"enable_ssl_verification,omitempty" json:"enable_ssl_verification,omitempty"`
}

func (c *Client) ListProjectHooks(owner, repo string, log *zap.SugaredLogger) ([]*ProjectHook, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/hooks", generateProjectName(owner, repo))
	qs := map[string]string{
		"per_page": "100",
	}

	var err error
	var projectHooks []*ProjectHook
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&projectHooks)); err != nil {
		log.Errorf("Failed to list project hooks, error: %s", err)
		return projectHooks, err
	}

	return projectHooks, nil
}

func (c *Client) AddProjectHook(owner, repo, hookURL, token string) (*ProjectHook, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/hooks", generateProjectName(owner, repo))
	hook := &Hook{
		URL:                 hookURL,
		Token:               token,
		MergeRequestsEvents: true,
		PushEvents:          true,
		TagPushEvents:       true,
	}
	var projectHook *ProjectHook
	if _, err := c.Post(url, httpclient.SetBody(hook), httpclient.SetResult(&projectHook)); err != nil {
		log.Errorf("Failed to create project hook, error: %s", err)
		return projectHook, err
	}
	return projectHook, nil
}

func (c *Client) DeleteProjectHook(owner, repo string, id int) error {
	url := fmt.Sprintf("/api/v4/projects/%s/hooks/%d", generateProjectName(owner, repo), id)
	if _, err := c.Delete(url); err != nil {
		log.Errorf("Failed to delete project webhook, error: %s", err)
		return err
	}
	return nil
}
