/*
Copyright 2026 The KodeRover Authors.

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

package jira

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestIsJiraCloud(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"acme.atlassian.net", true},
		{"https://acme.atlassian.net", true},
		{"https://acme.atlassian.net:443/", true},
		{"https://ACME.Atlassian.NET", true},
		{"https://api.atlassian.com/ex/jira/11223344-a1b2-c3d4-e5f6-123456abcdef", true},
		{"api.atlassian.com/ex/jira/11223344-a1b2-c3d4-e5f6-123456abcdef", true},
		{"https://api.atlassian.com/oauth/token", false},
		{"https://jira.example.com", false},
		{"http://jira.internal:8080/jira", false},
		{"https://evilatlassian.net", false},
		{"https://atlassian.net.example.com", false},
		{"", false},
		{"://bad host", false},
	}
	for _, c := range cases {
		if got := isJiraCloud(c.host); got != c.want {
			t.Errorf("isJiraCloud(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

// cloudSearch serves /rest/api/2/search/jql with one page per nextPageToken and records the
// tokens it was asked for.
func cloudSearch(t *testing.T, pages map[string]string, tokens *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/search/jql" {
			t.Errorf("path = %s, want /rest/api/2/search/jql", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields"); got != "*navigable" {
			t.Errorf("fields = %q, want *navigable", got)
		}
		token := r.URL.Query().Get("nextPageToken")
		*tokens = append(*tokens, token)
		page, ok := pages[token]
		if !ok {
			http.Error(w, `{"errorMessages":["unknown token"]}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, page)
	}))
}

func issueKeys(issues []*Issue) []string {
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		keys = append(keys, issue.Key)
	}
	return keys
}

func TestSearchByJQLCloudFollowsNextPageTokenUntilIsLast(t *testing.T) {
	var tokens []string
	server := cloudSearch(t, map[string]string{
		"":       `{"issues":[{"key":"HELP-1"},{"key":"HELP-2"}],"nextPageToken":"page-2","isLast":false}`,
		"page-2": `{"issues":[{"key":"HELP-3"}],"nextPageToken":"page-3","isLast":true}`,
	}, &tokens)
	defer server.Close()
	client := NewJiraClientWithBasicAuth(server.URL, "user", "token")

	issues, err := client.Issue.searchByJQLCloud("project = HELP", true)
	if err != nil {
		t.Fatalf("searchByJQLCloud: %v", err)
	}
	if keys := issueKeys(issues); !slices.Equal(keys, []string{"HELP-1", "HELP-2", "HELP-3"}) {
		t.Errorf("issues = %v, want HELP-1, HELP-2 and HELP-3", keys)
	}
	// isLast ends the search even when the last page still carries a nextPageToken.
	if !slices.Equal(tokens, []string{"", "page-2"}) {
		t.Errorf("tokens requested = %q, want the first page and page-2 only", tokens)
	}
}

func TestSearchByJQLCloudStopsWithoutANextPageToken(t *testing.T) {
	var tokens []string
	server := cloudSearch(t, map[string]string{
		"": `{"issues":[{"key":"HELP-1"}]}`,
	}, &tokens)
	defer server.Close()
	client := NewJiraClientWithBasicAuth(server.URL, "user", "token")

	issues, err := client.Issue.searchByJQLCloud("project = HELP", true)
	if err != nil {
		t.Fatalf("searchByJQLCloud: %v", err)
	}
	if keys := issueKeys(issues); !slices.Equal(keys, []string{"HELP-1"}) {
		t.Errorf("issues = %v, want HELP-1", keys)
	}
	if len(tokens) != 1 {
		t.Errorf("requests = %d, want 1", len(tokens))
	}
}

func TestSearchByJQLCloudReadsOnePageUnlessFindAll(t *testing.T) {
	var tokens []string
	server := cloudSearch(t, map[string]string{
		"": `{"issues":[{"key":"HELP-1"}],"nextPageToken":"page-2","isLast":false}`,
	}, &tokens)
	defer server.Close()
	client := NewJiraClientWithBasicAuth(server.URL, "user", "token")

	issues, err := client.Issue.searchByJQLCloud("project = HELP", false)
	if err != nil {
		t.Fatalf("searchByJQLCloud: %v", err)
	}
	if keys := issueKeys(issues); !slices.Equal(keys, []string{"HELP-1"}) {
		t.Errorf("issues = %v, want HELP-1", keys)
	}
	if len(tokens) != 1 {
		t.Errorf("requests = %d, want 1", len(tokens))
	}
}

func TestSearchByJQLCloudReturnsJiraErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["The requested API has been removed."]}`, http.StatusGone)
	}))
	defer server.Close()
	client := NewJiraClientWithBasicAuth(server.URL, "user", "token")

	if _, err := client.Issue.searchByJQLCloud("project = HELP", true); err == nil {
		t.Fatal("searchByJQLCloud on a 410: got no error")
	}
}

func TestSearchByJQLKeepsTheOldSearchOffJiraCloud(t *testing.T) {
	var starts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/search" {
			t.Errorf("path = %s, want /rest/api/2/search on Jira Server and Data Center", r.URL.Path)
		}
		start := r.URL.Query().Get("startAt")
		starts = append(starts, start)
		w.Header().Set("Content-Type", "application/json")
		switch start {
		case "0":
			fmt.Fprint(w, `{"startAt":0,"maxResults":2,"total":3,"issues":[{"key":"HELP-1"},{"key":"HELP-2"}]}`)
		case "2":
			fmt.Fprint(w, `{"startAt":2,"maxResults":2,"total":3,"issues":[{"key":"HELP-3"}]}`)
		default:
			http.Error(w, "unexpected startAt", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	// httptest serves on 127.0.0.1, which is no Jira Cloud site.
	client := NewJiraClientWithBasicAuth(server.URL, "user", "token")

	issues, err := client.Issue.SearchByJQL("project = HELP", true)
	if err != nil {
		t.Fatalf("SearchByJQL: %v", err)
	}
	if keys := issueKeys(issues); !slices.Equal(keys, []string{"HELP-1", "HELP-2", "HELP-3"}) {
		t.Errorf("issues = %v, want HELP-1, HELP-2 and HELP-3", keys)
	}
	if !slices.Equal(starts, []string{"0", "2"}) {
		t.Errorf("startAt requested = %q, want 0 and 2", starts)
	}
}
