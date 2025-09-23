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
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type TarArchiveStep struct {
	spec       *step.StepTarArchiveSpec
	envs       []string
	secretEnvs []string
	workspace  string
	logger     *log.JobLogger
	dirs       *types.AgentWorkDirs
}

func NewTarArchiveStep(spec interface{}, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*TarArchiveStep, error) {
	tarArchiveStep := &TarArchiveStep{dirs: dirs, workspace: dirs.Workspace, envs: envs, secretEnvs: secretEnvs, logger: logger}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return tarArchiveStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &tarArchiveStep.spec); err != nil {
		return tarArchiveStep, fmt.Errorf("unmarshal spec %s to tar archive spec failed", yamlBytes)
	}
	return tarArchiveStep, nil
}

func (s *TarArchiveStep) Run(ctx context.Context) error {
	if len(s.spec.ResultDirs) == 0 {
		return nil
	}
	s.logger.Infof(fmt.Sprintf("Start tar archive %s.", s.spec.FileName))
	client, err := s3.NewClient(s.spec.S3Storage.Endpoint, s.spec.S3Storage.Ak, s.spec.S3Storage.Sk, s.spec.S3Storage.Region, s.spec.S3Storage.Insecure, s.spec.S3Storage.Provider)
	if err != nil {
		if s.spec.IgnoreErr {
			s.logger.Errorf("failed to create s3 client to upload file, err: %s", err)
			return nil
		} else {
			return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
		}
	}

	envMap := util.MakeEnvMap(s.envs, s.secretEnvs)
	tarName := filepath.Join(s.spec.DestDir, s.spec.FileName)
	tarName = util.ReplaceEnvWithValue(tarName, envMap)
	s.spec.TarDir = util.ReplaceEnvWithValue(s.spec.TarDir, envMap)

	cmdAndArtifactFullPaths := make([]string, 0)
	cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, "-czf")
	cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, tarName)
	if s.spec.ChangeTarDir {
		cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, "--exclude", tarName, "-C", s.spec.TarDir)
	}

	for _, artifactPath := range s.spec.ResultDirs {
		if len(artifactPath) == 0 {
			continue
		}
		artifactPath = util.ReplaceEnvWithValue(artifactPath, envMap)
		if !s.spec.AbsResultDir {
			artifactPath = strings.TrimPrefix(artifactPath, "/")
			artifactPath = filepath.Join(s.workspace, artifactPath)
		}
		cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, artifactPath)
	}

	if len(cmdAndArtifactFullPaths) < 3 {
		return nil
	}

	temp, err := os.Create(tarName)
	if err != nil {
		if s.spec.IgnoreErr {
			s.logger.Errorf("failed to create %s err: %s", tarName, err)
			return nil
		} else {
			return fmt.Errorf("failed to create %s err: %s", tarName, err)
		}
	}
	err = temp.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s err: %s", tarName, err)
	}

	cmd := exec.Command("tar", cmdAndArtifactFullPaths...)
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		if s.spec.IgnoreErr {
			s.logger.Errorf("failed to compress %s, cmd: %s, err: %s", tarName, cmd.String(), err)
			return nil
		} else {
			return fmt.Errorf("failed to compress %s, cmd: %s, err: %s", tarName, cmd.String(), err)
		}
	}

	objectKey := filepath.Join(s.spec.S3DestDir, s.spec.FileName)
	objectKey = filepath.ToSlash(objectKey)
	if err := client.Upload(s.spec.S3Storage.Bucket, tarName, objectKey); err != nil {
		if s.spec.IgnoreErr {
			s.logger.Errorf("failed to upload archive to s3, bucketName: %s, src: %s, objectKey: %s, err: %s", s.spec.S3Storage.Bucket, tarName, objectKey, err)
			return nil
		} else {
			return fmt.Errorf("failed to upload archive to s3, bucketName: %s, src: %s, objectKey: %s, err: %s", s.spec.S3Storage.Bucket, tarName, objectKey, err)
		}
	}
	s.logger.Infof("Finish archive %s.", s.spec.FileName)
	return nil
}
