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
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type DownloadArtifactStep struct {
	spec       *step.StepDownloadArtifactSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewDownloadArtifactStep(spec interface{}, workspace string, envs, secretEnvs []string) (*DownloadArtifactStep, error) {
	archiveStep := &DownloadArtifactStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return archiveStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &archiveStep.spec); err != nil {
		return archiveStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return archiveStep, nil
}

func (s *DownloadArtifactStep) Run(ctx context.Context) error {
	start := time.Now()
	defer func() {
		log.Infof("Download Artifact ended. Duration: %.2f seconds", time.Since(start).Seconds())
	}()

	envmaps := makeEnvMap(s.envs, s.secretEnvs)
	artifact := replaceEnvWithValue(s.spec.Artifact, envmaps)
	log.Infof("Start download artifact %s.", artifact)

	forcedPathStyle := true
	if s.spec.S3.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3.NewClient(s.spec.S3.Endpoint, s.spec.S3.Ak, s.spec.S3.Sk, s.spec.S3.Region, s.spec.S3.Insecure, forcedPathStyle)
	if err != nil {
		return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
	}

	objectKey := artifact
	if len(s.spec.S3.Subfolder) > 0 {
		objectKey = strings.TrimLeft(path.Join(s.spec.S3.Subfolder, artifact), "/")
	}

	destPath := path.Join(s.workspace, "artifact", artifact)
	err = client.Download(s.spec.S3.Bucket, objectKey, destPath)
	if err != nil {
		return fmt.Errorf("failed to download artifact from s3 bucketName: %s, objectKey: %s, destPath: %s, err: %s", s.spec.S3.Bucket, objectKey, destPath, err)
	}
	return nil
}
