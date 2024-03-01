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

package taskcontroller

// HookPayload reference. Ref can be a SHA, a branch name, or a tag name.
type HookPayload struct {
	Owner string `json:"owner,omitempty"`
	Repo  string `json:"repo,omitempty"`
	Ref   string `json:"ref,omitempty"`
	IsPr  bool   `json:"is_pr,omitempty"`
}

// CancelMessage ...
type CancelMessage struct {
	Revoker      string `json:"revoker"`
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
	ReqID        string `json:"req_id"`
}

// StatsMessage ...
type StatsMessage struct {
	Agent            string `json:"agent"`
	MessagesReceived uint64 `json:"messages_received"`
	MessagesFinished uint64 `json:"messages_finished"`
	MessagesRequeued uint64 `json:"messages_requeued"`
	Connections      int    `json:"connections"`
	StartTime        int64  `json:"start_time"`
	UpdateTime       int64  `json:"update_time"`
}
