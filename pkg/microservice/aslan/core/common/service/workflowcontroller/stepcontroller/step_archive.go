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

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/types/step"
)

type archiveCtl struct {
	step        *commonmodels.StepTask
	archiveSpec *step.StepArchiveSpec
	log         *zap.SugaredLogger
}

func NewArchiveCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*archiveCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal tool install spec error: %v", err)
	}
	archiveSpec := &step.StepArchiveSpec{}
	if err := yaml.Unmarshal(yamlString, &archiveSpec); err != nil {
		return nil, fmt.Errorf("unmarshal git spec error: %v", err)
	}
	stepTask.Spec = archiveSpec
	return &archiveCtl{archiveSpec: archiveSpec, log: log, step: stepTask}, nil
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
	}
	if modelS3.Insecure {
		resp.Protocol = "http"
	}
	return resp
}
