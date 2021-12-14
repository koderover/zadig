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

package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
)

func CreateWorkflowTask(header http.Header, qs url.Values, body []byte, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().CreateWorkflowTask(header, qs, body)
}

func CancelWorkflowTask(header http.Header, qs url.Values, id string, name string, _ *zap.SugaredLogger) (int, error) {
	return aslan.New().CancelWorkflowTask(header, qs, id, name)
}

func RestartWorkflowTask(header http.Header, qs url.Values, id string, name string, _ *zap.SugaredLogger) (int, error) {
	return aslan.New().RestartWorkflowTask(header, qs, id, name)
}

func ListWorkflowTask(header http.Header, qs url.Values, commitId string, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().ListWorkflowTask(header, qs, commitId)
}

func ListDelivery(header http.Header, qs url.Values, productName string, workflowName string, taskId string, perPage string, page string, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().ListDelivery(header, qs, productName, workflowName, taskId, perPage, page)
}

type DeliveryArtifact struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	Source              string `json:"source"`
	Image               string `json:"image,omitempty"`
	ImageHash           string `json:"image_hash,omitempty"`
	ImageTag            string `json:"image_tag"`
	ImageDigest         string `json:"image_digest,omitempty"`
	ImageSize           int64  `json:"image_size,omitempty"`
	Architecture        string `json:"architecture,omitempty"`
	Os                  string `json:"os,omitempty"`
	PackageFileLocation string `json:"package_file_location,omitempty"`
	PackageStorageURI   string `json:"package_storage_uri,omitempty"`
	CreatedBy           string `json:"created_by"`
	CreatedTime         int64  `json:"created_time"`
}

type DeliveryActivity struct {
	Type              string            `json:"type"`
	Content           string            `json:"content,omitempty"`
	URL               string            `json:"url,omitempty"`
	Commits           []*ActivityCommit `json:"commits,omitempty"`
	Issues            []string          `json:"issues,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	EnvName           string            `json:"env_name,omitempty"`
	PublishHosts      []string          `json:"publish_hosts,omitempty"`
	PublishNamespaces []string          `json:"publish_namespaces,omitempty"`
	RemoteFileKey     string            `json:"remote_file_key,omitempty"`
	DistStorageURL    string            `json:"dist_storage_url,omitempty"`
	SrcStorageURL     string            `json:"src_storage_url,omitempty"`
	StartTime         int64             `json:"start_time,omitempty"`
	EndTime           int64             `json:"end_time,omitempty"`
	CreatedBy         string            `json:"created_by"`
	CreatedTime       int64             `json:"created_time"`
}

type ActivityCommit struct {
	Address       string `json:"address"`
	Source        string `json:"source,omitempty"`
	RepoOwner     string `json:"repo_owner"`
	RepoName      string `json:"repo_name"`
	Branch        string `json:"branch"`
	PR            int    `json:"pr,omitempty"`
	Tag           string `json:"tag,omitempty"`
	CommitID      string `json:"commit_id,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	AuthorName    string `json:"author_name,omitempty"`
}

type DeliveryArtifactInfo struct {
	DeliveryArtifact      *DeliveryArtifact              `json:"delivery_artifact"`
	DeliveryActivities    []*DeliveryActivity            `json:"activities"`
	DeliveryActivitiesMap map[string][]*DeliveryActivity `json:"sortedActivities,omitempty"`
}

func GetArtifactInfo(header http.Header, qs url.Values, image string, _ *zap.SugaredLogger) (*DeliveryArtifactInfo, error) {
	artifactID, err := aslan.New().GetArtifactByImage(header, qs, image)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact by image err:%s", err)
	}

	body, err := aslan.New().GetArtifactInfo(header, qs, string(artifactID))
	var deliveryArtifactInfo *DeliveryArtifactInfo
	if err = json.Unmarshal(body, &deliveryArtifactInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deliveryArtifactInfo err:%s", err)
	}

	return deliveryArtifactInfo, nil
}
