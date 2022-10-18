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

package taskplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	t "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kodo"
)

func int32Ptr(i int32) *int32 { return &i }

func uploadFileToS3(access, secret, bucket, remote, local string) error {
	s3Cli, err := kodo.NewUploadClient(access, secret, bucket)
	if err != nil {
		return err
	}
	if _, _, err := s3Cli.UploadFile(remote, local); err != nil {
		return err
	}
	return nil
}

//  选择最合适的dockerhost
func GetBestDockerHost(hostList []string, pipelineType, namespace string, log *zap.SugaredLogger) (string, error) {
	bestHosts := []string{}
	containerCount := 0
	for _, host := range hostList {
		if host == "" {
			continue
		}
		if pipelineType == string(config.ServiceType) {
			dockerHostArray := strings.Split(host, ":")
			if len(dockerHostArray) == 3 {
				host = fmt.Sprintf("%s:%s:%s", dockerHostArray[0], fmt.Sprintf("%s.%s", dockerHostArray[1], namespace), dockerHostArray[2])
			}
		}
		cli, err := client.NewClientWithOpts(client.WithHost(host))
		if err != nil {
			log.Warnf("[%s]create docker client error :%v", host, err)
			continue
		}
		containers, err := cli.ContainerList(context.Background(), t.ContainerListOptions{})
		// avoid too many docker connections
		_ = cli.Close()
		if err != nil {
			log.Warnf("[%s]list container error :%v", host, err)
			continue
		}
		if len(bestHosts) == 0 || containerCount > len(containers) {
			bestHosts = []string{host}
			containerCount = len(containers)
			continue
		}
		if containerCount == len(containers) {
			bestHosts = append(bestHosts, host)
		}
	}
	if len(bestHosts) == 0 {
		return "", fmt.Errorf("no docker host found")
	}
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	randomIndex := r.Intn(len(bestHosts))
	return bestHosts[randomIndex], nil
}

type Preview struct {
	TaskType config.TaskType `json:"type"`
	Enabled  bool            `json:"enabled"`
}

func ToPreview(sb map[string]interface{}) (*Preview, error) {
	var pre *Preview
	if err := IToi(sb, &pre); err != nil {
		return nil, fmt.Errorf("convert interface to SubTaskPreview error: %s", err)
	}
	return pre, nil
}

func ToBuildTask(sb map[string]interface{}) (*task.Build, error) {
	var t *task.Build
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to BuildTaskV2 error: %s", err)
	}
	return t, nil
}

func ToScanningTask(sb map[string]interface{}) (*task.Scanning, error) {
	var t *task.Scanning
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to Scanning task error: %s", err)
	}
	return t, nil
}

func ToArtifactTask(sb map[string]interface{}) (*task.ArtifactPackage, error) {
	var t *task.ArtifactPackage
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ArtifactTask error: %s", err)
	}
	return t, nil
}

func ToDockerBuildTask(sb map[string]interface{}) (*task.DockerBuild, error) {
	var t *task.DockerBuild
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DockerBuildTask error: %s", err)
	}
	return t, nil
}

func ToDeployTask(sb map[string]interface{}) (*task.Deploy, error) {
	var t *task.Deploy
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DeployTask error: %s", err)
	}
	return t, nil
}

func ToTestingTask(sb map[string]interface{}) (*task.Testing, error) {
	var t *task.Testing
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to Testing error: %s", err)
	}
	return t, nil
}

func ToDistributeToS3Task(sb map[string]interface{}) (*task.DistributeToS3, error) {
	var t *task.DistributeToS3
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DistributeToS3Task error: %s", err)
	}
	return t, nil
}

func ToReleaseImageTask(sb map[string]interface{}) (*task.ReleaseImage, error) {
	var t *task.ReleaseImage
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ReleaseImageTask error: %s", err)
	}
	return t, nil
}

func ToArtifactPackageTask(sb map[string]interface{}) (*task.ArtifactPackageTaskArgs, error) {
	var ret *task.ArtifactPackageTaskArgs
	if err := IToi(sb, &ret); err != nil {
		return nil, fmt.Errorf("convert interface to ArtifactPackageTaskArgs error: %s", err)
	}
	return ret, nil
}

func ToJiraTask(sb map[string]interface{}) (*task.Jira, error) {
	var t *task.Jira
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to JiraTask error: %s", err)
	}
	return t, nil
}

