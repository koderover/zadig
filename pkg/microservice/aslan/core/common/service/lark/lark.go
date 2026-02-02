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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/pkg/errors"

	config2 "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	// ApprovalStatusNotFound not defined by lark open api, it just means not found in local manager.
	ApprovalStatusNotFound = "NOTFOUND"

	ApprovalStatusPending     = "PENDING"
	ApprovalStatusApproved    = "APPROVED"
	ApprovalStatusRejected    = "REJECTED"
	ApprovalStatusTransferred = "TRANSFERRED"
	ApprovalStatusDone        = "DONE"
	ApprovalStatusCanceled    = "CANCELED"
	ApprovalStatusDeleted     = "DELETED"
)

type DepartmentInfo struct {
	UserList          []*lark.UserInfo       `json:"user_list"`
	SubDepartmentList []*lark.DepartmentInfo `json:"sub_department_list"`
}

func GetLarkDepartment(approvalID, openID, userIDType string) (*DepartmentInfo, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	userList, err := cli.ListUserFromDepartment(openID, setting.LarkDepartmentOpenID, userIDType)
	if err != nil {
		return nil, errors.Wrap(err, "get user list")
	}
	departmentList, err := cli.ListSubDepartmentsInfo(openID, setting.LarkDepartmentOpenID, userIDType, false)
	if err != nil {
		return nil, errors.Wrap(err, "get sub-department list")
	}
	return &DepartmentInfo{
		UserList:          userList,
		SubDepartmentList: departmentList,
	}, nil
}

