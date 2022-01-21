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

package git

const (
	PushEvent              = "push"
	PullRequestEvent       = "pull_request"
	CheckRunEvent          = "check_run"
	BranchOrTagCreateEvent = "create"
)

type Hook struct {
	URL    string   `json:"url,omitempty"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events,omitempty"`
	Active *bool    `json:"active,omitempty"`
}
