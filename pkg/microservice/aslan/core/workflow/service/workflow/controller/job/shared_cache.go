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
	"fmt"
	"path"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	typesstep "github.com/koderover/zadig/v2/pkg/types/step"
)

func getSharedCacheStoreDir(cacheKey string) string {
	return path.Join(setting.SharedCacheStoreRoot, cacheKey)
}

func getSharedCacheMetadataFile(jobName string) string {
	return path.Join(setting.SharedCacheMetadataRoot, sanitizeSharedCacheSegment(jobName)+".json")
}

func getSharedCacheVersion(taskID int64, jobName string) string {
	return fmt.Sprintf("task-%d-%s", taskID, sanitizeSharedCacheSegment(jobName))
}

func getSharedCachePublishLockKey(cacheKey string) string {
	return "workflow-shared-cache-publish:" + strings.ReplaceAll(cacheKey, "/", ":")
}

func sanitizeSharedCacheSegment(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", ".", "-")
	return replacer.Replace(value)
}

func buildSharedCacheRestoreStep(stepName, jobName, cacheDir, cacheKey string) *commonmodels.StepTask {
	return &commonmodels.StepTask{
		Name:     stepName,
		JobName:  jobName,
		StepType: config.StepSharedCacheRestore,
		Spec: &typesstep.StepSharedCacheRestoreSpec{
			CacheDir:     cacheDir,
			StoreDir:     getSharedCacheStoreDir(cacheKey),
			MetadataFile: getSharedCacheMetadataFile(jobName),
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
			CacheDir:       cacheDir,
			StoreDir:       getSharedCacheStoreDir(cacheKey),
			MetadataFile:   getSharedCacheMetadataFile(jobName),
			Version:        getSharedCacheVersion(taskID, jobName),
			PublishLockKey: getSharedCachePublishLockKey(cacheKey),
			WorkflowName:   workflowName,
			JobName:        jobName,
			TaskID:         taskID,
			IgnoreErr:      true,
		},
	}
}
