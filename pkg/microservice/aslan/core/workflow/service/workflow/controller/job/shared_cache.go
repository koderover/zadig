/*
Copyright 2026 The KodeRover Authors.

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

package job

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	typesstep "github.com/koderover/zadig/v2/pkg/types/step"
)

func getSharedCacheStoreDir(cacheKey string) string {
	return path.Join(setting.SharedCacheStoreRoot, cacheKey)
}

func getSharedCacheMetadataFile(workflowName, jobName, cacheKey string) string {
	return path.Join(setting.SharedCacheMetadataRoot, sharedCacheShortHash(workflowName, jobName, cacheKey)+".json")
}

func getSharedCacheVersion(taskID int64, jobName string) string {
	return fmt.Sprintf("task-%d-%s-%s", taskID, uuid.NewString(), sharedCacheShortHash(jobName))
}

func getSharedCachePublishLeaseName(cacheKey string) string {
	return "workflow-shared-cache-publish-" + sharedCacheShortHash(cacheKey)
}

func sharedCacheShortHash(parts ...string) string {
	hash := sha1.Sum([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:8])
}

func buildSharedCacheRestoreStep(stepName, workflowName, jobName, cacheDir, cacheKey string, skipContent bool) *commonmodels.StepTask {
	return &commonmodels.StepTask{
		Name:     stepName,
		JobName:  jobName,
		StepType: config.StepSharedCacheRestore,
		Spec: &typesstep.StepSharedCacheRestoreSpec{
			CacheDir:     cacheDir,
			StoreDir:     getSharedCacheStoreDir(cacheKey),
			MetadataFile: getSharedCacheMetadataFile(workflowName, jobName, cacheKey),
			SkipContent:  skipContent,
			IgnoreErr:    true,
		},
	}
}

func buildSharedCachePublishStep(stepName, workflowName, jobName, cacheDir, cacheKey string, taskID int64) *commonmodels.StepTask {
	return &commonmodels.StepTask{
		Name:     stepName,
		JobName:  jobName,
		StepType: config.StepSharedCachePublish,
		Spec: &typesstep.StepSharedCachePublishSpec{
			CacheDir:             cacheDir,
			StoreDir:             getSharedCacheStoreDir(cacheKey),
			MetadataFile:         getSharedCacheMetadataFile(workflowName, jobName, cacheKey),
			Version:              getSharedCacheVersion(taskID, jobName),
			LeaseName:            getSharedCachePublishLeaseName(cacheKey),
			LeaseDurationSeconds: 30,
			WorkflowName:         workflowName,
			JobName:              jobName,
			TaskID:               taskID,
			IgnoreErr:            true,
		},
	}
}
