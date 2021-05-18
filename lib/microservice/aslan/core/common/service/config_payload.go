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
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
)

func GetConfigPayload() *models.ConfigPayload {
	payload := &models.ConfigPayload{
		Aslan: models.AslanConfig{
			URL:              config.AslanURL(),
			WarpdriveService: config.ENVWarpdriveService(),
		},
		S3Storage: models.S3Config{
			Ak:       config.S3StorageAK(),
			Sk:       config.S3StorageSK(),
			Endpoint: config.S3StorageBucket(),
			Bucket:   config.S3StorageBucket(),
			Path:     config.S3StoragePath(),
			Protocol: config.S3StorageProtocol(),
		},
		Build: models.BuildConfig{
			KubeNamespace: config.Namespace(),
		},
		Test: models.TestConfig{
			KubeNamespace: config.Namespace(),
		},
		Registry: models.RegistryConfig{
			AccessKey:   config.RegistryAccessKey(),
			SecretKey:   config.RegistrySecretKey(),
			Namespace:   config.RegistryNamespace(),
			RepoAddress: config.RegistryAddress(),
		},
		Github: models.GithubConfig{
			SSHKey:    config.GithubSSHKey(),
			KnownHost: config.GithubKnownHost(),
		},
		Release: models.ReleaseConfig{
			ReaperImage:      config.ReaperImage(),
			ReaperBinaryFile: config.ReaperBinaryFile(),
			PredatorImage:    config.PredatorImage(),
		},
		Docker: models.DockerConfig{
			HostList: config.DockerHosts(),
		},
		ClassicBuild:       config.UseClassicBuild(),
		CustomDNSSupported: config.CustomDNSNotSupported(),
		HubServerAddr:      config.HubServerAddress(),
		JenkinsBuildConfig: models.JenkinsBuildConfig{
			JenkinsBuildImage: config.JenkinsImage(),
		},
	}

	githubApps, _ := repo.NewGithubAppColl().Find()
	if len(githubApps) != 0 {
		payload.Github.AppKey = githubApps[0].AppKey
		payload.Github.AppID = githubApps[0].AppID
	}

	proxies, _ := repo.NewProxyColl().List(&repo.ProxyArgs{})
	if len(proxies) != 0 {
		payload.Proxy = *proxies[0]
	}

	return payload
}
