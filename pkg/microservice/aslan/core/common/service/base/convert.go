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

package base

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
)

type Preview struct {
	TaskType config.TaskType `json:"type"`
	Enabled  bool            `json:"enabled"`
}

func ToPreview(sb map[string]interface{}) (*Preview, error) {
	var pre *Preview
	if err := task.IToi(sb, &pre); err != nil {
		return nil, fmt.Errorf("convert interface to SubTaskPreview error: %v", err)
	}
	return pre, nil
}

func ToBuildTask(sb map[string]interface{}) (*task.Build, error) {
	var t *task.Build
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to BuildTaskV2 error: %v", err)
	}
	return t, nil
}

func ToTestingTask(sb map[string]interface{}) (*task.Testing, error) {
	var t *task.Testing
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to Testing error: %v", err)
	}
	return t, nil
}

func ToArtifactTask(sb map[string]interface{}) (*task.Artifact, error) {
	var t *task.Artifact
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ArtifactTask error: %v", err)
	}
	return t, nil
}

func ToDockerBuildTask(sb map[string]interface{}) (*task.DockerBuild, error) {
	var t *task.DockerBuild
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DockerBuildTask error: %v", err)
	}
	return t, nil
}

func ToDeployTask(sb map[string]interface{}) (*task.Deploy, error) {
	var t *task.Deploy
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DeployTask error: %v", err)
	}
	return t, nil
}

func ToDistributeToS3Task(sb map[string]interface{}) (*task.DistributeToS3, error) {
	var t *task.DistributeToS3
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to DistributeToS3Task error: %v", err)
	}
	return t, nil
}

func ToReleaseImageTask(sb map[string]interface{}) (*task.ReleaseImage, error) {
	var t *task.ReleaseImage
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to ReleaseImageTask error: %v", err)
	}
	return t, nil
}

func ToJiraTask(sb map[string]interface{}) (*task.Jira, error) {
	var t *task.Jira
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to JiraTask error: %v", err)
	}
	return t, nil
}

func ToSecurityTask(sb map[string]interface{}) (*task.Security, error) {
	var t *task.Security
	if err := task.IToi(sb, &t); err != nil {
		return nil, fmt.Errorf("convert interface to securityTask error: %v", err)
	}
	return t, nil
}

func ToJenkinsBuildTask(sb map[string]interface{}) (*task.JenkinsBuild, error) {
	var jenkinsBuild *task.JenkinsBuild
	if err := task.IToi(sb, &jenkinsBuild); err != nil {
		return nil, fmt.Errorf("convert interface to JenkinsBuildTask error: %v", err)
	}
	return jenkinsBuild, nil
}