func GetLarkAppContactRange(approvalID, userIDType string) (*DepartmentInfo, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	reply, err := cli.ListAppContactRange(userIDType)
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
	util.Go(func() {
		userList, err1 = getLarkUserInfoConcurrently(cli, reply.UserIDs, userIDType, 10)
		wg.Done()
	})
	util.Go(func() {
		departmentList, err2 = getLarkDepartmentInfoConcurrently(cli, reply.DepartmentIDs, userIDType, 10)
		wg.Done()
	})
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

func ListAvailableLarkChat(imAppID string) ([]*commonmodels.LarkChat, error) {
	cli, err := GetLarkClientByIMAppID(imAppID)
	if err != nil {
		log.Errorf("failed to get lark client for id: %s, error: %s", imAppID, err)
		return nil, fmt.Errorf("failed to get lark client, error: %s", err)
	}

	chatList, _, _, err := cli.ListAvailableChats(100)
	if err != nil {
		log.Errorf("failed to list lark chats, error: %s", err)
		return nil, fmt.Errorf("failed to list lark chats, error: %s", err)
	}

	resp := make([]*commonmodels.LarkChat, 0)

	for _, chat := range chatList {
		resp = append(resp, &commonmodels.LarkChat{
			ChatID:   util.GetStringFromPointer(chat.ChatId),
			ChatName: util.GetStringFromPointer(chat.Name),
		})
	}

	return resp, nil
}

func SearchLarkChat(imAppID, query string) ([]*commonmodels.LarkChat, error) {
	cli, err := GetLarkClientByIMAppID(imAppID)
	if err != nil {
		log.Errorf("failed to get lark client for id: %s, error: %s", imAppID, err)
		return nil, fmt.Errorf("failed to get lark client, error: %s", err)
	}

	chatList, _, _, err := cli.SearchAvailableChats(query, 100, "")
	if err != nil {
		log.Errorf("failed to list lark chats, error: %s", err)
		return nil, fmt.Errorf("failed to list lark chats, error: %s", err)
	}

	resp := make([]*commonmodels.LarkChat, 0)

	for _, chat := range chatList {
		resp = append(resp, &commonmodels.LarkChat{
			ChatID:   util.GetStringFromPointer(chat.ChatId),
			ChatName: util.GetStringFromPointer(chat.Name),
		})
	}

	return resp, nil
}

func ListLarkChatMembers(imAppID, chatID string) ([]*lark.UserInfo, error) {
	cli, err := GetLarkClientByIMAppID(imAppID)
	if err != nil {
		log.Errorf("failed to get lark client for id: %s, error: %s", imAppID, err)
		return nil, fmt.Errorf("failed to get lark client, error: %s", err)
	}

	chatMembers, err := cli.ListAllChatMembers(chatID)
	if err != nil {
		log.Errorf("failed to list lark chats, error: %s", err)
		return nil, fmt.Errorf("failed to list lark chats, error: %s", err)
	}

	resp := make([]*lark.UserInfo, 0)

	for _, member := range chatMembers {
		resp = append(resp, &lark.UserInfo{
			ID:   util.GetStringFromPointer(member.MemberId),
			Name: util.GetStringFromPointer(member.Name),
		})
	}

	return resp, nil
}

func getLarkUserInfoConcurrently(client *lark.Client, idList []string, userIDType string, concurrentNum int) ([]*lark.UserInfo, error) {
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
				info, err := client.GetUserInfoByID(arg, userIDType)
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

func getLarkDepartmentInfoConcurrently(client *lark.Client, idList []string, userIDType string, concurrentNum int) ([]*lark.DepartmentInfo, error) {
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
				info, err := client.GetDepartmentInfoByID(arg, userIDType)
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

func GetLarkUserID(approvalID, queryType, queryValue, userIDType string) (string, error) {
	switch queryType {
	case "email":
	default:
		return "", errors.New("invalid query type")
	}

	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return "", errors.Wrap(err, "get client")
	}
	userInfo, err := cli.GetUserIDByEmailOrMobile(lark.QueryTypeEmail, queryValue, userIDType)
	if err != nil {
		return "", err
	}
	return util.GetStringFromPointer(userInfo.UserId), nil
}

type LarkUserGroup struct {
	GroupID               string `json:"group_id"`
	GroupName             string `json:"group_name"`
	MemberUserCount       int    `json:"member_user_count"`
	MemberDepartmentCount int    `json:"member_department_count"`
	Description           string `json:"description"`
}

func GetLarkUserGroup(approvalID, groupID string) (*LarkUserGroup, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	userGroup, err := cli.GetUserGroup(groupID)
	if err != nil {
		return nil, fmt.Errorf("get user group error: %s", err)
	}

	return &LarkUserGroup{
		GroupID:               util.GetStringFromPointer(userGroup.Id),
		GroupName:             util.GetStringFromPointer(userGroup.Name),
		MemberUserCount:       util.GetIntFromPointer(userGroup.MemberUserCount),
		MemberDepartmentCount: util.GetIntFromPointer(userGroup.MemberDepartmentCount),
		Description:           util.GetStringFromPointer(userGroup.Description),
	}, nil
}

func GetLarkUserGroups(approvalID, queryType, pageToken string) ([]*LarkUserGroup, string, bool, error) {
	userGroupType := 0
	switch queryType {
	case "user_group":
		userGroupType = 1
	case "user_dynamic_group":
		userGroupType = 2
	default:
		return nil, "", false, errors.New("invalid query type")
	}

	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, "", false, errors.Wrap(err, "get client")
	}
	userGroups, pageToken, hasMore, err := cli.GetUserGroups(userGroupType, pageToken)
	if err != nil {
		return nil, "", false, fmt.Errorf("get user groups error: %s", err)
	}

	userGroupList := make([]*LarkUserGroup, 0)
	for _, userGroup := range userGroups {
		userGroupList = append(userGroupList, &LarkUserGroup{
			GroupID:               util.GetStringFromPointer(userGroup.Id),
			GroupName:             util.GetStringFromPointer(userGroup.Name),
			MemberUserCount:       util.GetIntFromPointer(userGroup.MemberUserCount),
			MemberDepartmentCount: util.GetIntFromPointer(userGroup.MemberDepartmentCount),
			Description:           util.GetStringFromPointer(userGroup.Description),
		})
	}
	return userGroupList, pageToken, hasMore, nil
}

func GetLarkUserGroupMembersInfo(approvalID, userGroupID, memberType, memberIDType, pageToken string) ([]*lark.UserInfo, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}
	members, _, _, err := cli.GetUserGroupMembers(userGroupID, memberType, memberIDType, pageToken)
	if err != nil {
		return nil, fmt.Errorf("get user group members error: %s", err)
	}

	userGroupMembers := make([]string, 0)
	for _, member := range members {
		userGroupMembers = append(userGroupMembers, util.GetStringFromPointer(member.MemberId))
	}

	if memberIDType == setting.LarkDepartmentID {
		departmentsUserInfos, err := GetLarkDepartmentUserInfos(approvalID, userGroupMembers)
		if err != nil {
			return nil, err
		}

		return departmentsUserInfos, nil
	} else {
		userInfos, err := GetLarkUserInfos(approvalID, setting.LarkUserOpenID, userGroupMembers)
		if err != nil {
			return nil, err
		}

		return userInfos, nil
	}
}

