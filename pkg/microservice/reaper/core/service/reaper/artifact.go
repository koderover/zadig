/*
Copyright 2021 The KodeRover Authors.

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

package reaper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/pkg/microservice/reaper/internal/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util/fs"
)

func artifactsUpload(ctx *meta.Context, activeWorkspace string, artifactPaths []string, pluginType ...string) error {
	var (
		err   error
		store *s3.S3
	)

	if ctx.StorageURI != "" {
		if store, err = s3.NewS3StorageFromEncryptedURI(ctx.StorageURI, ctx.AesKey); err != nil {
			log.Errorf("artifactsUpload failed to create s3 storage err:%v", err)
			return err
		}
		if store.Subfolder != "" {
			store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, ctx.PipelineName, ctx.TaskID, "artifact")
		} else {
			store.Subfolder = fmt.Sprintf("%s/%d/%s", ctx.PipelineName, ctx.TaskID, "artifact")
		}
	}

	if len(pluginType) == 0 {
		tarName := setting.ArtifactResultOut
		cmdAndArtifactFullPaths := make([]string, 0)
		cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, "-czf")
		cmdAndArtifactFullPaths = append(cmdAndArtifactFullPaths, tarName)
		for _, artifactPath := range artifactPaths {
			if len(artifactPath) == 0 {
				continue
			}
			artifactPath = strings.TrimPrefix(artifactPath, "/")
			artifactPath := filepath.Join(activeWorkspace, artifactPath)
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
			log.Errorf("failed to create %s err: %s", tarName, err)
			return err
		}
		_ = temp.Close()
		cmd := exec.Command("tar", cmdAndArtifactFullPaths...)
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			log.Errorf("failed to compress %s err:%s", tarName, err)
			return err
		}
		if store != nil {
			objectKey := store.GetObjectPath(fmt.Sprintf("%s/%s", activeWorkspace, tarName))
			if err = s3FileUpload(store, temp.Name(), objectKey); err != nil {
				return err
			}
		}
		return nil
	}

	artifactPath := filepath.Join(activeWorkspace, artifactPaths[0])
	isDir, err := fs.IsDir(artifactPath)
	if err != nil {
		return err
	}
	if isDir {
		temp, err := os.CreateTemp("", "*artifact.tar.gz")
		if err != nil {
			log.Errorf("failed to create temp file %s", err)
			return err
		}

		_ = temp.Close()
		cmd := exec.Command("tar", "czf", temp.Name(), "-C", artifactPath, ".")
		cmd.Dir = artifactPath
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err = cmd.Run(); err != nil {
			log.Errorf("failed to compress artifact err:%s", err)
			return err
		}
		if store != nil {
			objectKey := store.GetObjectPath("artifact.tar.gz")
			if err = s3FileUpload(store, temp.Name(), objectKey); err != nil {
				return err
			}
		}
	} else {
		if store != nil {
			_, fileName := filepath.Split(artifactPath)
			objectKey := store.GetObjectPath(fileName)
			if err = s3FileUpload(store, artifactPath, objectKey); err != nil {
				return err
			}
		}
	}

	return nil
}

func s3FileUpload(store *s3.S3, sourceFile, objectKey string) error {
	forcedPathStyle := true
	if store.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("failed to create s3 client, error is: %s", err)
		return err
	}

	if err = s3client.Upload(store.Bucket, sourceFile, objectKey); err != nil {
		log.Errorf("Archive s3 upload err:%s", err)
		return err
	}
	return nil
}
