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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

type DownloadArchiveStep struct {
	spec       *step.StepDownloadArchiveSpec
	envs       []string
	secretEnvs []string
	workspace  string
}

func NewDownloadArchiveStep(spec interface{}, workspace string, envs, secretEnvs []string) (*DownloadArchiveStep, error) {
	archiveStep := &DownloadArchiveStep{workspace: workspace, envs: envs, secretEnvs: secretEnvs}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return archiveStep, fmt.Errorf("marshal spec %+v failed", spec)
	}
	if err := yaml.Unmarshal(yamlBytes, &archiveStep.spec); err != nil {
		return archiveStep, fmt.Errorf("unmarshal spec %s to shell spec failed", yamlBytes)
	}
	return archiveStep, nil
}

func (s *DownloadArchiveStep) Run(ctx context.Context) error {
	start := time.Now()
	defer func() {
		log.Infof("Download Archive ended. Duration: %.2f seconds", time.Since(start).Seconds())
	}()

	envmaps := makeEnvMap(s.envs, s.secretEnvs)
	fileName := replaceEnvWithValue(s.spec.FileName, envmaps)
	s.spec.DestDir = replaceEnvWithValue(s.spec.DestDir, envmaps)
	log.Infof("Start download archive %s.", fileName)

	forcedPathStyle := true
	if s.spec.S3.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3.NewClient(s.spec.S3.Endpoint, s.spec.S3.Ak, s.spec.S3.Sk, s.spec.S3.Region, s.spec.S3.Insecure, forcedPathStyle)
	if err != nil {
		if s.spec.IgnoreErr {
			log.Errorf("failed to create s3 client to upload file, err: %s", err)
			return nil
		} else {
			return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
		}
	}

	destPath := path.Join(s.workspace, s.spec.DestDir, fileName)
	objectKey := strings.TrimLeft(path.Join(s.spec.ObjectPath, fileName), "/")
	if !s.spec.UnTar {
		err = client.Download(s.spec.S3.Bucket, objectKey, destPath)
		if err != nil {
			if s.spec.IgnoreErr {
				log.Errorf("download archive err, bucketName: %s, objectKey: %s, err: %v", s.spec.S3.Bucket, objectKey, err)
				return nil
			} else {
				return fmt.Errorf("failed to download archive from s3, bucketName: %s, objectKey: %s, destPath: %s, err: %s", s.spec.S3.Bucket, objectKey, destPath, err)
			}
		}
	} else {
		if sourceFilename, err := util.GenerateTmpFile(); err == nil {
			defer func() {
				_ = os.Remove(sourceFilename)
			}()
			err = client.Download(s.spec.S3.Bucket, objectKey, sourceFilename)
			if err != nil {
				if s.spec.IgnoreErr {
					log.Errorf("failed to download archive from s3, bucketName: %s, objectKey: %s, err: %v", s.spec.S3.Bucket, objectKey, err)
					return nil
				} else {
					return fmt.Errorf("failed to download archive from s3, bucketName: %s, objectKey: %s, source: %s, err: %s", s.spec.S3.Bucket, objectKey, sourceFilename, err)
				}
			}

			destPath = s.spec.DestDir
			if err = os.MkdirAll(destPath, os.ModePerm); err != nil {
				if s.spec.IgnoreErr {
					log.Errorf("failed to MkdirAll destPath %s, err: %s", destPath, err)
					return nil
				} else {
					return fmt.Errorf("failed to MkdirAll destPath %s, err: %s", destPath, err)
				}
			}
			out := bytes.NewBufferString("")
			cmd := exec.Command("tar", "xzf", sourceFilename, "-C", destPath)
			cmd.Stderr = out
			if err := cmd.Run(); err != nil {
				if s.spec.IgnoreErr {
					log.Errorf("cmd: %s, err: %s %v", cmd.String(), out.String(), err)
					return nil
				} else {
					return fmt.Errorf("cmd: %s, err: %s %v", cmd.String(), out.String(), err)
				}
			}
		} else {
			if s.spec.IgnoreErr {
				log.Errorf("failed to GenerateTmpFile, err: %s", err)
				return nil
			} else {
				return fmt.Errorf("failed to GenerateTmpFile, err: %s", err)
			}
		}
	}
	return nil
}
