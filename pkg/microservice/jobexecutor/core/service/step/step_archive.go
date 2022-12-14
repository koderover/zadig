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
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/types/step"
	"gopkg.in/yaml.v2"
)

type ArchiveStep struct {
	spec       *step.StepArchiveSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewArchiveStep(spec interface{}, workspace string, envs, secretEnvs []string) (*ArchiveStep, error) {
	archiveStep := &ArchiveStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return archiveStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &archiveStep.spec); err != nil {
		return archiveStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return archiveStep, nil
}

func (s *ArchiveStep) Run(ctx context.Context) error {
	start := time.Now()
	defer func() {
		log.Infof("Archive ended. Duration: %.2f seconds", time.Since(start).Seconds())
	}()

	for _, upload := range s.spec.UploadDetail {
		log.Infof("Start archive %s.", upload.FilePath)
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

		envmaps := make(map[string]string)
		for _, env := range s.envs {
			kv := strings.Split(env, "=")
			if len(kv) != 2 {
				continue
			}
			envmaps[kv[0]] = kv[1]
		}
		for _, secretEnv := range s.secretEnvs {
			kv := strings.Split(secretEnv, "=")
			if len(kv) != 2 {
				continue
			}
			envmaps[kv[0]] = kv[1]
		}

		upload.AbsFilePath = fmt.Sprintf("$WORKSPACE/%s", upload.FilePath)
		upload.AbsFilePath = replaceEnvWithValue(upload.AbsFilePath, envmaps)
		upload.DestinationPath = replaceEnvWithValue(upload.DestinationPath, envmaps)

		if len(s.spec.S3.Subfolder) > 0 {
			upload.DestinationPath = strings.TrimLeft(path.Join(s.spec.S3.Subfolder, upload.DestinationPath), "/")
		}

		info, err := os.Stat(upload.AbsFilePath)
		if err != nil {
			return fmt.Errorf("failed to upload file path [%s] to destination [%s], the error is: %s", upload.AbsFilePath, upload.DestinationPath, err)
		}
		// if the given path is a directory
		if info.IsDir() {
			err := client.UploadDir(s.spec.S3.Bucket, upload.AbsFilePath, upload.DestinationPath)
			if err != nil {
				return err
			}
		} else {
			key := filepath.Join(upload.DestinationPath, info.Name())
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
	for key, value := range envs {
		strKey := fmt.Sprintf("$%s", key)
		ret = strings.ReplaceAll(ret, strKey, value)
	}
	return ret
}
