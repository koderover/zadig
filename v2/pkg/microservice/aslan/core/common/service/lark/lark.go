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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	config2 "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
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
	larkApprovalManagerMap ApprovalManagerMap
)

type ApprovalManagerMap struct {
	//m map[string]*ApprovalManager
}

type NodeUserApprovalResult map[string]map[string]*UserApprovalResult

type ApprovalManager struct {
	// key nodeID
	NodeMap     NodeUserApprovalResult
	NodeKeyMap  map[string]string
	RequestUUID map[string]struct{}
}

type UserApprovalResult struct {
	Result          string
	ApproveOrReject config.ApproveOrReject
	OperationTime   int64
}

func larkApprovalCacheKey(instanceID string) string {
	return fmt.Sprint("lark-approval-", instanceID)
}

func larkApprovalLockKey(instanceID string) string {
	return fmt.Sprint("lark-approval-lock-", instanceID)
}

func GetLarkApprovalInstanceManager(instanceID string) *ApprovalManager {
	redisMutex := cache.NewRedisLock(larkApprovalLockKey(instanceID))
	redisMutex.Lock()
	defer redisMutex.Unlock()

	approvalStr, _ := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).GetString(larkApprovalCacheKey(instanceID))
	if len(approvalStr) > 0 {
		approvalManager := &ApprovalManager{}
		err := json.Unmarshal([]byte(approvalStr), approvalManager)
		if err != nil {
			log.Errorf("unmarshal approval manager error: %v", err)
		}
		return approvalManager
	} else {
		approvalData := &ApprovalManager{
			NodeMap:     make(NodeUserApprovalResult),
			RequestUUID: make(map[string]struct{}),
			NodeKeyMap:  make(map[string]string),
		}
		bs, _ := json.Marshal(approvalData)
		err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Write(larkApprovalCacheKey(instanceID), string(bs), 0)
		if err != nil {
			log.Errorf("write approval manager error: %v", err)
		}
		return approvalData
	}
}

func RemoveLarkApprovalInstanceManager(instanceID string) {
	cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Delete(larkApprovalCacheKey(instanceID))
}

func GetNodeUserApprovalResults(instanceID, nodeID string) map[string]*UserApprovalResult {
	approvalManager := GetLarkApprovalInstanceManager(instanceID)
	return approvalManager.getNodeUserApprovalResults(nodeID)
}

func UpdateNodeUserApprovalResult(instanceID, nodeKey, nodeID, userID string, result *UserApprovalResult) {
	writeKey := fmt.Sprint("lark-approval-lock-write-", instanceID)
	writeMutex := cache.NewRedisLock(writeKey)
	writeMutex.Lock()
	defer writeMutex.Unlock()

	approvalManager := GetLarkApprovalInstanceManager(instanceID)
	approvalManager.updateNodeKeyMap(nodeKey, nodeID)
	approvalManager.updateNodeUserApprovalResult(nodeID, userID, result)
	bs, _ := json.Marshal(approvalManager)
	err := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Write(larkApprovalCacheKey(instanceID), string(bs), 0)
	if err != nil {
		log.Errorf("write approval manager error: %v", err)
	}
}

func (l *ApprovalManager) getNodeUserApprovalResults(nodeID string) map[string]*UserApprovalResult {
	m := make(map[string]*UserApprovalResult)
	if re, ok := l.NodeMap[nodeID]; !ok {
		return m
	} else {
		for userID, result := range re {
			m[userID] = result
		}
		return m
	}
}

func (l *ApprovalManager) updateNodeUserApprovalResult(nodeID, userID string, result *UserApprovalResult) {

	if _, ok := l.NodeMap[nodeID]; !ok {
		l.NodeMap[nodeID] = make(map[string]*UserApprovalResult)
	}
	if _, ok := l.NodeMap[nodeID][userID]; !ok && result != nil {
		switch result.Result {
		case ApprovalStatusApproved:
			l.NodeMap[nodeID][userID] = result
			result.ApproveOrReject = config.Approve
		case ApprovalStatusRejected:
			l.NodeMap[nodeID][userID] = result
			result.ApproveOrReject = config.Reject
		}
	}
	return
}

func (l *ApprovalManager) GetNodeKeyMap() map[string]string {
	m := make(map[string]string)
	for k, v := range l.NodeKeyMap {
		m[k] = v
	}
	return m
}

func (l *ApprovalManager) updateNodeKeyMap(nodeKey, nodeCustomKey string) {
	l.NodeKeyMap[nodeCustomKey] = nodeKey
}

func (l *ApprovalManager) CheckAndUpdateUUID(uuid string) bool {
	if _, ok := l.RequestUUID[uuid]; ok {
		return false
	}
	l.RequestUUID[uuid] = struct{}{}
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
