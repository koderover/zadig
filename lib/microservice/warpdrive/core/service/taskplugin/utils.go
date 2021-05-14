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
	"net/http"
	"strings"
	"time"

	t "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"

	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

func int32Ptr(i int32) *int32 { return &i }

func GetFeatures() (*Features, error) {
	header := http.Header{}
	header.Set(setting.Auth, fmt.Sprintf("%s%s", setting.AuthPrefix, config.PoetryAPIRootKey()))
	header.Set("content-type", "application/json")

	url := config.PoetryAPIAddr() + "/directory/check"

	data, err := util.SendRequest(url, http.MethodGet, header, nil)
	if err != nil {
		return nil, fmt.Errorf(err.Error())
	}

	var featuresObj *Features
	if err := json.Unmarshal(data, &featuresObj); err != nil {
		return nil, fmt.Errorf(err.Error())
	}

	return featuresObj, nil
}

//  选择最合适的dockerhost
func GetBestDockerHost(hostList []string, pipelineType, namespace string, log *xlog.Logger) (string, error) {
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
		return nil, fmt.Errorf("convert interface to SubTaskPreview error: %v", err)
	}
	return pre, nil
}

func ToBuildTask(sb map[string]interface{}) (*task.Build, error) {
	var t *task.Build
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to BuildTaskV2 error: %v", err)
	}
	return t, nil
}

func ToArtifactTask(sb map[string]interface{}) (*task.Artifact, error) {
	var t *task.Artifact
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ArtifactTask error: %v", err)
	}
	return t, nil
}

func ToDockerBuildTask(sb map[string]interface{}) (*task.DockerBuild, error) {
	var t *task.DockerBuild
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DockerBuildTask error: %v", err)
	}
	return t, nil
}

func ToDeployTask(sb map[string]interface{}) (*task.Deploy, error) {
	var t *task.Deploy
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DeployTask error: %v", err)
	}
	return t, nil
}

func ToTestingTask(sb map[string]interface{}) (*task.Testing, error) {
	var t *task.Testing
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to Testing error: %v", err)
	}
	return t, nil
}

func ToDistributeToS3Task(sb map[string]interface{}) (*task.DistributeToS3, error) {
	var t *task.DistributeToS3
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DistributeToS3Task error: %v", err)
	}
	return t, nil
}

func ToReleaseImageTask(sb map[string]interface{}) (*task.ReleaseImage, error) {
	var t *task.ReleaseImage
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ReleaseImageTask error: %v", err)
	}
	return t, nil
}

func ToJiraTask(sb map[string]interface{}) (*task.Jira, error) {
	var t *task.Jira
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to JiraTask error: %v", err)
	}
	return t, nil
}

func ToSecurityTask(sb map[string]interface{}) (*task.Security, error) {
	var t *task.Security
	if err := IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to securityTask error: %v", err)
	}
	return t, nil
}

func ToJenkinsBuildTask(sb map[string]interface{}) (*task.JenkinsBuild, error) {
	var task *task.JenkinsBuild
	if err := IToi(sb, &task); err != nil {
		return nil, fmt.Errorf("convert interface to JenkinsBuildTask error: %v", err)
	}
	return task, nil
}

func IToi(before interface{}, after interface{}) error {
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := json.Unmarshal(b, &after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}

	return nil
}