func ToSecurityTask(sb map[string]interface{}) (*task.Security, error) {
	var t *task.Security
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to securityTask error: %s", err)
	}
	return t, nil
}

func ToJenkinsBuildTask(sb map[string]interface{}) (*task.JenkinsBuild, error) {
	var task *task.JenkinsBuild
	if err := IToi(sb, &task); err != nil {
		return nil, fmt.Errorf("convert interface to JenkinsBuildTask error: %s", err)
	}
	return task, nil
}

func IToi(before interface{}, after interface{}) error {
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %s", err)
	}

	if err := json.Unmarshal(b, &after); err != nil {
		return fmt.Errorf("unmarshal task error: %s", err)
	}

	return nil
}

func ToTriggerTask(sb map[string]interface{}) (*task.Trigger, error) {
	var trigger *task.Trigger
	if err := task.IToi(sb, &trigger); err != nil {
		return nil, fmt.Errorf("convert interface to triggerTask error: %s", err)
	}
	return trigger, nil
}

func ToExtensionTask(sb map[string]interface{}) (*task.Extension, error) {
	var extension *task.Extension
	if err := task.IToi(sb, &extension); err != nil {
		return nil, fmt.Errorf("convert interface to extensionTask error: %s", err)
	}
	return extension, nil
}

func GetK8sClients(hubServerAddr, clusterID string) (crClient.Client, kubernetes.Interface, *rest.Config, error) {
	controllerRuntimeClient, err := kubeclient.GetKubeClient(hubServerAddr, clusterID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get controller runtime client: %s", err)
	}

	clientset, err := kubeclient.GetKubeClientSet(hubServerAddr, clusterID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get clientset: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(hubServerAddr, clusterID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get rest config: %s", err)
	}

	return controllerRuntimeClient, clientset, restConfig, nil
}

// InstantiateBuildSysVariables instantiate system variables for build module
func InstantiateBuildSysVariables(jobCtx *task.JobCtx) []*task.KeyVal {
	ret := make([]*task.KeyVal, 0)
	for index, repo := range jobCtx.Builds {

		repoNameIndex := fmt.Sprintf("REPONAME_%d", index)
		ret = append(ret, &task.KeyVal{Key: fmt.Sprintf(repoNameIndex), Value: repo.RepoName, IsCredential: false})

		repoName := strings.Replace(repo.RepoName, "-", "_", -1)

		repoIndex := fmt.Sprintf("REPO_%d", index)
		ret = append(ret, &task.KeyVal{Key: fmt.Sprintf(repoIndex), Value: repoName, IsCredential: false})

		if len(repo.Branch) > 0 {
			ret = append(ret, &task.KeyVal{Key: fmt.Sprintf("%s_BRANCH", repoName), Value: repo.Branch, IsCredential: false})
		}

		if len(repo.Tag) > 0 {
			ret = append(ret, &task.KeyVal{Key: fmt.Sprintf("%s_TAG", repoName), Value: repo.Tag, IsCredential: false})
		}

		if repo.PR > 0 {
			ret = append(ret, &task.KeyVal{Key: fmt.Sprintf("%s_PR", repoName), Value: strconv.Itoa(repo.PR), IsCredential: false})
		}

		if len(repo.PRs) > 0 {
			prStrs := []string{}
			for _, pr := range repo.PRs {
				prStrs = append(prStrs, strconv.Itoa(pr))
			}
			ret = append(ret, &task.KeyVal{Key: fmt.Sprintf("%s_PR", repoName), Value: strings.Join(prStrs, ","), IsCredential: false})
		}

		if len(repo.CommitID) > 0 {
			ret = append(ret, &task.KeyVal{Key: fmt.Sprintf("%s_COMMIT_ID", repoName), Value: repo.CommitID, IsCredential: false})
		}
	}
	return ret
}

// getReaperImage generates the image used to run reaper
// depends on the setting on page 'Build'
func getReaperImage(reaperImage, buildOS, imageFrom string) string {
	// for built-in image, reaperImage and buildOs can generate a complete image
	// reaperImage: koderover.tencentcloudcr.com/koderover-public/build-base:${BuildOS}-amd64
	// buildOS: focal xenial bionic
	jobImage := strings.ReplaceAll(reaperImage, "${BuildOS}", buildOS)
	// for custom image, buildOS represents the exact custom image
	if imageFrom == setting.ImageFromCustom {
		jobImage = buildOS
	}
	return jobImage
}
