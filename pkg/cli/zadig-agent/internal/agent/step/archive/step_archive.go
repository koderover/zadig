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
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type ArchiveStep struct {
	spec       *step.StepArchiveSpec
	envs       []string
	secretEnvs []string
	workspace  string
	logger     *log.JobLogger
	dirs       *types.AgentWorkDirs
}

func NewArchiveStep(spec interface{}, dirs *types.AgentWorkDirs, envs, secretEnvs []string, logger *log.JobLogger) (*ArchiveStep, error) {
	archiveStep := &ArchiveStep{dirs: dirs, envs: envs, secretEnvs: secretEnvs, logger: logger}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return archiveStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &archiveStep.spec); err != nil {
		return archiveStep, fmt.Errorf("unmarshal spec %s to script spec failed", yamlBytes)
	}
	return archiveStep, nil
}

func (s *ArchiveStep) Run(ctx context.Context) error {
	start := time.Now()
	defer func() {
		s.logger.Infof(fmt.Sprintf("Archive ended. Duration: %.2f seconds", time.Since(start).Seconds()))
	}()

	for _, upload := range s.spec.UploadDetail {
		envmaps := helper.MakeEnvMap(s.envs, s.secretEnvs)
		s.logger.Infof(fmt.Sprintf("Start archive %s.", replaceEnvWithValue(upload.FilePath, envmaps)))

		if upload.DestinationPath == "" || upload.FilePath == "" {
			return nil
		}
		forcedPathStyle := true
		if s.spec.S3.Provider == setting.ProviderSourceAli {
			forcedPathStyle = false
		}
		client, err := s3.NewClient(s.spec.S3.Endpoint, s.spec.S3.Ak, s.spec.S3.Sk, s.spec.S3.Region, s.spec.S3.Insecure, forcedPathStyle)
		if err != nil {
			return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
		}

		upload.AbsFilePath = fmt.Sprintf("$WORKSPACE/%s", upload.FilePath)
		upload.AbsFilePath = replaceEnvWithValue(upload.AbsFilePath, envmaps)
		upload.DestinationPath = replaceEnvWithValue(upload.DestinationPath, envmaps)

		if runtime.GOOS == "windows" {
			upload.AbsFilePath = filepath.FromSlash(filepath.ToSlash(upload.AbsFilePath))
			upload.AbsFilePath = strings.TrimSpace(upload.AbsFilePath)
		}

		if len(s.spec.S3.Subfolder) > 0 {
			upload.DestinationPath = strings.TrimLeft(path.Join(s.spec.S3.Subfolder, upload.DestinationPath), "/")
		}
		info, err := os.Stat(upload.AbsFilePath)
		if err != nil {
			return fmt.Errorf("failed to upload file path [%s] to destination [%s], the error is: %w", upload.AbsFilePath, upload.DestinationPath, err)
		}
		upload.DestinationPath = filepath.ToSlash(upload.DestinationPath)
		// if the given path is a directory
		if info.IsDir() {
			err := client.UploadDir(s.spec.S3.Bucket, upload.AbsFilePath, upload.DestinationPath)
			if err != nil {
				return err
			}
		} else {
			key := path.Join(upload.DestinationPath, info.Name())
			err := client.Upload(s.spec.S3.Bucket, upload.AbsFilePath, key)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func replaceEnvWithValue(str string, envs map[string]string) string {
	ret := str
	// Exec twice to render nested variables
	for i := 0; i < 2; i++ {
		for key, value := range envs {
			strKey := fmt.Sprintf("$%s", key)
			ret = strings.ReplaceAll(ret, strKey, value)
			strKey = fmt.Sprintf("${%s}", key)
			ret = strings.ReplaceAll(ret, strKey, value)
			strKey = fmt.Sprintf("%%%s%%", key)
			ret = strings.ReplaceAll(ret, strKey, value)
		}
	}
	return ret
}
