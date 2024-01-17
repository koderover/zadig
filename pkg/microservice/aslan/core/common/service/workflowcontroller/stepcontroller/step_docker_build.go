/*
Copyright 2022 The KodeRover Authors.

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

package stepcontroller

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type dockerBuildCtl struct {
	step            *commonmodels.StepTask
	dockerBuildSpec *step.StepDockerBuildSpec
	workflowCtx     *commonmodels.WorkflowTaskCtx
	log             *zap.SugaredLogger
}

func NewDockerBuildCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*dockerBuildCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal docker build spec error: %v", err)
	}
	dockerBuildSpec := &step.StepDockerBuildSpec{}
	if err := yaml.Unmarshal(yamlString, &dockerBuildSpec); err != nil {
		return nil, fmt.Errorf("unmarshal docker build spec error: %v", err)
	}
	if dockerBuildSpec.Proxy == nil {
		dockerBuildSpec.Proxy = &step.Proxy{}
	}
	stepTask.Spec = dockerBuildSpec
	return &dockerBuildCtl{dockerBuildSpec: dockerBuildSpec, workflowCtx: workflowCtx, log: log, step: stepTask}, nil
}

func (s *dockerBuildCtl) PreRun(ctx context.Context) error {
	proxies, _ := mongodb.NewProxyColl().List(&mongodb.ProxyArgs{})
	if len(proxies) != 0 {
		s.dockerBuildSpec.Proxy.Address = proxies[0].Address
		s.dockerBuildSpec.Proxy.EnableApplicationProxy = proxies[0].EnableApplicationProxy
		s.dockerBuildSpec.Proxy.EnableRepoProxy = proxies[0].EnableRepoProxy
		s.dockerBuildSpec.Proxy.NeedPassword = proxies[0].NeedPassword
		s.dockerBuildSpec.Proxy.Password = proxies[0].Password
		s.dockerBuildSpec.Proxy.Port = proxies[0].Port
		s.dockerBuildSpec.Proxy.Type = proxies[0].Type
		s.dockerBuildSpec.Proxy.Username = proxies[0].Username
	}
	s.step.Spec = s.dockerBuildSpec
	return nil
}

func (s *dockerBuildCtl) AfterRun(ctx context.Context) error {
	deliveryArtifact := new(commonmodels.DeliveryArtifact)
	deliveryArtifact.CreatedBy = s.workflowCtx.WorkflowTaskCreatorUsername
	deliveryArtifact.CreatedTime = time.Now().Unix()
	deliveryArtifact.Source = string(config.WorkflowTypeV4)

	image := s.dockerBuildSpec.ImageName
	s.log.Debugf("dockerBuildCtl AfterRun: image:%s", image)
	imageArray := strings.Split(image, "/")
	tagArray := strings.Split(imageArray[len(imageArray)-1], ":")
	imageName := tagArray[0]
	imageTag := tagArray[1]

	deliveryArtifact.Image = image
	deliveryArtifact.Type = string(config.Image)
	deliveryArtifact.Name = imageName
	deliveryArtifact.ImageTag = imageTag
	err := commonrepo.NewDeliveryArtifactColl().Insert(deliveryArtifact)
	if err != nil {
		return fmt.Errorf("archiveCtl AfterRun: insert delivery artifact error: %v", err)
	}

	deliveryActivity := new(commonmodels.DeliveryActivity)
	deliveryActivity.ArtifactID = deliveryArtifact.ID
	deliveryActivity.Type = setting.BuildType
	deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", s.workflowCtx.ProjectName, s.workflowCtx.WorkflowName, s.workflowCtx.TaskID, url.QueryEscape(s.workflowCtx.WorkflowDisplayName))
	commits := make([]*commonmodels.ActivityCommit, 0)
	for _, repo := range s.dockerBuildSpec.Repos {
		deliveryCommit := new(commonmodels.ActivityCommit)
		deliveryCommit.Address = repo.Address
		deliveryCommit.Source = repo.Source
		deliveryCommit.RepoOwner = repo.RepoOwner
		deliveryCommit.RepoName = repo.RepoName
		deliveryCommit.Branch = repo.Branch
		deliveryCommit.Tag = repo.Tag
		deliveryCommit.PR = repo.PR
		deliveryCommit.PRs = repo.PRs
		deliveryCommit.CommitID = repo.CommitID
		deliveryCommit.CommitMessage = repo.CommitMessage
		deliveryCommit.AuthorName = repo.AuthorName

		commits = append(commits, deliveryCommit)
	}
	deliveryActivity.Commits = commits

	deliveryActivity.CreatedBy = s.workflowCtx.WorkflowTaskCreatorUsername
	deliveryActivity.CreatedTime = time.Now().Unix()
	deliveryActivity.StartTime = s.workflowCtx.StartTime.Unix()
	deliveryActivity.EndTime = time.Now().Unix()

	err = commonrepo.NewDeliveryActivityColl().Insert(deliveryActivity)
	if err != nil {
		return fmt.Errorf("archiveCtl AfterRun: build deliveryActivityColl insert err:%v", err)
	}
	return nil
}
