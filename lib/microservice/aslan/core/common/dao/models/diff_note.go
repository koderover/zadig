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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DiffNote struct {
	ObjectID       primitive.ObjectID `bson:"_id,omitempty"            json:"_id"`
	Repo           *RepoInfo          `bson:"repo"                     json:"repo"`
	MergeRequestId int                `bson:"merge_request_id"         json:"merge_request_id"`
	CommitId       string             `bson:"commit_id"                json:"commit_id"`
	Body           string             `bson:"body"                     json:"body"`
	Resolved       bool               `bson:"resolved"                 json:"resolved"`
	DiscussionId   string             `bson:"discussion_id"            json:"discussion_id"`
	NoteId         int                `bson:"note_id"                  json:"note_id"`
	CreateTime     int64              `bson:"create_time"              json:"create_time"`
}

type RepoInfo struct {
	CodehostId int    `bson:"codehost_id"                json:"codehost_id"`
	Source     string `bson:"source"                     json:"source"`
	ProjectId  string `bson:"project_id"                 json:"project_id"`
	Address    string `bson:"address"                    json:"address"`
	OauthToken string `bson:"oauth_token"                json:"oauth_token"`
}

func (DiffNote) TableName() string {
	return "diff_note"
}
