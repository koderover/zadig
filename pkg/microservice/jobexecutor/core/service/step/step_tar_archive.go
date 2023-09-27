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

package step

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service/meta"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/koderover/zadig/pkg/util/fs"
)

type TarArchiveStep struct {
	spec           *step.StepTarArchiveSpec
	envs           []string
	secretEnvs     []string
	dirs           *meta.ExecutorWorkDirs
	logger         *zap.SugaredLogger
	infrastructure string
}

func NewTararchiveStep(metaData *meta.JobMetaData, logger *zap.SugaredLogger) (*TarArchiveStep, error) {
	tarArchiveStep := &TarArchiveStep{dirs: metaData.Dirs, envs: metaData.Envs, secretEnvs: metaData.SecretEnvs, logger: logger}
	spec := metaData.Step.Spec
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return tarArchiveStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &tarArchiveStep.spec); err != nil {
		return tarArchiveStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return tarArchiveStep, nil
}

func (s *TarArchiveStep) Run(ctx context.Context) error {
	if len(s.spec.ResultDirs) == 0 {
		return nil
	}
	s.logger.Infof("Start tar archive %s.", s.spec.FileName)
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
	envMap := makeEnvMap(s.envs, s.secretEnvs)
	for _, artifactPath := range s.spec.ResultDirs {
		if len(artifactPath) == 0 {
			continue
		}
		artifactPath = replaceEnvWithValue(artifactPath, envMap)
		artifactPath = strings.TrimPrefix(artifactPath, "/")

		artifactPath := filepath.Join(s.dirs.Workspace, artifactPath)
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
	_ = temp.Close()
	cmd := exec.Command("tar", cmdAndArtifactFullPaths...)
	//cmd.Stderr = os.Stderr

	needPersistentLog := false
	var fileName string
	if s.infrastructure == setting.JobVMInfrastructure {
		fileName = s.dirs.JobLogPath
		needPersistentLog = true
	}

	wg := &sync.WaitGroup{}
	err = SetCmdStdout(cmd, fileName, s.secretEnvs, needPersistentLog, wg)
	if err != nil {
		s.logger.Errorf("set cmd stdout error: %v", err)
		return err
	}

	if err = cmd.Run(); err != nil {
		s.logger.Errorf("failed to compress %s err:%s", tarName, err)
		return err
	}
	wg.Wait()

	objectKey := filepath.Join(s.spec.S3DestDir, s.spec.FileName)
	if err := client.Upload(s.spec.S3Storage.Bucket, tarName, objectKey); err != nil {
		return err
	}
	s.logger.Infof("Finish archive %s.", s.spec.FileName)
	return nil
}
