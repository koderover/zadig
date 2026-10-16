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
	"testing"

	"github.com/google/go-github/v35/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
