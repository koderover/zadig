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

package webhook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-github/v35/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGitlabEmojiHookFeedback(t *testing.T) {
	// Reduced from GitLab's Emoji Hook delivery: event_type is a top-level field.
	payload := `{"object_kind":"emoji","event_type":"award","merge_request":{"iid":108},"project_id":32336969,"project":{"web_url":"https://gitlab.com/kr-test-org1/multi-service-demo"},"object_attributes":{"id":57677299,"name":"thumbsup","awardable_type":"Note","awardable_id":3914791402},"note":{"noteable_type":"MergeRequest"}}`
	for _, tt := range []struct {
		name, payload string
		want          bool
	}{
		{"award", payload, true},
		{"revoke", strings.Replace(payload, `"event_type":"award"`, `"event_type":"revoke"`, 1), true},
		{"thumbsdown", strings.Replace(payload, `"thumbsup"`, `"thumbsdown"`, 1), true},
		{"missing event type", strings.Replace(payload, `"event_type":"award",`, "", 1), false},
		{"issue note", strings.Replace(payload, `"MergeRequest"`, `"Issue"`, 1), false},
		{"other emoji", strings.Replace(payload, `"thumbsup"`, `"smile"`, 1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var event gitlabEmojiHook
			if err := json.Unmarshal([]byte(tt.payload), &event); err != nil {
				t.Fatal(err)
			}
			if got := event.isReviewFeedback(); got != tt.want {
				t.Fatalf("isReviewFeedback() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "webhook Suite")
}

func TestShouldTriggerByGithubPullRequestEvent(t *testing.T) {
	tests := []struct {
		name   string
		event  *github.PullRequestEvent
		wanted bool
	}{
		{
			name:   "opened pull request",
			event:  &github.PullRequestEvent{Action: github.String("opened")},
			wanted: true,
		},
		{
			name:   "synchronized pull request",
			event:  &github.PullRequestEvent{Action: github.String("synchronize")},
			wanted: true,
		},
		{
			name:  "edited pull request",
			event: &github.PullRequestEvent{Action: github.String("edited")},
		},
		{
			name:  "labeled pull request",
			event: &github.PullRequestEvent{Action: github.String("labeled")},
		},
		{
			name:  "reopened pull request",
			event: &github.PullRequestEvent{Action: github.String("reopened")},
		},
		{
			name:  "missing action",
			event: &github.PullRequestEvent{},
		},
		{
			name: "nil event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldTriggerByGithubPullRequestEvent(tt.event); got != tt.wanted {
				t.Errorf("shouldTriggerByGithubPullRequestEvent() = %t, want %t", got, tt.wanted)
			}
		})
	}
}
