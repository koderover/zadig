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
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type archiveCtl struct {
	step        *commonmodels.StepTask
	archiveSpec *step.StepArchiveSpec
	workflowCtx *commonmodels.WorkflowTaskCtx
	log         *zap.SugaredLogger
}

func NewArchiveCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*archiveCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal archive spec error: %v", err)
	}
	archiveSpec := &step.StepArchiveSpec{}
	if err := yaml.Unmarshal(yamlString, &archiveSpec); err != nil {
		return nil, fmt.Errorf("unmarshal archive spec error: %v", err)
	}
	stepTask.Spec = archiveSpec
	return &archiveCtl{archiveSpec: archiveSpec, workflowCtx: workflowCtx, log: log, step: stepTask}, nil
}

func (s *archiveCtl) PreRun(ctx context.Context) error {
	if s.archiveSpec.S3 != nil {
		return nil
	}
	var modelS3 *commonmodels.S3Storage
	var err error
	if s.archiveSpec.ObjectStorageID == "" {
		modelS3, err = commonrepo.NewS3StorageColl().FindDefault()
		if err != nil {
			return err
		}
		s.archiveSpec.S3 = modelS3toS3(modelS3)
	} else {
		modelS3, err = commonrepo.NewS3StorageColl().Find(s.archiveSpec.ObjectStorageID)
		if err != nil {
			return err
		}
		s.archiveSpec.S3 = modelS3toS3(modelS3)
		s.archiveSpec.S3.Subfolder = ""
	}
	s.step.Spec = s.archiveSpec
	return nil
}

func (s *archiveCtl) AfterRun(ctx context.Context) error {
	for _, upload := range s.archiveSpec.UploadDetail {
		if !upload.IsFileArchive {
			continue
		}
		deliveryArtifact := new(commonmodels.DeliveryArtifact)
		deliveryArtifact.CreatedBy = s.workflowCtx.WorkflowTaskCreatorUsername
		deliveryArtifact.CreatedTime = time.Now().Unix()
		deliveryArtifact.Source = string(config.WorkflowTypeV4)
		deliveryArtifact.Name = upload.ServiceModule + "_" + upload.ServiceName
		// TODO(Ray) file类型的交付物名称存放在Image和ImageTag字段是不规范的，优化时需要考虑历史数据的兼容问题。
		deliveryArtifact.Image = upload.Name
		deliveryArtifact.ImageTag = upload.Name
		deliveryArtifact.Type = string(config.File)
		deliveryArtifact.PackageFileLocation = upload.PackageFileLocation
		deliveryArtifact.PackageStorageURI = s.archiveSpec.S3.Endpoint + "/" + s.archiveSpec.S3.Bucket
		err := commonrepo.NewDeliveryArtifactColl().Insert(deliveryArtifact)
		if err != nil {
			return fmt.Errorf("archiveCtl AfterRun: insert delivery artifact error: %v", err)
		}

		deliveryActivity := new(commonmodels.DeliveryActivity)
		deliveryActivity.Type = setting.BuildType
		deliveryActivity.ArtifactID = deliveryArtifact.ID
		deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", s.workflowCtx.ProjectName, s.workflowCtx.WorkflowName, s.workflowCtx.TaskID, url.QueryEscape(s.workflowCtx.WorkflowDisplayName))
		commits := make([]*commonmodels.ActivityCommit, 0)
		for _, repo := range s.archiveSpec.Repos {
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
	}
	return nil
}

func modelS3toS3(modelS3 *commonmodels.S3Storage) *step.S3 {
	resp := &step.S3{
		Ak:        modelS3.Ak,
		Sk:        modelS3.Sk,
		Endpoint:  modelS3.Endpoint,
		Bucket:    modelS3.Bucket,
		Subfolder: modelS3.Subfolder,
		Insecure:  modelS3.Insecure,
		Provider:  modelS3.Provider,
		Region:    modelS3.Region,
	}
	if modelS3.Insecure {
		resp.Protocol = "http"
	}
	return resp
}
