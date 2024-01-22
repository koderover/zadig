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

type ApproveMap struct {
}

type ApproveWithLock struct {
	Approval *commonmodels.NativeApproval
}

var GlobalApproveMap ApproveMap

func approveKey(instanceID string) string {
	return fmt.Sprintf("native-approve-%s", instanceID)
}

func approveLockKey(instanceID string) string {
	return fmt.Sprintf("native-approve-lock-%s", instanceID)
}

func (c *ApproveMap) SetApproval(key string, value *ApproveWithLock) {
	bytes, _ := json.Marshal(value.Approval)
	log.Infof("----- setting approval data， key: %s, value: %s", key, string(bytes))
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Write(approveKey(key), string(bytes), time.Duration(value.Approval.Timeout)*time.Second)
}

func (c *ApproveMap) GetApproval(key string) (*ApproveWithLock, bool) {
	value, err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).GetString(approveKey(key))
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf("get approval from redis error: %v", err)
		return nil, false
	}

	if errors.Is(err, redis.Nil) {
		return nil, false
	}

	log.Infof("--- get approval data is %s", value)

	approval := &ApproveWithLock{}
	err = json.Unmarshal([]byte(value), approval)
	if err != nil {
		log.Errorf("unmarshal approval error: %v", err)
		return nil, false
	}
	return approval, true
}

func (c *ApproveMap) DeleteApproval(key string) {
	log.Infof("------- deleting approval key: %s", key)
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Delete(approveKey(key))
}

func (c *ApproveMap) DoApproval(key, userName, userID, comment string, approval bool) error {
	redisMutex := cache.NewRedisLock(approveLockKey(key))
	redisMutex.Lock()
	defer redisMutex.Unlock()

	approvalData, ok := c.GetApproval(key)
	if !ok {
		return fmt.Errorf("not found approval")
	}
	err := approvalData.doApproval(userName, userID, comment, approval)
	if err != nil {
		return err
	}
	c.SetApproval(key, approvalData)
	return nil
}

func (c *ApproveMap) IsApproval(key string) (bool, int, error) {
	approval, ok := c.GetApproval(key)
	if !ok {
		return false, 0, fmt.Errorf("not found approval")
	}
	log.Infof("------- approval data :%+v", *approval.Approval)
	return approval.isApproval()
}

func (c *ApproveWithLock) isApproval() (bool, int, error) {
	ApproveCount := 0
	for _, user := range c.Approval.ApproveUsers {
		if user.RejectOrApprove == config.Reject {
			c.Approval.RejectOrApprove = config.Reject
			return false, ApproveCount, fmt.Errorf("%s reject this task", user.UserName)
		}
		if user.RejectOrApprove == config.Approve {
			ApproveCount++
		}
	}
	if ApproveCount >= c.Approval.NeededApprovers {
		c.Approval.RejectOrApprove = config.Approve
		return true, ApproveCount, nil
	}
	return false, ApproveCount, nil
}

func (c *ApproveWithLock) doApproval(userName, userID, comment string, appvove bool) error {
	for _, user := range c.Approval.ApproveUsers {
		if user.UserID != userID {
			continue
		}
		if user.RejectOrApprove != "" {
			return fmt.Errorf("%s have %s already", userName, user.RejectOrApprove)
		}
		user.Comment = comment
		user.OperationTime = time.Now().Unix()
		if appvove {
			user.RejectOrApprove = config.Approve
			return nil
		} else {
			user.RejectOrApprove = config.Reject
			return nil
		}
	}
	return fmt.Errorf("user %s has no authority to Approve", userName)
}
