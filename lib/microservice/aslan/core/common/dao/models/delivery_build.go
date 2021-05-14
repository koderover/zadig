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

	"github.com/koderover/zadig/lib/types"
)

type DeliveryBuild struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"                json:"id,omitempty"`
	ReleaseID   primitive.ObjectID  `bson:"release_id"           json:"releaseId"`
	ServiceName string              `bson:"service_name"         json:"serviceName"`
	ImageInfo   *DeliveryImage      `bson:"image_info"           json:"imageInfo"`
	ImageName   string              `bson:"image_name"           json:"imageName"`
	PackageInfo *DeliveryPackage    `bson:"package_info"         json:"packageInfo"`
	Issues      []*JiraIssue        `bson:"issues"               json:"issues"`
	Commits     []*types.Repository `bson:"commits"              json:"commits"`
	StartTime   int64               `bson:"start_time"           json:"start_time,omitempty"`
	EndTime     int64               `bson:"end_time"             json:"end_time,omitempty"`
	CreatedAt   int64               `bson:"created_at"           json:"created_at"`
	DeletedAt   int64               `bson:"deleted_at"           json:"deleted_at"`
}

// JiraIssue ...
type JiraIssue struct {
	ID          string `bson:"id,omitempty"                    json:"id,omitempty"`
	Key         string `bson:"key,omitempty"                   json:"key,omitempty"`
	URL         string `bson:"url,omitempty"                   json:"url,omitempty"`
	Summary     string `bson:"summary"                         json:"summary"`
	Description string `bson:"description,omitempty"           json:"description,omitempty"`
	Priority    string `bson:"priority,omitempty"              json:"priority,omitempty"`
	Creator     string `bson:"creator,omitempty"               json:"creator,omitempty"`
	Assignee    string `bson:"assignee,omitempty"              json:"assignee,omitempty"`
	Reporter    string `bson:"reporter,omitempty"              json:"reporter,omitempty"`
}

type DeliveryImage struct {
	RepoName      string `bson:"repo_name"       json:"repoName"`
	TagName       string `bson:"tag_name"        json:"tagName"`
	ImageSize     int64  `bson:"image_size"      json:"imageSize"`
	ImageDigest   string `bson:"image_digest"    json:"imageDigest"`
	Author        string `bson:"author"          json:"author"`
	Architecture  string `bson:"architecture"    json:"architecture"`
	DockerVersion string `bson:"docker_version"  json:"dockerVersion"`
	Os            string `bson:"os"              json:"os"`
	CreationTime  string `bson:"creation_time"   json:"creationTime"`
	UpdateTime    string `bson:"update_time"     json:"updateTime"`
}

type DeliveryPackage struct {
	PackageFileLocation string `bson:"package_file_location"        json:"packageFileLocation"`
	PackageFileName     string `bson:"package_file_name"            json:"packageFileName"`
	PackageStorageUri   string `bson:"package_storage_uri"          json:"packageStorageUri"`
}

func (DeliveryBuild) TableName() string {
	return "delivery_build"
}
