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

type DeliveryArtifact struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"                   json:"id"`
	Name                string             `bson:"name"                            json:"name"`
	Type                string             `bson:"type"                            json:"type"`
	Source              string             `bson:"source"                          json:"source"`
	Image               string             `bson:"image,omitempty"                 json:"image,omitempty"`
	ImageHash           string             `bson:"image_hash,omitempty"            json:"image_hash,omitempty"`
	ImageTag            string             `bson:"image_tag"                       json:"image_tag"`
	ImageDigest         string             `bson:"image_digest,omitempty"          json:"image_digest,omitempty"`
	ImageSize           int64              `bson:"image_size,omitempty"            json:"image_size,omitempty"`
	Architecture        string             `bson:"architecture,omitempty"          json:"architecture,omitempty"`
	Os                  string             `bson:"os,omitempty"                    json:"os,omitempty"`
	DockerFile          string             `bson:"docker_file,omitempty"           json:"docker_file,omitempty"`
	Layers              []Descriptor       `bson:"layers,omitempty"                json:"layers,omitempty"`
	PackageFileLocation string             `bson:"package_file_location,omitempty" json:"package_file_location,omitempty"`
	PackageStorageURI   string             `bson:"package_storage_uri,omitempty"   json:"package_storage_uri,omitempty"`
	CreatedBy           string             `bson:"created_by"                      json:"created_by"`
	CreatedTime         int64              `bson:"created_time"                    json:"created_time"`
}

type Descriptor struct {
	MediaType string   `bson:"mediatype" json:"media_type,omitempty"`
	Size      int64    `bson:"size" json:"size"`
	Digest    string   `bson:"digest" json:"digest,omitempty"`
	URLs      []string `bson:"urls" json:"urls,omitempty"`
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

func (DeliveryArtifact) TableName() string {
	return "artifact"
}
