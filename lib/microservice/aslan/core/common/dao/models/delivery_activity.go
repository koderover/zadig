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

type DeliveryActivity struct {
	ArtifactID        primitive.ObjectID `bson:"artifact_id"                  json:"artifact_id"`
	Type              string             `bson:"type"                         json:"type"`
	Content           string             `bson:"content,omitempty"            json:"content,omitempty"`
	URL               string             `bson:"url,omitempty"                json:"url,omitempty"`
	Commits           []*ActivityCommit  `bson:"commits,omitempty"            json:"commits,omitempty"`
	Issues            []string           `bson:"issues,omitempty"             json:"issues,omitempty"`
	Namespace         string             `bson:"namespace,omitempty"          json:"namespace,omitempty"`
	EnvName           string             `bson:"env_name,omitempty"           json:"env_name,omitempty"`
	PublishHosts      []string           `bson:"publish_hosts,omitempty"      json:"publish_hosts,omitempty"`
	PublishNamespaces []string           `bson:"publish_namespaces,omitempty" json:"publish_namespaces,omitempty"`
	RemoteFileKey     string             `bson:"remote_file_key,omitempty"    json:"remote_file_key,omitempty"`
	DistStorageUrl    string             `bson:"dist_storage_url,omitempty"   json:"dist_storage_url,omitempty"`
	SrcStorageUrl     string             `bson:"src_storage_url,omitempty"    json:"src_storage_url,omitempty"`
	StartTime         int64              `bson:"start_time,omitempty"         json:"start_time,omitempty"`
	EndTime           int64              `bson:"end_time,omitempty"           json:"end_time,omitempty"`
	CreatedBy         string             `bson:"created_by"                   json:"created_by"`
	CreatedTime       int64              `bson:"created_time"                 json:"created_time"`
}

type ActivityCommit struct {
	Address       string `bson:"address"                   json:"address"`
	Source        string `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner     string `bson:"repo_owner"                json:"repo_owner"`
	RepoName      string `bson:"repo_name"                 json:"repo_name"`
	Branch        string `bson:"branch"                    json:"branch"`
	PR            int    `bson:"pr,omitempty"              json:"pr,omitempty"`
	Tag           string `bson:"tag,omitempty"             json:"tag,omitempty"`
	CommitID      string `bson:"commit_id,omitempty"       json:"commit_id,omitempty"`
	CommitMessage string `bson:"commit_message,omitempty"  json:"commit_message,omitempty"`
	AuthorName    string `bson:"author_name,omitempty"     json:"author_name,omitempty"`
}

func (DeliveryActivity) TableName() string {
	return "activity"
}
