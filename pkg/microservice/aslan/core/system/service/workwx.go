/*
 * Copyright 2024 The KodeRover Authors.
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

package service

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/workwx"
	"go.uber.org/zap"
)

type GetWorkWxDepartmentResp struct {
	Departments []*workwx.Department `json:"departments"`
}

func GetWorkWxDepartment(appID string, departmentID int, log *zap.SugaredLogger) (*GetWorkWxDepartmentResp, error) {
	app, err := mongodb.NewIMAppColl().GetByID(context.Background(), appID)
	if err != nil {
		errStr := fmt.Sprintf("failed to find im app by id: %s, error: %s", appID, err)
		log.Errorf(errStr)
		return nil, fmt.Errorf(errStr)
	}

	client := workwx.NewClient(app.Host, app.CorpID, app.AgentID, app.AgentSecret)

	resp, err := client.ListDepartment(departmentID)
	if err != nil {
		errStr := fmt.Sprintf("failed to list department, err: %s", err)
		log.Errorf(errStr)
		return nil, fmt.Errorf(errStr)
	}

	return &GetWorkWxDepartmentResp{Departments: resp.Department}, nil
}

type GetWorkWxUsersResp struct {
	UserList []*workwx.UserBriefInfo `json:"user_list"`
}

func GetWorkWxUsers(appID string, departmentID int, log *zap.SugaredLogger) (*GetWorkWxUsersResp, error) {
	app, err := mongodb.NewIMAppColl().GetByID(context.Background(), appID)
	if err != nil {
		errStr := fmt.Sprintf("failed to find im app by id: %s, error: %s", appID, err)
		log.Errorf(errStr)
		return nil, fmt.Errorf(errStr)
	}

	// list of all user in a company is no longer supported
	if departmentID == 0 {
		errStr := fmt.Sprintf("the departmentID cannot be 0, it is now forbidden to list the user from the whole company")
		log.Errorf(errStr)
		return nil, fmt.Errorf(errStr)
	}

	client := workwx.NewClient(app.Host, app.CorpID, app.AgentID, app.AgentSecret)

	resp, err := client.ListDepartmentUsers(departmentID)
	if err != nil {
		errStr := fmt.Sprintf("failed to list department users, err: %s", err)
		log.Errorf(errStr)
		return nil, fmt.Errorf(errStr)
	}

	return &GetWorkWxUsersResp{
		UserList: resp.UserList,
	}, nil
}

func ValidateWorkWXWebhook(id, timestamp, nonce, echoString, validationString string, log *zap.SugaredLogger) (string, error) {
	app, err := mongodb.NewIMAppColl().GetByID(context.Background(), id)
	if err != nil {
		errStr := fmt.Sprintf("failed to find im app by id: %s, error: %s", id, err)
		log.Errorf(errStr)
		return "", fmt.Errorf(errStr)
	}

	signatureOK := workwx.CallbackValidate(validationString, app.WorkWXToken, timestamp, nonce, echoString)
	if !signatureOK {
		return "", fmt.Errorf("cannon verify the signature: data corrupted")
	}

	plaintext, _, err := workwx.DecodeEncryptedMessage(app.WorkWXAESKey, echoString)
	return string(plaintext), err
}
