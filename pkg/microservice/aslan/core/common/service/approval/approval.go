/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package approval

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	config2 "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type GlobalApproveManager struct {
}

var GlobalApproveMap GlobalApproveManager

func approveKey(instanceID string) string {
	return fmt.Sprintf("native-approve-%s", instanceID)
}

func approveLockKey(instanceID string) string {
	return fmt.Sprintf("native-approve-lock-%s", instanceID)
}

func (c *GlobalApproveManager) SetApproval(key string, value *commonmodels.NativeApproval) {
	bytes, _ := json.Marshal(value)
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Write(approveKey(key), string(bytes), time.Duration(value.Timeout)*time.Minute)
}

func (c *GlobalApproveManager) GetApproval(key string) (*commonmodels.NativeApproval, bool) {
	value, err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).GetString(approveKey(key))
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf("get approval from redis error: %v", err)
		return nil, false
	}

	if errors.Is(err, redis.Nil) {
		return nil, false
	}

	approval := &commonmodels.NativeApproval{}
	err = json.Unmarshal([]byte(value), approval)
	if err != nil {
		log.Errorf("unmarshal approval error: %v", err)
		return nil, false
	}
	return approval, true
}

func (c *GlobalApproveManager) DeleteApproval(key string) {
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Delete(approveKey(key))
}

func (c *GlobalApproveManager) DoApproval(key, userName, userID, comment string, approve bool) (*commonmodels.NativeApproval, error) {
	redisMutex := cache.NewRedisLock(approveLockKey(key))
	redisMutex.Lock()
	defer redisMutex.Unlock()

	approvalData, ok := c.GetApproval(key)
	if !ok {
		return nil, fmt.Errorf("not found approval")
	}

	meetUser := false
	for _, user := range approvalData.ApproveUsers {
		if user.UserID != userID {
			continue
		}
		if user.RejectOrApprove != "" {
			return nil, fmt.Errorf("%s have %s already", userName, user.RejectOrApprove)
		}
		user.Comment = comment
		user.OperationTime = time.Now().Unix()
		if approve {
			user.RejectOrApprove = config.ApprovalStatusApprove
			meetUser = true
			break
		} else {
			user.RejectOrApprove = config.ApprovalStatusReject
			meetUser = true
			break
		}
	}
	if !meetUser {
		return nil, fmt.Errorf("user %s has no authority to Approve", userName)
	}

	c.SetApproval(key, approvalData)
	return approvalData, nil
}

// first return value is whether the approval is approved, second return value is whether the approval is rejected
func (c *GlobalApproveManager) IsApproval(key string) (bool, bool, *commonmodels.NativeApproval, error) {
	approval, ok := c.GetApproval(key)
	if !ok {
		return false, false, nil, fmt.Errorf("not found approval")
	}

	ApproveCount := 0
	for _, user := range approval.ApproveUsers {
		if user.RejectOrApprove == config.ApprovalStatusReject {
			approval.RejectOrApprove = config.ApprovalStatusReject
			return false, true, approval, nil
		}
		if user.RejectOrApprove == config.ApprovalStatusApprove {
			ApproveCount++
		}
	}
	if ApproveCount >= approval.NeededApprovers {
		approval.RejectOrApprove = config.ApprovalStatusApprove
		return true, false, approval, nil
	}
	return false, false, approval, nil
}
