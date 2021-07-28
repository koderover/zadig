package codehub

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	PushEvents             = "push_events"
	PullRequestEvent       = "merge_requests_events"
	BranchOrTagCreateEvent = "tag_push_events"
)

type AddCodehubHookPayload struct {
	HookURL    string   `json:"hook_url"`
	Service    string   `json:"service"`
	Token      string   `json:"token"`
	HookEvents []string `json:"hook_events"`
}

type AddCodehubHookResp struct {
	Result CodehubHook `json:"result"`
	Status string      `json:"status"`
}

type CodehubHook struct {
	ID                     int    `json:"id"`
	ProjectID              int    `json:"project_id"`
	CreatedAt              string `json:"created_at"`
	EnableSslVerification  bool   `json:"enable_ssl_verification"`
	PushEvents             bool   `json:"push_events"`
	TagPushEvents          bool   `json:"tag_push_events"`
	RepositoryUpdateEvents bool   `json:"repository_update_events"`
	MergeRequestsEvents    bool   `json:"merge_requests_events"`
	IssuesEvents           bool   `json:"issues_events"`
	NoteEvents             bool   `json:"note_events"`
	PipelineEvents         bool   `json:"pipeline_events"`
	WikiPageEvents         bool   `json:"wiki_page_events"`
}

type GetCodehubHookResp struct {
	Result GetCodehubHookResult `json:"result"`
	Status string               `json:"status"`
}

type GetCodehubHookResult struct {
	Hooks []CodehubHook `json:"hooks"`
}

type DeleteCodehubWebhookResp struct {
	Result string `json:"result"`
	Status string `json:"status"`
}

func (c *CodeHubClient) AddWebhook(repoOwner, repoName string, codehubHookPayload *AddCodehubHookPayload) (string, error) {
	payload, err := json.Marshal(codehubHookPayload)
	if err != nil {
		return "", err
	}
	body, err := c.sendRequest("POST", fmt.Sprintf("/v1/repositories/%s/%s/hooks", repoOwner, repoName), payload)
	if err != nil {
		return "", err
	}
	defer body.Close()

	addCodehubHookResp := new(AddCodehubHookResp)
	if err = json.NewDecoder(body).Decode(addCodehubHookResp); err != nil {
		return "", err
	}
	if addCodehubHookResp.Status == "success" {
		return strconv.Itoa(addCodehubHookResp.Result.ID), nil
	}

	return "", fmt.Errorf("add codehub webhook failed")
}

func (c *CodeHubClient) ListCodehubWebhooks(repoOwner, repoName, hookID string) ([]CodehubHook, error) {
	var codehubHooks []CodehubHook

	url := fmt.Sprintf("/v1/repositories/%s/%s/hooks?hook_id=%s", repoOwner, repoName, hookID)
	body, err := c.sendRequest("GET", url, []byte{})
	if err != nil {
		return codehubHooks, err
	}
	defer body.Close()

	codehubHookResp := new(GetCodehubHookResp)
	if err = json.NewDecoder(body).Decode(codehubHookResp); err != nil {
		return codehubHooks, err
	}
	if codehubHookResp.Status == "success" {
		return codehubHookResp.Result.Hooks, nil
	}
	return codehubHooks, fmt.Errorf("get codehub webhooks failed")
}

func (c *CodeHubClient) DeleteCodehubWebhook(repoOwner, repoName, hookID string) error {
	url := fmt.Sprintf("/v1/repositories/%s/%s/hooks/%s", repoOwner, repoName, hookID)
	body, err := c.sendRequest("DELETE", url, []byte{})
	if err != nil {
		return err
	}
	defer body.Close()

	deleteCodehubWebhookResp := new(DeleteCodehubWebhookResp)
	if err = json.NewDecoder(body).Decode(deleteCodehubWebhookResp); err != nil {
		return err
	}
	if deleteCodehubWebhookResp.Status == "success" {
		return nil
	}
	return fmt.Errorf("delete codehub webhook [%s] failed", hookID)
}
