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

package task

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type ConfigPayload struct {
	APIToken           string             `json:"api_token"`
	Proxy              Proxy              `json:"proxy"`
	S3Storage          S3Config           `json:"s3_storage"`
	Github             GithubConfig       `json:"github"`
	Gitlab             GitlabConfig       `json:"gitlab"`
	Build              BuildConfig        `json:"build"`
	Test               TestConfig         `json:"test"`
	Registry           RegistryConfig     `json:"registry"`
	Release            ReleaseConfig      `json:"release"`
	JenkinsBuildConfig JenkinsBuildConfig `json:"jenkins_build_config"`
	// 推送线上镜像需要的用户名密码, 根据pipeline镜像发布模块动态配置
	ImageRelease ImageReleaseConfig `json:"image_release"`
	Docker       DockerConfig       `json:"docker"`

	ClassicBuild bool `json:"classic_build"`

	CustomDNSSupported bool `json:"custom_dns_supported"`
	HubServerAddr      string
	DeployClusterID    string
	AesKey             string `json:"aes_key"`

	RepoConfigs map[string]*RegistryNamespace

	// IgnoreCache means ignore docker build cache
	IgnoreCache bool `json:"ignore_cache"`

	// ResetCache means ignore workspace cache
	ResetCache  bool          `json:"reset_cache"`
	PrivateKeys []*PrivateKey `json:"private_keys"`
	K8SClusters []*K8SCluster `json:"k8s_clusters"`

	RegistryID string `json:"registry_id"`

	// build concurrency settings
	BuildConcurrency int64 `json:"build_concurrency"`
}

func (cp *ConfigPayload) GetGitKnownHost() string {
	var host string
	if cp.Github.KnownHost != "" {
		host = cp.Github.KnownHost
	}

	if cp.Gitlab.KnownHost != "" {
		host = fmt.Sprintf("%s\n%s", host, cp.Gitlab.KnownHost)
	}
	return host
}

type ProxyConfig struct {
	HTTPAddr   string
	HTTPSAddr  string
	Socks5Addr string
	NoProxy    string
}

type S3Config struct {
	Ak       string
	Sk       string
	Endpoint string
	Bucket   string
	Path     string
	Protocol string
	Provider int8
	Region   string
}

type GithubConfig struct {
	// github API access token
	AccessToken string `json:"access_token"`
	// github ssh key with base64 encoded
	SSHKey string `json:"ssh_key"`
	// github knownhost
	KnownHost string `json:"known_host"`
	// github app private key
	AppKey string `json:"app_key"`
	// gihhub app id
	AppID int `json:"app_id"`
}

type GitlabConfig struct {
	APIServer string
	// Github API access token
	AccessToken string
	// gitlab ssh key with base64 encoded
	SSHKey string
	// gitlab knownhost
	KnownHost string
}

type BuildConfig struct {
	KubeNamespace string
}

type TestConfig struct {
	KubeNamespace string
}

type RegistryConfig struct {
	Addr        string
	AccessKey   string
	SecretKey   string
	Namespace   string
	RepoAddress string
}

type ReleaseConfig struct {
	// ReaperImage sets build job image
	// e.g. xxx.com/resources/reaper-plugin:1.0.0
	ReaperImage string
	// ReaperBinaryFile sets download url of reaper binary file in build job
	// e.g. http://resource.koderover.com/reaper-20201014203000
	ReaperBinaryFile string
	// PredatorImage sets docker build image
	// e.g. xxx.com/resources/predator-plugin:v0.1.0
	PredatorImage string
	// PackagerImage sets docker build image
	// e.g. xxx.com/resources/packager-plugin:v0.1.0
	PackagerImage string
}

type ImageReleaseConfig struct {
	Addr      string
	AccessKey string
	SecretKey string
}

type DockerConfig struct {
	HostList []string
}

type JenkinsBuildConfig struct {
	JenkinsBuildImage string
}

type PrivateKey struct {
	Name string `json:"name"`
}

type K8SCluster struct {
	ID             string          `json:"id,omitempty"                bson:"id,omitempty"`
	Name           string          `json:"name"                        bson:"name"`
	AdvancedConfig *AdvancedConfig `json:"advanced_config,omitempty"   bson:"advanced_config,omitempty"`
}

type AdvancedConfig struct {
	Strategy    string                     `json:"strategy,omitempty"      bson:"strategy,omitempty"`
	NodeLabels  []*NodeSelectorRequirement `json:"node_labels,omitempty"   bson:"node_labels,omitempty"`
	Tolerations string                     `json:"tolerations"             bson:"tolerations"`
}

type NodeSelectorRequirement struct {
	Key      string                      `json:"key"`
	Value    []string                    `json:"value"`
	Operator corev1.NodeSelectorOperator `json:"operator"`
}
