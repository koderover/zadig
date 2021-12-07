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
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/crypto"
)

func GetConfigPayload(codeHostID int) *models.ConfigPayload {
	payload := &models.ConfigPayload{
		S3Storage: models.S3Config{
			Ak:       config.S3StorageAK(),
			Sk:       config.S3StorageSK(),
			Endpoint: config.S3StorageEndpoint(),
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
		AesKey: crypto.GetAesKey(),
	}

	githubApps, _ := mongodb.NewGithubAppColl().Find()
	if len(githubApps) != 0 {
		payload.Github.AppKey = githubApps[0].AppKey
		payload.Github.AppID = githubApps[0].AppID
	}

	if codeHostID > 0 {
		ch, _ := systemconfig.New().GetCodeHost(codeHostID)
		if ch != nil && ch.Type == setting.SourceFromGithub {
			payload.Github.AccessToken = ch.AccessToken
		}
	}

	proxies, _ := mongodb.NewProxyColl().List(&mongodb.ProxyArgs{})
	if len(proxies) != 0 {
		payload.Proxy = *proxies[0]
	}

	privateKeys, _ := mongodb.NewPrivateKeyColl().List(&mongodb.PrivateKeyArgs{})
	if len(privateKeys) != 0 {
		payload.PrivateKeys = privateKeys
	}

	k8sClusters, _ := mongodb.NewK8SClusterColl().List(nil)
	if len(k8sClusters) != 0 {
		var K8SClusterResp []*models.K8SClusterResp
		for _, k8sCluster := range k8sClusters {
			K8SClusterResp = append(K8SClusterResp, &models.K8SClusterResp{
				ID:             k8sCluster.ID.Hex(),
				Name:           k8sCluster.Name,
				AdvancedConfig: k8sCluster.AdvancedConfig,
			})
		}
		payload.K8SClusters = K8SClusterResp
	}
	return payload
}
