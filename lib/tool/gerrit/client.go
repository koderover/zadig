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

package gerrit

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/andygrunwald/go-gerrit"
)

const refHeader = "refs/heads/"

type Client struct {
	cli *gerrit.Client
}

func NewClient(address, accessToken string) *Client {
	httpClient := &http.Client{Transport: &BasicAuthTransporter{EncodedUserPass: accessToken}}
	cli, _ := gerrit.NewClient(address+"/a", httpClient)
	return &Client{cli: cli}
}

func (c *Client) ListProjects() ([]*gerrit.ProjectInfo, error) {
	return c.ListProjectsByKey("")
}

func (c *Client) ListProjectsByKey(keyword string) ([]*gerrit.ProjectInfo, error) {
	opts := &gerrit.ProjectOptions{
		Tree: false,
		Type: "CODE",
		ProjectBaseOptions: gerrit.ProjectBaseOptions{
			Limit: 100,
		},
	}

	if keyword != "" {
		opts.Substring = keyword
	}

	resp, _, err := c.cli.Projects.ListProjects(opts)

	if err != nil {
		return nil, err
	}

	var result []*gerrit.ProjectInfo

	for _, pi := range *resp {
		p := pi
		result = append(result, &p)
	}

	return result, nil
}

func (c *Client) ListBranches(project string) ([]string, error) {
	project = Unescape(project)
	resp, _, err := c.cli.Projects.ListBranches(project, &gerrit.BranchOptions{
		Limit: 100,
	})
	if err != nil {
		return nil, err
	}

	result := make([]string, 0)

	for _, branch := range *resp {
		if strings.HasPrefix(branch.Ref, refHeader) {
			name := branch.Ref[len(refHeader):]
			result = append(result, name)
		}
	}

	return result, nil
}

const tagPrefix = "refs/tags/"

func (c *Client) ListTags(project string) (result []*gerrit.TagInfo, err error) {
	project = Unescape(project)
	resp, _, err := c.cli.Projects.ListTags(project, &gerrit.ProjectBaseOptions{})
	if err != nil {
		return nil, err
	}

	for _, tag := range *resp {
		t := tag
		if strings.HasPrefix(t.Ref, tagPrefix) {
			t.Ref = t.Ref[len(tagPrefix):]
			result = append(result, &t)
		}
	}

	return result, nil
}

func (c *Client) GetCommitByBranch(project string, branch string) (*gerrit.CommitInfo, error) {
	var revision string
	project = Unescape(project)
	branchInfo, _, err := c.cli.Projects.GetBranch(project, branch)
	if err != nil {
		return nil, err
	} else {
		revision = branchInfo.Revision
	}

	commit, _, err := c.cli.Projects.GetCommit(project, revision)
	if err != nil {
		return nil, err
	}

	return commit, err
}

func (c *Client) GetCommitByTag(project string, tag string) (*gerrit.CommitInfo, error) {
	var revision string
	project = Unescape(project)
	tagInfo, _, err := c.cli.Projects.GetTag(project, tag)
	if err != nil {
		return nil, err
	} else {
		revision = tagInfo.Revision
	}

	commit, _, err := c.cli.Projects.GetCommit(project, revision)
	if err != nil {
		return nil, err
	}

	return commit, err
}

func (c *Client) GetCommit(project string, branch string) (*gerrit.CommitInfo, error) {
	var revision string
	project = Unescape(project)
	branchInfo, _, err := c.cli.Projects.GetBranch(project, branch)
	if err != nil {
		// try to get tag
		tagInfo, _, err := c.cli.Projects.GetTag(project, branch)
		if err != nil {
			return nil, err
		}
		revision = tagInfo.Revision
	} else {
		revision = branchInfo.Revision
	}

	commit, _, err := c.cli.Projects.GetCommit(project, revision)
	if err != nil {
		return nil, err
	}

	return commit, err
}

