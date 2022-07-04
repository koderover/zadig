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
	"gopkg.in/yaml.v3"
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
	log.Infof("Start archive %s.", s.spec.FilePath)
	defer func() {
		log.Infof("Archive %s ended. Duration: %.2f seconds", s.spec.FilePath, time.Since(start).Seconds())
	}()

	if s.spec.DestinationPath == "" || s.spec.FilePath == "" {
		return nil
	}
	forcedPathStyle := true
	if s.spec.S3.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3.NewClient(s.spec.S3.Endpoint, s.spec.S3.Ak, s.spec.S3.Sk, s.spec.S3.Insecure, forcedPathStyle)
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

	s.spec.AbsFilePath = fmt.Sprintf("$WORKSPACE/%s", s.spec.FilePath)
	s.spec.AbsFilePath = replaceEnvWithValue(s.spec.AbsFilePath, envmaps)
	s.spec.DestinationPath = replaceEnvWithValue(s.spec.DestinationPath, envmaps)

	if len(s.spec.S3.Subfolder) > 0 {
		s.spec.DestinationPath = strings.TrimLeft(path.Join(s.spec.S3.Subfolder, s.spec.DestinationPath), "/")
	}

	info, err := os.Stat(s.spec.AbsFilePath)
	if err != nil {
		return fmt.Errorf("failed to upload file path [%s] to destination [%s], the error is: %s", s.spec.AbsFilePath, s.spec.DestinationPath, err)
	}
	// if the given path is a directory
	if info.IsDir() {
		err := client.UploadDir(s.spec.S3.Bucket, s.spec.AbsFilePath, s.spec.DestinationPath)
		if err != nil {
			return err
		}
	} else {
		key := filepath.Join(s.spec.DestinationPath, info.Name())
		err := client.Upload(s.spec.S3.Bucket, s.spec.AbsFilePath, key)
		if err != nil {
			return err
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
