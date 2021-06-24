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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/pkg/microservice/reaper/internal/s3"
	"github.com/koderover/zadig/pkg/tool/log"
)

func artifactsUpload(ctx *meta.Context, activeWorkspace string) error {
	var (
		err   error
		store *s3.S3
	)

	if ctx.StorageURI != "" {
		if store, err = s3.NewS3StorageFromEncryptedURI(ctx.StorageURI); err != nil {
			log.Errorf("artifactsUpload failed to create s3 storage err:%v", err)
			return err
		}
		if store.Subfolder != "" {
			store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, ctx.PipelineName, ctx.TaskID, "artifact")
		} else {
			store.Subfolder = fmt.Sprintf("%s/%d/%s", ctx.PipelineName, ctx.TaskID, "artifact")
		}
	}

	artifactPaths := ctx.GinkgoTest.ArtifactPaths
	for _, artifactPath := range artifactPaths {
		if len(artifactPath) == 0 {
			continue
		}

		artifactPath = strings.TrimPrefix(artifactPath, "/")

		artifactPath = filepath.Join(activeWorkspace, artifactPath)

		artifactFiles, err := ioutil.ReadDir(artifactPath)
		if err != nil || len(artifactFiles) == 0 {
			continue
		}

		for _, artifactFile := range artifactFiles {
			if artifactFile.IsDir() {
				continue
			}
			filePath := path.Join(artifactPath, artifactFile.Name())

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				continue
			}

			if store != nil {
				err = s3.Upload(
					context.Background(),
					store,
					filePath,
					fmt.Sprintf("%s/%s", artifactPath, artifactFile.Name()),
				)

				if err != nil {
					log.Errorf("artifactsUpload failed to upload package %s, err:%v", filePath, err)
					return err
				}
			}
		}
	}
	return nil
}
