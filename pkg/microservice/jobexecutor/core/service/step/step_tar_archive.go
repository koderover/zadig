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
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util/fs"
)

type TarArchiveStep struct {
	spec       *step.StepTarArchiveSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewTarArchiveStep(spec interface{}, workspace string, envs, secretEnvs []string) (*TarArchiveStep, error) {
	tarArchiveStep := &TarArchiveStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
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
	log.Infof("%s   Start tar archive %s.", time.Now().Format(setting.WorkflowTimeFormat), s.spec.FileName)
	forcedPathStyle := true
	if s.spec.S3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3.NewClient(s.spec.S3Storage.Endpoint, s.spec.S3Storage.Ak, s.spec.S3Storage.Sk, s.spec.S3Storage.Region, s.spec.S3Storage.Insecure, forcedPathStyle)
	if err != nil {
		if s.spec.IgnoreErr {
			log.Errorf("%s   failed to create s3 client to upload file, err: %s", time.Now().Format(setting.WorkflowTimeFormat), err)
			return nil
		} else {
			return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
		}
	}

	envMap := makeEnvMap(s.envs, s.secretEnvs)
	tarName := filepath.Join(s.spec.DestDir, s.spec.FileName)
	tarName = replaceEnvWithValue(tarName, envMap)
	s.spec.TarDir = replaceEnvWithValue(s.spec.TarDir, envMap)

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
		artifactPath = replaceEnvWithValue(artifactPath, envMap)
		if !s.spec.AbsResultDir {
			artifactPath = strings.TrimPrefix(artifactPath, "/")
			artifactPath = filepath.Join(s.workspace, artifactPath)
		}
		isDir, err := fs.IsDir(artifactPath)
		if err != nil || !isDir {
			log.Errorf("artifactPath is not exist  %s err: %s", artifactPath, err)
			continue
		}
		cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, artifactPath)
	}

	if len(cmdAndArtifactFullPaths) < 3 {
		return nil
	}

	temp, err := os.Create(tarName)
	if err != nil {
		if s.spec.IgnoreErr {
			log.Errorf("failed to create %s err: %s", tarName, err)
			return nil
		} else {
			return fmt.Errorf("failed to create %s err: %s", tarName, err)
		}
	}
	_ = temp.Close()
	cmd := exec.Command("tar", cmdAndArtifactFullPaths...)

	cmdOutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	outScanner := bufio.NewScanner(cmdOutReader)
	go func() {
		for outScanner.Scan() {
			fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), outScanner.Text())
		}
	}()

	cmdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	errScanner := bufio.NewScanner(cmdErrReader)
	go func() {
		for errScanner.Scan() {
			fmt.Printf("%s   %s\n", time.Now().Format(setting.WorkflowTimeFormat), errScanner.Text())
		}
	}()

	if err = cmd.Run(); err != nil {
		if s.spec.IgnoreErr {
			log.Errorf("%s   failed to compress %s, cmd: %s, err: %s", time.Now().Format(setting.WorkflowTimeFormat), tarName, cmd.String(), err)
			return nil
		} else {
			return fmt.Errorf("failed to compress %s, cmd: %s, err: %s", tarName, cmd.String(), err)
		}
	}

	objectKey := filepath.Join(s.spec.S3DestDir, s.spec.FileName)
	if err := client.Upload(s.spec.S3Storage.Bucket, tarName, objectKey); err != nil {
		if s.spec.IgnoreErr {
			log.Errorf("%s   failed to upload archive to s3, bucketName: %s, src: %s, objectKey: %s, err: %s", time.Now().Format(setting.WorkflowTimeFormat), s.spec.S3Storage.Bucket, tarName, objectKey, err)
			return nil
		} else {
			return fmt.Errorf("failed to upload archive to s3, bucketName: %s, src: %s, objectKey: %s, err: %s", s.spec.S3Storage.Bucket, tarName, objectKey, err)
		}
	}
	log.Infof("%s   Finish archive %s.", time.Now().Format(setting.WorkflowTimeFormat), s.spec.FileName)
	return nil
}
