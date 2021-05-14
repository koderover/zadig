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

package types

import (
	"fmt"
	"regexp"
	"time"
)

// Repository struct
type Repository struct {
	// Source is github, gitlab
	Source        string `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner     string `bson:"repo_owner"                json:"repo_owner"`
	RepoName      string `bson:"repo_name"                 json:"repo_name"`
	RemoteName    string `bson:"remote_name,omitempty"     json:"remote_name,omitempty"`
	Branch        string `bson:"branch"                    json:"branch"`
	PR            int    `bson:"pr,omitempty"              json:"pr,omitempty"`
	Tag           string `bson:"tag,omitempty"             json:"tag,omitempty"`
	CommitID      string `bson:"commit_id,omitempty"       json:"commit_id,omitempty"`
	CommitMessage string `bson:"commit_message,omitempty"  json:"commit_message,omitempty"`
	CheckoutPath  string `bson:"checkout_path,omitempty"   json:"checkout_path,omitempty"`
	SubModules    bool   `bson:"submodules,omitempty"      json:"submodules,omitempty"`
	// UseDefault defines if the repo can be configured in start pipeline task page
	UseDefault bool `bson:"use_default,omitempty"          json:"use_default,omitempty"`
	// IsPrimary used to generated image and package name, each build has one primary repo
	IsPrimary  bool `bson:"is_primary"                     json:"is_primary"`
	CodehostID int  `bson:"codehost_id"                    json:"codehost_id"`
	// add
	OauthToken  string `bson:"oauth_token"                  json:"oauth_token"`
	Address     string `bson:"address"                      json:"address"`
	AuthorName  string `bson:"author_name,omitempty"        json:"author_name,omitempty"`
	CheckoutRef string `bson:"checkout_ref,omitempty"       json:"checkout_ref,omitempty"`
}

// GetReleaseCandidateTag 返回待发布对象Tag
// Branch: 20060102150405-{TaskID}-master
// PR: 20060102150405-{TaskID}-pr-1765
// Branch + PR: 20060102150405-{TaskID}-master-pr-1276
// Tag: 20060102150405-{TaskID}-v0.9.1
func (repo *Repository) GetReleaseCandidateTag(taskID int64) string {

	var tag string

	timeStamp := time.Now().Format("20060102150405")

	if repo.Tag != "" {
		tag = fmt.Sprintf("%s-%d-%s", timeStamp, taskID, repo.Tag)
	} else if repo.Branch != "" && repo.PR != 0 {
		tag = fmt.Sprintf("%s-%d-%s-pr-%d", timeStamp, taskID, repo.Branch, repo.PR)
	} else if repo.Branch == "" && repo.PR != 0 {
		tag = fmt.Sprintf("%s-%d-pr-%d", timeStamp, taskID, repo.PR)
	} else if repo.Branch != "" && repo.PR == 0 {
		tag = fmt.Sprintf("%s-%d-%s", timeStamp, taskID, repo.Branch)
	} else {
		return "invalid"
	}

	// 验证tag是否符合kube的image tag规则
	reg := regexp.MustCompile(`[^\w.-]`)
	tagByte := reg.ReplaceAll([]byte(tag), []byte("-"))
	tag = string(tagByte)
	if len(tag) > 127 {
		return "invalid"
	}
	return tag
}
