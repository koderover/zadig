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

package dingtalk

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/dingtalk"
	"github.com/koderover/zadig/pkg/tool/log"
)

type SubDepartmentAndUserIDsResponse struct {
	SubDepartmentIDs []int64  `json:"sub_department_ids"`
	UserIDs          []string `json:"user_ids"`
}

func GetDingTalkDepartment(id, departmentID string) (*SubDepartmentAndUserIDsResponse, error) {
	client, err := GetDingTalkClientByIMAppID(id)
	if err != nil {
		return nil, errors.Wrap(err, "get dingtalk client error")
	}
	deptID, err := strconv.Atoi(departmentID)
	if err != nil {
		return nil, errors.Wrap(err, "parse departmentID error")
	}
	userIDs, err := client.GetDepartmentUserIDs(deptID)
	if err != nil {
		return nil, errors.Wrap(err, "get department userIDs error")
	}
	subDepartments, err := client.GetSubDepartmentsInfo(deptID)
	if err != nil {
		return nil, errors.Wrap(err, "get sub departments error")
	}
	return &SubDepartmentAndUserIDsResponse{
		SubDepartmentIDs: func() (list []int64) {
			for _, department := range subDepartments {
				list = append(list, department.ID)
			}
			return
		}(),
		UserIDs: userIDs.UserIDList,
	}, nil
}

func GetDingTalkUserIDByMobile(id, mobile string) (string, error) {
	client, err := GetDingTalkClientByIMAppID(id)
	if err != nil {
		return "", errors.Wrap(err, "get dingtalk client error")
	}
	resp, err := client.GetUserIDByMobile(mobile)
	if err != nil {
		return "", errors.Wrap(err, "get userID by mobile error")
	}
	return resp.UserID, nil
}

type EventResponse struct {
	MsgSignature string `json:"msg_signature"`
	TimeStamp    string `json:"timeStamp"`
	Nonce        string `json:"nonce"`
	Encrypt      string `json:"encrypt"`
}

func EventHandler(appKey string, body []byte, signature, ts, nonce string) (*EventResponse, error) {
	logger := log.SugaredLogger().With("func", "DingTalkEventHandler").With("appKey", appKey)

	logger.Info("New dingtalk event received")
	info, err := mongodb.NewIMAppColl().GetDingTalkByAppKey(context.Background(), appKey)
	if err != nil {
		logger.Errorf("get dingtalk info error: %v", err)
		return nil, errors.Wrap(err, "get dingtalk info error")
	}

	d, err := NewDingTalkCrypto(info.DingTalkToken, info.DingTalkAesKey, info.DingTalkAppKey)
	if err != nil {
		logger.Errorf("new dingtalk crypto error: %v", err)
		return nil, errors.Wrap(err, "new dingtalk crypto error")
	}

	suc, err := d.GetDecryptMsg(signature, ts, nonce, gjson.Get(string(body), "encrypt").String())
	if err != nil {
		logger.Errorf("get decrypt msg error: %v", err)
		return nil, errors.Wrap(err, "get decrypt msg error")
	}
	log.Infof("suc: %s", suc)

	msg, err := d.GetEncryptMsg("success")
	if err != nil {
		logger.Errorf("get encrypt msg error: %v", err)
		return nil, errors.Wrap(err, "get encrypt msg error")
	}
	return &EventResponse{
		MsgSignature: msg["msg_signature"],
		TimeStamp:    msg["timeStamp"],
		Nonce:        msg["nonce"],
		Encrypt:      msg["encrypt"],
	}, nil
}

func GetDingTalkClientByIMAppID(id string) (*dingtalk.Client, error) {
	imApp, err := mongodb.NewIMAppColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, errors.Wrap(err, "db error")
	}
	if imApp.Type != setting.IMDingTalk {
		return nil, errors.Errorf("unexpected imApp type %s", imApp.Type)
	}
	return dingtalk.NewClient(imApp.DingTalkAppKey, imApp.DingTalkAppSecret), nil
}
