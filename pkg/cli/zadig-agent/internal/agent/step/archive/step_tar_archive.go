/*
Copyright 2023 The KodeRover Authors.

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

package archive

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util/fs"
)

type TarArchiveStep struct {
	spec       *step.StepTarArchiveSpec
	envs       []string
	secretEnvs []string
	workspace  string
	logger     *log.JobLogger
	dirs       *types.AgentWorkDirs
}

func NewTararchiveStep(spec interface{}, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*TarArchiveStep, error) {
	tarArchiveStep := &TarArchiveStep{dirs: dirs, envs: envs, secretEnvs: secretEnvs, logger: logger}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return tarArchiveStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &tarArchiveStep.spec); err != nil {
		return tarArchiveStep, fmt.Errorf("unmarshal spec %s to script spec failed", yamlBytes)
	}
	return tarArchiveStep, nil
}

func (s *TarArchiveStep) Run(ctx context.Context) error {
	if len(s.spec.ResultDirs) == 0 {
		return nil
	}
	s.logger.Infof(fmt.Sprintf("Start tar archive %s.", s.spec.FileName))
	forcedPathStyle := true
	if s.spec.S3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3.NewClient(s.spec.S3Storage.Endpoint, s.spec.S3Storage.Ak, s.spec.S3Storage.Sk, s.spec.S3Storage.Region, s.spec.S3Storage.Insecure, forcedPathStyle)
	if err != nil {
		return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
	}
	tarName := filepath.Join(s.spec.DestDir, s.spec.FileName)
	cmdAndArtifactFullPaths := make([]string, 0)
	cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, "-czf")
	cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, tarName)
	envMap := helper.MakeEnvMap(s.envs, s.secretEnvs)
	for _, artifactPath := range s.spec.ResultDirs {
		if len(artifactPath) == 0 {
			continue
		}
		artifactPath = replaceEnvWithValue(artifactPath, envMap)
		artifactPath = strings.TrimPrefix(artifactPath, "/")

		artifactPath := filepath.Join(s.workspace, artifactPath)
		isDir, err := fs.IsDir(artifactPath)
		if err != nil || !isDir {
			s.logger.Errorf("artifactPath is not exist  %s err: %s", artifactPath, err)
			continue
		}
		cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, artifactPath)
	}

	if len(cmdAndArtifactFullPaths) < 3 {
		return nil
	}

	temp, err := os.Create(tarName)
	if err != nil {
		s.logger.Errorf("failed to create %s err: %s", tarName, err)
		return err
	}
	err = temp.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s err: %s", tarName, err)
	}
	cmd := exec.Command("tar", cmdAndArtifactFullPaths...)
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		s.logger.Errorf("failed to compress %s err:%s", tarName, err)
		return err
	}

	objectKey := filepath.Join(s.spec.S3DestDir, s.spec.FileName)
	objectKey = filepath.ToSlash(objectKey)
	if err := client.Upload(s.spec.S3Storage.Bucket, tarName, objectKey); err != nil {
		return err
	}
	s.logger.Infof("Finish archive %s.", s.spec.FileName)
	return nil
}