func GetLarkUserGroupMembers(approvalID, userGroupID, memberType, memberIDType, pageToken string) ([]string, string, bool, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, "", false, errors.Wrap(err, "get client")
	}
	members, pageToken, hasMore, err := cli.GetUserGroupMembers(userGroupID, memberType, memberIDType, pageToken)
	if err != nil {
		return nil, "", false, fmt.Errorf("get user group members error: %s", err)
	}

	userGroupMembers := make([]string, 0)
	for _, member := range members {
		userGroupMembers = append(userGroupMembers, util.GetStringFromPointer(member.MemberId))
	}
	return userGroupMembers, pageToken, hasMore, nil
}

func GetLarkUserInfos(approvalID, userIDType string, userIDs []string) ([]*lark.UserInfo, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}

	userList, err := getLarkUserInfoConcurrently(cli, userIDs, userIDType, 10)
	if err != nil {
		return nil, errors.Wrap(err, "get user info")
	}

	return userList, nil
}

func GetLarkDepartmentUserInfos(approvalID string, departmentIDs []string) ([]*lark.UserInfo, error) {
	cli, err := GetLarkClientByIMAppID(approvalID)
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}

	finalDepartmentIDList := departmentIDs
	for _, departmentID := range departmentIDs {
		departmentList, err := cli.ListSubDepartmentsInfo(departmentID, setting.LarkDepartmentID, setting.LarkUserOpenID, true)
		if err != nil {
			return nil, errors.Wrap(err, "get sub-department info")
		}

		for _, department := range departmentList {
			finalDepartmentIDList = append(finalDepartmentIDList, department.DepartmentID)
		}
	}

	resp := make([]*lark.UserInfo, 0)
	for _, departmentID := range finalDepartmentIDList {
		userList, err := cli.ListUserFromDepartment(departmentID, setting.LarkDepartmentID, setting.LarkUserOpenID)
		if err != nil {
			return nil, errors.Wrap(err, "get user from department")
		}
		resp = append(resp, userList...)
	}

	return resp, nil
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
	ApproveOrReject config.ApprovalStatus
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

func (l *ApprovalManager) updateNodeUserApprovalResult(nodeID, userID string, result *UserApprovalResult) {
	if _, ok := l.NodeMap[nodeID]; !ok {
		l.NodeMap[nodeID] = make(map[string]*UserApprovalResult)
	}
	switch result.Result {
	case ApprovalStatusApproved:
		l.NodeMap[nodeID][userID] = result
		result.ApproveOrReject = config.ApprovalStatusApprove
	case ApprovalStatusRejected:
		l.NodeMap[nodeID][userID] = result
		result.ApproveOrReject = config.ApprovalStatusReject
	case ApprovalStatusTransferred:
		l.NodeMap[nodeID][userID] = result
		result.ApproveOrReject = config.ApprovalStatusRedirect
	case ApprovalStatusDone:
		l.NodeMap[nodeID][userID] = result
		result.ApproveOrReject = config.ApprovalStatusDone
	}
	return
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
	if approval.Type != setting.IMLark && approval.Type != setting.IMLarkIntl {
		return nil, errors.Errorf("unexpected approval type %s", approval.Type)
	}
	return lark.NewClient(approval.AppID, approval.AppSecret, approval.Type), nil
}
