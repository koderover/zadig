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
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/lark"
	"github.com/koderover/zadig/pkg/tool/log"
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

type ApprovalManager struct {
	sync.RWMutex
	m           map[string]string
	requestUUID map[string]struct{}
}

func GetLarkApprovalManager(id string) *ApprovalManager {
	if larkApprovalManagerMap == nil {
		once.Do(func() {
			larkApprovalManagerMap = &ApprovalManagerMap{m: make(map[string]*ApprovalManager)}
		})
	}

	larkApprovalManagerMap.Lock()
	defer larkApprovalManagerMap.Unlock()

	if manager, ok := larkApprovalManagerMap.m[id]; !ok {
		larkApprovalManagerMap.m[id] = &ApprovalManager{
			m:           make(map[string]string),
			requestUUID: make(map[string]struct{}),
		}
		return larkApprovalManagerMap.m[id]
	} else {
		return manager
	}
}

func (l *ApprovalManager) GetInstanceStatus(id string) string {
	l.RLock()
	defer l.RUnlock()
	if status, ok := l.m[id]; !ok {
		return ApprovalStatusNotFound
	} else {
		return status
	}
}

func (l *ApprovalManager) UpdateInstanceStatus(id, status string) {
	l.Lock()
	defer l.Unlock()
	l.m[id] = status
}

func (l *ApprovalManager) RemoveInstance(id string) {
	l.Lock()
	defer l.Unlock()
	delete(l.m, id)
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

type CallbackData struct {
	UUID  string `json:"uuid"`
	Event struct {
		AppID               string `json:"app_id"`
		ApprovalCode        string `json:"approval_code"`
		InstanceCode        string `json:"instance_code"`
		InstanceOperateTime string `json:"instance_operate_time"`
		OperateTime         string `json:"operate_time"`
		Status              string `json:"status"`
		TenantKey           string `json:"tenant_key"`
		Type                string `json:"type"`
		UUID                string `json:"uuid"`
	} `json:"event"`
	Token string `json:"token"`
	Ts    string `json:"ts"`
	Type  string `json:"type"`
}

type EventHandlerResponse struct {
	Challenge string `json:"challenge"`
}

func EventHandler(appID, sign, ts, nonce, body string) (*EventHandlerResponse, error) {
	approval, err := mongodb.NewIMAppColl().GetByAppID(context.Background(), appID)
	if err != nil {
		return nil, errors.Wrap(err, "get approval by appID")
	}
	key := approval.EncryptKey
	approvalID := approval.ID.Hex()
	log.Infof("EventHandler: new request approval ID %s", approvalID)

	raw, err := larkDecrypt(gjson.Get(body, "encrypt").String(), key)
	if err != nil {
		return nil, errors.Wrap(err, "decrypt body")
	}

	// handle lark open platform webhook URL check request, which only need reply the challenge field.
	if sign == "" {
		return &EventHandlerResponse{Challenge: gjson.Get(raw, "challenge").String()}, nil
	}

	if sign != larkCalculateSignature(ts, nonce, key, body) {
		return nil, errors.New("check sign failed")
	}

	callback := &CallbackData{}
	err = json.Unmarshal([]byte(raw), callback)
	if err != nil {
		log.Errorf("unmarshal callback data failed: %v", err)
		return nil, errors.Wrap(err, "unmarshal")
	}

	if callback.Event.Type != "approval_instance" {
		log.Infof("get unknown callback event type %s, ignored", callback.Event.Type)
		return nil, nil
	}

	manager := GetLarkApprovalManager(approvalID)
	if !manager.CheckAndUpdateUUID(callback.UUID) {
		log.Infof("check existed request uuid %s, ignored", callback.UUID)
		return nil, nil
	}
	manager.UpdateInstanceStatus(callback.Event.InstanceCode, callback.Event.Status)
	log.Infof("update approval id: %s, instance code: %s, status: %s", approvalID, callback.Event.InstanceCode, callback.Event.Status)
	return nil, nil
}

func larkDecrypt(encrypt string, key string) (string, error) {
	buf, err := base64.StdEncoding.DecodeString(encrypt)
	if err != nil {
		return "", fmt.Errorf("base64StdEncode Error[%v]", err)
	}
	if len(buf) < aes.BlockSize {
		return "", errors.New("cipher  too short")
	}
	keyBs := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(keyBs[:sha256.Size])
	if err != nil {
		return "", fmt.Errorf("AESNewCipher Error[%v]", err)
	}
	iv := buf[:aes.BlockSize]
	buf = buf[aes.BlockSize:]
	// CBC mode always works in whole blocks.
	if len(buf)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(buf, buf)
	n := strings.Index(string(buf), "{")
	if n == -1 {
		n = 0
	}
	m := strings.LastIndex(string(buf), "}")
	if m == -1 {
		m = len(buf) - 1
	}
	return string(buf[n : m+1]), nil
}
func larkCalculateSignature(timestamp, nonce, encryptKey, bodystring string) string {
	var b strings.Builder
	b.WriteString(timestamp)
	b.WriteString(nonce)
	b.WriteString(encryptKey)
	b.WriteString(bodystring)
	bs := []byte(b.String())
	h := sha256.New()
	h.Write(bs)
	bs = h.Sum(nil)
	sig := fmt.Sprintf("%x", bs)
	return sig
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
