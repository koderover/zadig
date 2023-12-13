/*
 * Copyright 2022 The KodeRover Authors.
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

package lark

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
)

const (
	// ApprovalStatusNotFound not defined by lark open api, it just means not found in local manager.
	ApprovalStatusNotFound = "NOTFOUND"

	ApprovalStatusPending  = "PENDING"
	ApprovalStatusApproved = "APPROVED"
	ApprovalStatusRejected = "REJECTED"
	ApprovalStatusCanceled = "CANCELED"
	ApprovalStatusDeleted  = "DELETED"
)

type DepartmentInfo struct {
	UserList          []*lark.UserInfo       `json:"user_list"`
	SubDepartmentList []*lark.DepartmentInfo `json:"sub_department_list"`
}

func GetLarkDepartment(approvalID, openID string) (*DepartmentInfo, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	userList, err := cli.ListUserFromDepartment(openID)
	if err != nil {
		return nil, errors.Wrap(err, "get user list")
	}
	departmentList, err := cli.ListSubDepartmentsInfo(openID)
	if err != nil {
		return nil, errors.Wrap(err, "get sub-department list")
	}
	return &DepartmentInfo{
		UserList:          userList,
		SubDepartmentList: departmentList,
	}, nil
}

func GetLarkAppContactRange(approvalID string) (*DepartmentInfo, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	reply, err := cli.ListAppContactRange()
	if err != nil {
		return nil, errors.Wrap(err, "list range")
	}

	var (
		userList       []*lark.UserInfo
		departmentList []*lark.DepartmentInfo
		err1, err2     error
		wg             sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		userList, err1 = getLarkUserInfoConcurrently(cli, reply.UserIDs, 10)
		wg.Done()
	}()
	go func() {
		departmentList, err2 = getLarkDepartmentInfoConcurrently(cli, reply.DepartmentIDs, 10)
		wg.Done()
	}()
	wg.Wait()
	if err1 != nil {
		return nil, errors.Wrap(err, "get user info")
	}
	if err2 != nil {
		return nil, errors.Wrap(err, "get department info")
	}
	return &DepartmentInfo{
		UserList:          userList,
		SubDepartmentList: departmentList,
	}, nil
}

func getLarkUserInfoConcurrently(client *lark.Client, idList []string, concurrentNum int) ([]*lark.UserInfo, error) {
	var reply []*lark.UserInfo
	idNum := len(idList)

	type result struct {
		*lark.UserInfo
		Err error
	}
	argCh := make(chan string, 100)
	resultCh := make(chan *result, 100)
	for i := 0; i < concurrentNum; i++ {
		go func() {
			for arg := range argCh {
				info, err := client.GetUserInfoByID(arg)
				resultCh <- &result{
					UserInfo: info,
					Err:      err,
				}
			}
		}()
	}
	for _, s := range idList {
		argCh <- s
	}
	close(argCh)

	for i := 0; i < idNum; i++ {
		re := <-resultCh
		if re.Err != nil {
			return nil, re.Err
		}
		reply = append(reply, re.UserInfo)
	}
	return reply, nil
}

func getLarkDepartmentInfoConcurrently(client *lark.Client, idList []string, concurrentNum int) ([]*lark.DepartmentInfo, error) {
	var reply []*lark.DepartmentInfo
	idNum := len(idList)

	type result struct {
		*lark.DepartmentInfo
		Err error
	}
	argCh := make(chan string, 100)
	resultCh := make(chan *result, 100)
	for i := 0; i < concurrentNum; i++ {
		go func() {
			for arg := range argCh {
				info, err := client.GetDepartmentInfoByID(arg)
				resultCh <- &result{
					DepartmentInfo: info,
					Err:            err,
				}
			}
		}()
	}
	for _, s := range idList {
		argCh <- s
	}
	close(argCh)

	for i := 0; i < idNum; i++ {
		re := <-resultCh
		if re.Err != nil {
			return nil, re.Err
		}
		reply = append(reply, re.DepartmentInfo)
	}
	return reply, nil
}

func GetLarkUserID(approvalID, queryType, queryValue string) (string, error) {
	switch queryType {
	case "email":
	default:
		return "", errors.New("invalid query type")
	}

	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return "", errors.Wrap(err, "get client")
	}
	return cli.GetUserOpenIDByEmailOrMobile(lark.QueryTypeEmail, queryValue)
}

var (
	once                   sync.Once
	larkApprovalManagerMap *ApprovalManagerMap
)

type ApprovalManagerMap struct {
	sync.RWMutex
	m map[string]*ApprovalManager
}

type NodeUserApprovalResult map[string]map[string]*UserApprovalResult

type ApprovalManager struct {
	sync.RWMutex
	// key nodeID
	nodeMap     NodeUserApprovalResult
	nodeKeyMap  map[string]string
	requestUUID map[string]struct{}
}

type UserApprovalResult struct {
	Result          string
	ApproveOrReject config.ApproveOrReject
	OperationTime   int64
}

func GetLarkApprovalInstanceManager(instanceID string) *ApprovalManager {
	if larkApprovalManagerMap == nil {
		once.Do(func() {
			larkApprovalManagerMap = &ApprovalManagerMap{m: make(map[string]*ApprovalManager)}
		})
	}

	larkApprovalManagerMap.Lock()
	defer larkApprovalManagerMap.Unlock()

	if manager, ok := larkApprovalManagerMap.m[instanceID]; !ok {
		larkApprovalManagerMap.m[instanceID] = &ApprovalManager{
			nodeMap:     make(NodeUserApprovalResult),
			requestUUID: make(map[string]struct{}),
			nodeKeyMap:  make(map[string]string),
		}
		return larkApprovalManagerMap.m[instanceID]
	} else {
		return manager
	}
}

func RemoveLarkApprovalInstanceManager(instanceID string) {
	larkApprovalManagerMap.Lock()
	defer larkApprovalManagerMap.Unlock()
	delete(larkApprovalManagerMap.m, instanceID)
}

func (l *ApprovalManager) GetNodeUserApprovalResults(nodeID string) map[string]*UserApprovalResult {
	l.RLock()
	defer l.RUnlock()

	m := make(map[string]*UserApprovalResult)
	if re, ok := l.nodeMap[nodeID]; !ok {
		return m
	} else {
		for userID, result := range re {
			m[userID] = result
		}
		return m
	}
}

func (l *ApprovalManager) UpdateNodeUserApprovalResult(nodeID, userID string, result *UserApprovalResult) {
	l.Lock()
	defer l.Unlock()

	if _, ok := l.nodeMap[nodeID]; !ok {
		l.nodeMap[nodeID] = make(map[string]*UserApprovalResult)
	}
	if _, ok := l.nodeMap[nodeID][userID]; !ok && result != nil {
		switch result.Result {
		case ApprovalStatusApproved:
			l.nodeMap[nodeID][userID] = result
			result.ApproveOrReject = config.Approve
		case ApprovalStatusRejected:
			l.nodeMap[nodeID][userID] = result
			result.ApproveOrReject = config.Reject
		}
	}
	return
}

func (l *ApprovalManager) GetNodeKeyMap() map[string]string {
	l.RLock()
	defer l.RUnlock()
	m := make(map[string]string)
	for k, v := range l.nodeKeyMap {
		m[k] = v
	}
	return m
}

func (l *ApprovalManager) UpdateNodeKeyMap(nodeKey, nodeCustomKey string) {
	l.Lock()
	defer l.Unlock()
	l.nodeKeyMap[nodeCustomKey] = nodeKey
}

func (l *ApprovalManager) CheckAndUpdateUUID(uuid string) bool {
	l.Lock()
	defer l.Unlock()
	if _, ok := l.requestUUID[uuid]; ok {
		return false
	}
	l.requestUUID[uuid] = struct{}{}
	return true
}

func GetLarkClientByIMAppID(id string) (*lark.Client, error) {
	approval, err := mongodb.NewIMAppColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, errors.Wrap(err, "get external approval data")
	}
	if approval.Type != setting.IMLark {
		return nil, errors.Errorf("unexpected approval type %s", approval.Type)
	}
	return lark.NewClient(approval.AppID, approval.AppSecret), nil
}
