/*
Copyright 2022 The KodeRover Authors.

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

package gitee

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/koderover/zadig/v2/pkg/tool/git"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

type Project struct {
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	Path          string         `json:"path"`
	DefaultBranch string         `json:"default_branch,omitempty"`
	Namespace     *NamespaceInfo `json:"namespace,omitempty"`
}

type NamespaceInfo struct {
	Parent *ParentInfo `json:"parent"`
	Type   string      `json:"type"`
	Path   string      `json:"path"`
}

type ParentInfo struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

const (
	// fixedPagination is used to fix gitee's recent change to the API
	// It is a temporary solution for now since the webpage of zadig does not have pagination functionality for now
	fixedPagination = 100
)

func (c *Client) ListRepositoriesForAuthenticatedUser(hostURL, accessToken, keyword string, page, perPage int) ([]Project, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)
	// api reference: https://gitee.com/api/v5/swagger#/getV5UserRepos
	url := "/v5/user/repos"
	queryParams := make(map[string]string)
	queryParams["access_token"] = accessToken
	//queryParams["visibility"] = "all"
	//queryParams["affiliation"] = "owner"
	queryParams["type"] = "personal"
	queryParams["q"] = keyword
	queryParams["page"] = strconv.Itoa(page)
	queryParams["per_page"] = strconv.Itoa(fixedPagination)

	var projects []Project
	_, err := httpClient.Get(url, httpclient.SetQueryParams(queryParams), httpclient.SetResult(&projects))
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (c *Client) ListRepositoryForEnterprise(hostURL, accessToken, enterprise, keyword string, page, perPage int) ([]Project, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)

	// enterprise api reference: https://gitee.com/api/v5/swagger#/getV5EnterprisesEnterpriseRepos
	url := fmt.Sprintf("/v5/enterprises/%s/repos", enterprise)
	queryParams := make(map[string]string)
	queryParams["access_token"] = accessToken
	queryParams["type"] = "all"
	queryParams["search"] = keyword
	queryParams["page"] = strconv.Itoa(page)
	queryParams["per_page"] = strconv.Itoa(fixedPagination)
	queryParams["direct"] = "false"

	var projects []Project
	_, err := httpClient.Get(url, httpclient.SetQueryParams(queryParams), httpclient.SetResult(&projects))
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (c *Client) ListRepositoriesForOrg(hostURL, accessToken, org string, page, perPage int) ([]Project, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)

	// api reference: https://gitee.com/api/v5/swagger#/getV5OrgsOrgRepos
	url := fmt.Sprintf("/v5/orgs/%s/repos", org)
	queryParams := make(map[string]string)
	queryParams["access_token"] = accessToken
	queryParams["type"] = "all"
	queryParams["page"] = strconv.Itoa(page)
	queryParams["per_page"] = strconv.Itoa(fixedPagination)
	queryParams["direct"] = "false"

	var projects []Project
	_, err := httpClient.Get(url, httpclient.SetQueryParams(queryParams), httpclient.SetResult(&projects))
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (c *Client) GetRepositoryDetail(hostURL, accessToken, owner, repo string) (*Project, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)

	// api reference: http://gitee.com/api/v5/swagger#/getV5ReposOwnerRepo
	url := fmt.Sprintf("/v5/repos/%s/%s", owner, repo)
	queryParams := make(map[string]string)
	queryParams["access_token"] = accessToken

	var projectDetail Project
	_, err := httpClient.Get(url, httpclient.SetQueryParams(queryParams), httpclient.SetResult(&projectDetail))
	if err != nil {
		return nil, err
	}

	return &projectDetail, nil
}

func (c *Client) ListHooks(ctx context.Context, owner, repo string, opts *gitee.GetV5ReposOwnerRepoHooksOpts) ([]gitee.Hook, error) {
	hs, _, err := c.WebhooksApi.GetV5ReposOwnerRepoHooks(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}
	return hs, nil
}

func (c *Client) DeleteHook(ctx context.Context, owner, repo string, id int64) error {
	_, err := c.WebhooksApi.DeleteV5ReposOwnerRepoHooksId(ctx, owner, repo, int32(id), nil)
	if err != nil {
		return err
	}
	return nil
}

type Hook struct {
	ID                  int       `json:"id"`
	URL                 string    `json:"url"`
	CreatedAt           time.Time `json:"created_at"`
	Password            string    `json:"password"`
	ProjectID           int       `json:"project_id"`
	Result              string    `json:"result"`
	ResultCode          int       `json:"result_code"`
	PushEvents          bool      `json:"push_events"`
	TagPushEvents       bool      `json:"tag_push_events"`
	IssuesEvents        bool      `json:"issues_events"`
	NoteEvents          bool      `json:"note_events"`
	MergeRequestsEvents bool      `json:"merge_requests_events"`
}

func (c *Client) CreateHook(hostURL, accessToken, owner, repo string, hook *git.Hook) (*Hook, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)
	url := fmt.Sprintf("/v5/repos/%s/%s/hooks", owner, repo)
	var hookInfo *Hook
	_, err := httpClient.Post(url, httpclient.SetBody(struct {
		AccessToken         string `json:"access_token"`
		URL                 string `json:"url"`
		Password            string `json:"password"`
		PushEvents          string `json:"push_events"`
		TagPushEvents       string `json:"tag_push_events"`
		MergeRequestsEvents string `json:"merge_requests_events"`
	}{accessToken, hook.URL, hook.Secret, "true", "true", "true"}), httpclient.SetResult(&hookInfo))
	if err != nil {
		return nil, err
	}
	return hookInfo, nil
}

func (c *Client) UpdateHook(ctx context.Context, owner, repo string, id int64, hook *git.Hook) (gitee.Hook, error) {
	resp, _, err := c.WebhooksApi.PatchV5ReposOwnerRepoHooksId(ctx, owner, repo, int32(id), hook.URL, &gitee.PatchV5ReposOwnerRepoHooksIdOpts{
		Password:            optional.NewString(hook.Secret),
		PushEvents:          optional.NewBool(true),
		TagPushEvents:       optional.NewBool(true),
		MergeRequestsEvents: optional.NewBool(true),
	})
	if err != nil {
		return gitee.Hook{}, err
	}

	return resp, nil
}

func (c *Client) GetContents(ctx context.Context, owner, repo, sha string) (gitee.Blob, error) {
	fileContent, _, err := c.GitDataApi.GetV5ReposOwnerRepoGitBlobsSha(ctx, owner, repo, sha, &gitee.GetV5ReposOwnerRepoGitBlobsShaOpts{})
	if err != nil {
		return fileContent, err
	}
	return fileContent, nil
}

// "Recursive" Assign a value of 1 to get the directory recursively
// sha Can be a branch name (such as master), Commit, or the SHA value of the directory Tree
func (c *Client) GetTrees(ctx context.Context, owner, repo, sha string, level int) (gitee.Tree, error) {
	tree, _, err := c.GitDataApi.GetV5ReposOwnerRepoGitTreesSha(ctx, owner, repo, sha, &gitee.GetV5ReposOwnerRepoGitTreesShaOpts{
		Recursive: optional.NewInt32(int32(level)),
	})
	if err != nil {
		return gitee.Tree{}, err
	}
	return tree, nil
}

type RepoCommit struct {
	URL    string                `json:"url"`
	Sha    string                `json:"sha"`
	Commit RepoCommitInnerCommit `json:"commit"`
}

type RepoCommitInnerCommit struct {
	Author    RepoCommitAuthor `json:"author"`
	Committer RepoCommitAuthor `json:"committer"`
	Message   string           `json:"message"`
}

type RepoCommitAuthor struct {
	Name  string    `json:"name"`
	Date  time.Time `json:"date"`
	Email string    `json:"email"`
}

func (c *Client) GetSingleCommitOfProject(ctx context.Context, hostURL, accessToken, owner, repo, commitSha string) (*RepoCommit, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)
	url := fmt.Sprintf("/v5/repos/%s/%s/commits/%s", owner, repo, commitSha)

	var commit *RepoCommit
	_, err := httpClient.Get(url, httpclient.SetQueryParam("access_token", accessToken), httpclient.SetResult(&commit))
	if err != nil {
		return nil, err
	}
	return commit, nil
}

type AccessToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	CreatedAt    int    `json:"created_at"`
}

func RefreshAccessToken(baseAddr, refreshToken string) (*AccessToken, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(baseAddr),
	)
	url := "/oauth/token"
	queryParams := make(map[string]string)
	queryParams["grant_type"] = "refresh_token"
	queryParams["refresh_token"] = refreshToken

	var accessToken *AccessToken
	_, err := httpClient.Post(url, httpclient.SetQueryParams(queryParams), httpclient.SetResult(&accessToken))
	if err != nil {
		return nil, err
	}

	return accessToken, nil
}

type Compare struct {
	Commits []CompareCommit `json:"commits"`
	Files   []CompareFiles  `json:"files"`
}

type CompareCommit struct {
	URL         string             `json:"url"`
	Sha         string             `json:"sha"`
	HTMLURL     string             `json:"html_url"`
	CommentsURL string             `json:"comments_url"`
	Commit      CompareInnerCommit `json:"commit"`
	Author      CompareAuthor      `json:"author"`
	Committer   CompareCommitter   `json:"committer"`
	Parents     []CompareParents   `json:"parents"`
}

type CompareParents struct {
	Sha string `json:"sha"`
	URL string `json:"url"`
}

type CompareInnerCommit struct {
	Author    RepoCommitAuthor `json:"author"`
	Committer RepoCommitAuthor `json:"committer"`
	Message   string           `json:"message"`
	Tree      CompareTree      `json:"tree"`
}

type CompareTree struct {
	Sha string `json:"sha"`
	URL string `json:"url"`
}

type CompareFiles struct {
	Sha        string `json:"sha"`
	Filename   string `json:"filename"`
	Status     string `json:"status"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	Changes    int    `json:"changes"`
	BlobURL    string `json:"blob_url"`
	RawURL     string `json:"raw_url"`
	ContentURL string `json:"content_url"`
	Patch      string `json:"patch"`
}

type CompareCommitter struct {
	ID                int    `json:"id"`
	Login             string `json:"login"`
	Name              string `json:"name"`
	AvatarURL         string `json:"avatar_url"`
	URL               string `json:"url"`
	HTMLURL           string `json:"html_url"`
	Remark            string `json:"remark"`
	FollowersURL      string `json:"followers_url"`
	FollowingURL      string `json:"following_url"`
	GistsURL          string `json:"gists_url"`
	StarredURL        string `json:"starred_url"`
	SubscriptionsURL  string `json:"subscriptions_url"`
	OrganizationsURL  string `json:"organizations_url"`
	ReposURL          string `json:"repos_url"`
	EventsURL         string `json:"events_url"`
	ReceivedEventsURL string `json:"received_events_url"`
	Type              string `json:"type"`
}

type CompareAuthor struct {
	ID                int    `json:"id"`
	Login             string `json:"login"`
	Name              string `json:"name"`
	AvatarURL         string `json:"avatar_url"`
	URL               string `json:"url"`
	HTMLURL           string `json:"html_url"`
	Remark            string `json:"remark"`
	FollowersURL      string `json:"followers_url"`
	FollowingURL      string `json:"following_url"`
	GistsURL          string `json:"gists_url"`
	StarredURL        string `json:"starred_url"`
	SubscriptionsURL  string `json:"subscriptions_url"`
	OrganizationsURL  string `json:"organizations_url"`
	ReposURL          string `json:"repos_url"`
	EventsURL         string `json:"events_url"`
	ReceivedEventsURL string `json:"received_events_url"`
	Type              string `json:"type"`
}

func (c *Client) GetReposOwnerRepoCompareBaseHead(hostURL, accessToken, owner string, repo string, base string, head string) (*Compare, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)
	url := fmt.Sprintf("/v5/repos/%s/%s/compare/%s...%s", owner, repo, base, head)

	var compare *Compare
	_, err := httpClient.Get(url, httpclient.SetQueryParam("access_token", accessToken), httpclient.SetResult(&compare))
	if err != nil {
		return nil, err
	}
	return compare, nil
}

func (c *Client) GetReposOwnerRepoCompareBaseHeadForEnterprise(hostURL, accessToken, owner string, repo string, base string, head string) (*Compare, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)
	repoDetail, err := c.GetRepositoryDetail(hostURL, accessToken, owner, repo)
	if err != nil {
		return nil, err
	}

	enterpriseName := ""
	if repoDetail.Namespace.Type == "enterprise" {
		enterpriseName = repoDetail.Namespace.Path
	} else {
		// else this belongs to a group, thus getting it from repoDetail.Namespace.Parent.Path
		enterpriseName = repoDetail.Namespace.Parent.Path
	}

	url := fmt.Sprintf("/v5/repos/%s/%s/%s/compare/%s...%s", enterpriseName, owner, repo, base, head)

	var compare *Compare
	_, err = httpClient.Get(url, httpclient.SetQueryParam("access_token", accessToken), httpclient.SetResult(&compare))
	if err != nil {
		return nil, err
	}
	return compare, nil
}
