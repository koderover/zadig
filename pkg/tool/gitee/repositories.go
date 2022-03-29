package gitee

import (
	"context"
	"fmt"
	"time"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/koderover/zadig/pkg/tool/git"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

const (
	GiteeHOSTURL = "https://gitee.com/api"
)

type Project struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

func (c *Client) ListRepositoriesForAuthenticatedUser(ctx context.Context, accessToken string) ([]Project, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(GiteeHOSTURL),
	)
	url := "/v5/user/repos"
	queryParams := make(map[string]string)
	queryParams["access_token"] = accessToken
	queryParams["visibility"] = "all"
	queryParams["affiliation"] = "admin"
	queryParams["per_page"] = "100"

	var projects []Project
	_, err := httpClient.Get(url, httpclient.SetQueryParams(queryParams), httpclient.SetResult(&projects))
	if err != nil {
		return nil, err
	}

	return projects, nil
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

func (c *Client) CreateHook(ctx context.Context, owner, repo string, hook *git.Hook) (gitee.Hook, error) {
	fmt.Println(fmt.Sprintf("owner:%s", owner))
	fmt.Println(fmt.Sprintf("repo:%s", repo))
	fmt.Println(fmt.Sprintf("hook.secret:%s", hook.Secret))
	fmt.Println(fmt.Sprintf("hook.url:%s", hook.URL))
	resp, _, err := c.WebhooksApi.PostV5ReposOwnerRepoHooks(ctx, owner, repo, hook.URL, &gitee.PostV5ReposOwnerRepoHooksOpts{
		Password:            optional.NewString(hook.Secret),
		PushEvents:          optional.NewBool(true),
		TagPushEvents:       optional.NewBool(true),
		MergeRequestsEvents: optional.NewBool(true),
	})
	if err != nil {
		fmt.Println(fmt.Sprintf("PostV5ReposOwnerRepoHooks err:%+v", err))
		return gitee.Hook{}, err
	}

	return resp, nil
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
		return gitee.Blob{}, err
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
	URL    string `json:"url"`
	Sha    string `json:"sha"`
	Commit struct {
		Author struct {
			Name  string    `json:"name"`
			Date  time.Time `json:"date"`
			Email string    `json:"email"`
		} `json:"author"`
		Committer struct {
			Name  string    `json:"name"`
			Date  time.Time `json:"date"`
			Email string    `json:"email"`
		} `json:"committer"`
		Message string `json:"message"`
	} `json:"commit"`
}

func (c *Client) GetSingleCommitOfProject(ctx context.Context, accessToken, owner, repo, commitSha string) (*RepoCommit, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(GiteeHOSTURL),
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

func RefreshAccessToken(refreshToken string) (*AccessToken, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL("https://gitee.com"),
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
