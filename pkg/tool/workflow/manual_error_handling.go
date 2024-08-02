/*
Copyright 2024 The KodeRover Authors.

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

package workflow

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

type JobErrorDecision string

const (
	JobErrorDecisionReject JobErrorDecision = "reject"
	JobErrorDecisionIgnore JobErrorDecision = "ignore"
)

const (
	jobErrorHandlingDecisionFormat = "%s++%s++%s"
)

func workflowManualErrorHandlingCacheKey(workflowName, jobName string, taskID int64) string {
	return fmt.Sprintf("workflow-manual-error-handle-%s-%s-%d", workflowName, jobName, taskID)
}

func SetJobErrorHandlingDecision(workflowName, jobName string, taskID int64, decision JobErrorDecision, userID, username string) error {
	return cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Write(
		workflowManualErrorHandlingCacheKey(workflowName, jobName, taskID),
		fmt.Sprintf(jobErrorHandlingDecisionFormat, string(decision), userID, username),
		0,
	)
}

func GetJobErrorHandlingDecision(workflowName, jobName string, taskID int64) (JobErrorDecision, string, string, error) {
	resp, err := cache.NewRedisCache(config.RedisCommonCacheTokenDB()).GetString(workflowManualErrorHandlingCacheKey(workflowName, jobName, taskID))
	if err != nil {
		return "", "", "", err
	}

	// once the key is retrieved, we delete it in case someone just tried to re-run it. ignoring the error if there is one.
	_ = cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Delete(workflowManualErrorHandlingCacheKey(workflowName, jobName, taskID))

	handlingInfos := strings.Split(resp, "++")
	if len(handlingInfos) != 3 {
		return "", "", "", fmt.Errorf("the error handling info should only have 2 segments")
	}

	return JobErrorDecision(handlingInfos[0]), handlingInfos[1], handlingInfos[2], nil
}
