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

package service

import (
	"fmt"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
)

func GetTestArtifactInfo(pipelineName, dir string, taskID int64, log *zap.SugaredLogger) (*commonmodels.S3Storage, []string, error) {
	fis := make([]string, 0)

	if storage, err := s3.FindDefaultS3(); err == nil {
		if storage.Subfolder != "" {
			storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, taskID, "artifact")
		} else {
			storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "artifact")
		}

		if files, err := s3.ListFiles(storage, dir, true); err == nil && len(files) > 0 {
			return storage.S3Storage, files, nil
		} else if err != nil {
			log.Errorf("GetTestArtifactInfo ListFiles err:%v", err)
		}
	} else {
		log.Errorf("GetTestArtifactInfo FindDefaultS3 err:%v", err)
	}

	return nil, fis, nil
}