func (c *Client) GetCurrentVersionByChangeId(name string, pr int) (*gerrit.ChangeInfo, error) {
	name = Unescape(name)
	info, _, err := c.cli.Changes.GetChangeDetail(
		fmt.Sprintf("%s~%d", url.QueryEscape(name), pr),
		&gerrit.ChangeOptions{AdditionalFields: []string{"CURRENT_REVISION"}})
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (c *Client) SetReview(projectName string, changeId int, m, label, score, revision string) error {
	projectName = Unescape(projectName)
	var labels map[string]string
	if len(label) != 0 {
		labels = map[string]string{
			label: score,
		}
	}
	_, _, err := c.cli.Changes.SetReview(
		fmt.Sprintf("%s~%d", url.QueryEscape(projectName), changeId),
		revision,
		&gerrit.ReviewInput{
			Message:      m,
			Labels:       labels,
			StrictLabels: false,
		},
	)

	return err
}

// CompareTwoPatchset 如果两个Patchset更新的内容相同，返回true，不相同则返回false
func (c *Client) CompareTwoPatchset(changeId, newPatchSetID, oldPatchSetID string) (bool, error) {
	newPatchSetChangeFiles, _, err := c.cli.Changes.ListFiles(changeId, newPatchSetID)
	if err != nil || newPatchSetChangeFiles == nil {
		return false, err
	}
	oldPatchSetChangeFiles, _, err := c.cli.Changes.ListFiles(changeId, oldPatchSetID)
	if err != nil || oldPatchSetChangeFiles == nil {
		return false, err
	}
	newChangeFiles := *newPatchSetChangeFiles
	oldChangeFiles := *oldPatchSetChangeFiles
	if len(newChangeFiles) != len(oldChangeFiles) {
		return false, nil
	}
	for fileName := range newChangeFiles {
		if _, ok := oldChangeFiles[fileName]; !ok {
			return false, nil
		}
	}
	for fileName := range oldChangeFiles {
		if _, ok := newChangeFiles[fileName]; !ok {
			return false, nil
		}
	}

	for fileName, newFileInfo := range newChangeFiles {
		// 只更新comment时，/COMMIT_MSG的信息也会改变，忽略
		if fileName == "/COMMIT_MSG" {
			continue
		}
		oldFileInfo := oldChangeFiles[fileName]

		if newFileInfo.LinesInserted != oldFileInfo.LinesInserted || newFileInfo.LinesDeleted != oldFileInfo.LinesDeleted {
			return true, nil
		}
	}

	return false, nil
}

type BasicAuthTransporter struct {
	EncodedUserPass string
}

func (bt *BasicAuthTransporter) RoundTrip(req *http.Request) (*http.Response, error) {
	auth := "Basic " + bt.EncodedUserPass
	req.Header.Set("Authorization", auth)
	return http.DefaultTransport.RoundTrip(req)
}

var backslash = regexp.MustCompile("%2[F|f]")

func Unescape(id string) string {
	return backslash.ReplaceAllString(id, "/")
}

func Escape(name string) string {
	return strings.ReplaceAll(name, "/", "%2F")
}

const (
	DefaultNamespace   = "default"
	CodehostTypeGerrit = "gerrit"
	RemoteName         = "koderover"
	TimeFormat         = "2006-01-02 15:04:05.999999999"
)

type GerritWebhook struct {
	URL       string `json:"url"`
	MaxTries  int    `json:"max_tries"`
	SslVerify bool   `json:"ssl_verify"`
	//Events    []pipeline.HookEventType `json:"events"`
	Events []string `json:"events"`
}

func Do(url, method, accessToken string, reader io.Reader) ([]byte, error) {
	auth := "Basic " + accessToken
	header := http.Header{}
	header.Set("Authorization", auth)
	header.Set("Content-Type", "application/json; charset=UTF-8")

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header = header
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 == 2 {
		return body, nil
	}

	return nil, fmt.Errorf("response status error: %s %s %v %d", url, string(body), resp.Header, resp.StatusCode)
}
