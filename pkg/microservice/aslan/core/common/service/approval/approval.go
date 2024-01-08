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
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/redis/go-redis/v9"

	config2 "github.com/koderover/zadig/v2/pkg/config"

	"github.com/koderover/zadig/v2/pkg/tool/cache"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type ApproveMap struct {
	m map[string]*ApproveWithLock
	//sync.RWMutex
}

type ApproveWithLock struct {
	Approval *commonmodels.NativeApproval
	sync.RWMutex
}

var GlobalApproveMap ApproveMap

func init() {
	GlobalApproveMap.m = make(map[string]*ApproveWithLock, 0)
}

func approveKey(instanceID string) string {
	return fmt.Sprintf("normal-approve-%s", instanceID)
}

func (c *ApproveMap) SetApproval(key string, value *ApproveWithLock) {
	bytes, _ := json.Marshal(value.Approval)
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Write(approveKey(key), string(bytes), 0)
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

	approval := &ApproveWithLock{}
	err = json.Unmarshal([]byte(value), approval)
	if err != nil {
		log.Errorf("unmarshal approval error: %v", err)
		return nil, false
	}
	return approval, true
}

func (c *ApproveMap) DeleteApproval(key string) {
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Delete(approveKey(key))
}

func (c *ApproveWithLock) IsApproval() (bool, int, error) {
	c.Lock()
	defer c.Unlock()
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

func (c *ApproveWithLock) DoApproval(userName, userID, comment string, appvove bool) error {
	c.Lock()
	defer c.Unlock()
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
