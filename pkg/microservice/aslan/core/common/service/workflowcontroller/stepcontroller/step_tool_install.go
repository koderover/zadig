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
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type toolInstallCtl struct {
	step             *commonmodels.StepTask
	toolInstalldSpec *step.StepToolInstallSpec
	jobPath          *string
	log              *zap.SugaredLogger
}

func NewToolInstallCtl(stepTask *commonmodels.StepTask, jobPath *string, log *zap.SugaredLogger) (*toolInstallCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal tool install spec error: %v", err)
	}
	toolInstallSpec := &step.StepToolInstallSpec{}
	if err := yaml.Unmarshal(yamlString, &toolInstallSpec); err != nil {
		return nil, fmt.Errorf("unmarshal tool install spec error: %v", err)
	}
	stepTask.Spec = toolInstallSpec
	return &toolInstallCtl{toolInstalldSpec: toolInstallSpec, jobPath: jobPath, log: log, step: stepTask}, nil
}

func (s *toolInstallCtl) PreRun(ctx context.Context) error {
	spec := &step.StepToolInstallSpec{
		S3Storage: &step.S3{
			Ak:        config.S3StorageAK(),
			Sk:        config.S3StorageSK(),
			Endpoint:  config.S3StorageEndpoint(),
			Bucket:    config.S3StorageBucket(),
			Subfolder: config.S3StoragePath(),
			Protocol:  config.S3StorageProtocol(),
		},
	}
	objectStorage, _ := commonrepo.NewS3StorageColl().FindDefault()
	if objectStorage != nil {
		spec.S3Storage = modelS3toS3(objectStorage)
	}

	for _, tool := range s.toolInstalldSpec.Installs {
		install, err := buildInstallCtx(tool.Name, tool.Version)
		if err != nil {
			s.log.Error(err)
		}
		spec.Installs = append(spec.Installs, &step.Tool{
			Name:     tool.Name,
			Version:  tool.Version,
			BinPath:  install.BinPath,
			Download: install.DownloadPath,
			Envs:     install.Envs,
			Scripts:  strings.Split(replaceWrapLine(install.Scripts), "\n"),
		})
		s.jobPath = &install.BinPath
	}

	s.step.Spec = spec
	return nil
}

func (s *toolInstallCtl) AfterRun(ctx context.Context) error {
	return nil
}

// 根据用户的配置和BuildStep中步骤的依赖，从系统配置的InstallItems中获取配置项，构建Install Context
func buildInstallCtx(name, version string) (*commonmodels.Install, error) {
	resp := &commonmodels.Install{}
	if name == "" || version == "" {
		return resp, nil
	}
	install, err := commonrepo.NewInstallColl().Find(name, version)
	if err != nil {
		return resp, fmt.Errorf("%s:%s not found", name, version)
	}
	if !install.Enabled {
		return resp, fmt.Errorf("%s:%s disabled", name, version)
	}
	resp = install
	return resp, nil
}

func replaceWrapLine(script string) string {
	return strings.Replace(strings.Replace(
		script,
		"\r\n",
		"\n",
		-1,
	), "\r", "\n", -1)
}
