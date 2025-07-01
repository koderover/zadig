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

package models

import (
	"github.com/koderover/zadig/v2/pkg/setting"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type IMApp struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Type       string             `json:"type" bson:"type"`
	Name       string             `json:"name" bson:"name"`
	UpdateTime int64              `json:"update_time" bson:"update_time"`

	// Lark fields
	AppID      string `json:"app_id" bson:"app_id"`
	AppSecret  string `json:"app_secret" bson:"app_secret"`
	EncryptKey string `json:"encrypt_key" bson:"encrypt_key"`
	// Deprecated: use LarkApprovalCodeList instead
	LarkDefaultApprovalCode string `json:"-" bson:"lark_default_approval_code"`
	// LarkApprovalCodeList is a json string save all approval definition code
	LarkApprovalCodeList map[string]string `json:"-" bson:"lark_approval_code_list"`
	// LarkApprovalCodeListCommon is used for any source approval
	// because LarkApprovalCodeList title is fixed "Zadig 工作流", so we need a common approval code list
	LarkApprovalCodeListCommon map[string]string     `json:"-" bson:"lark_approval_code_list_common"`
	LarkEventType              setting.LarkEventType `json:"lark_event_type" bson:"lark_event_type"`

	// DingTalk fields
	DingTalkAppKey                  string `json:"dingtalk_app_key" bson:"dingtalk_app_key"`
	DingTalkAppSecret               string `json:"dingtalk_app_secret" bson:"dingtalk_app_secret"`
	DingTalkAesKey                  string `json:"dingtalk_aes_key" bson:"dingtalk_aes_key"`
	DingTalkToken                   string `json:"dingtalk_token" bson:"dingtalk_token"`
	DingTalkDefaultApprovalFormCode string `json:"-" bson:"dingtalk_default_approval_form_code"`

	// workwx fields
	Host                     string `json:"host"                        bson:"host"`
	CorpID                   string `json:"corp_id"                     bson:"corp_id"`
	AgentID                  int    `json:"agent_id"                    bson:"agent_id"`
	AgentSecret              string `json:"agent_secret"                bson:"agent_secret"`
	WorkWXApprovalTemplateID string `json:"workwx_approval_template_id" bson:"workwx_approval_template_id"`
	WorkWXToken              string `json:"workwx_token"                bson:"workwx_token"`
	WorkWXAESKey             string `json:"workwx_aes_key"              bson:"workwx_aes_key"`
}

func (IMApp) TableName() string {
	return "im_app"
}
