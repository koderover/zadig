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
	"fmt"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type ApproveMap struct {
	m map[string]*ApproveWithLock
	sync.RWMutex
}

type ApproveWithLock struct {
	Approval *commonmodels.NativeApproval
	sync.RWMutex
}

var GlobalApproveMap ApproveMap

func init() {
	GlobalApproveMap.m = make(map[string]*ApproveWithLock, 0)
}

func (c *ApproveMap) SetApproval(key string, value *ApproveWithLock) {
	c.Lock()
	defer c.Unlock()
	c.m[key] = value
}

func (c *ApproveMap) GetApproval(key string) (*ApproveWithLock, bool) {
	c.RLock()
	defer c.RUnlock()
	v, existed := c.m[key]
	return v, existed
}
func (c *ApproveMap) DeleteApproval(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.m, key)
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
