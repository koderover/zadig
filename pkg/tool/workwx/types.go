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

package workwx

import "fmt"

const (
	getAccessTokenAPI               = "cgi-bin/gettoken"
	listDepartmentAPI               = "cgi-bin/department/list"
	listDepartmentUserAPI           = "cgi-bin/user/simplelist"
	getUserIDByPhoneAPI             = "cgi-bin/user/getuserid"
	createApprovalInstanceAPI       = "cgi-bin/oa/applyevent"
	createApprovalTemplateDetailAPI = "cgi-bin/oa/approval/create_template"
)

type EventType string

const (
	EventTypeApprovalChange EventType = "sys_approval_change"
)

type ApprovalRel int

const (
	ApprovalRelAnd ApprovalRel = 1
	ApprovalRelOr  ApprovalRel = 2
)

type ApprovalType int

const (
	// 审批人
	ApprovalTypeApprove ApprovalType = 1
	// 抄送人
	ApprovalTypeCC ApprovalType = 2
)

type ApprovalStatus int

// 1-审批中；2-已通过；3-已驳回；4-已撤销；6-通过后撤销；7-已删除；10-已支付
const (
	ApprovalStatusWaiting            ApprovalStatus = 1
	ApprovalStatusApproved           ApprovalStatus = 2
	ApprovalStatusRejected           ApprovalStatus = 3
	ApprovalStatusRecant             ApprovalStatus = 4
	ApprovalStatusApprovedThenRecant ApprovalStatus = 6
	ApprovalStatusDeleted            ApprovalStatus = 7
	ApprovalStatusPaid               ApprovalStatus = 10
)

type ApprovalNodeStatus int

// 1-审批中；2-同意；3-驳回；4-转审；11-退回给指定审批人；12-加签；13-同意并加签；14-办理；15-转交
const (
	ApprovalNodeStatusWaiting                ApprovalNodeStatus = 1
	ApprovalNodeStatusApproved               ApprovalNodeStatus = 2
	ApprovalNodeStatusRejected               ApprovalNodeStatus = 3
	ApprovalNodeStatusForwarded              ApprovalNodeStatus = 4
	ApprovalNodeStatusThrowBackForwarded     ApprovalNodeStatus = 11
	ApprovalNodeStatusAddApprover            ApprovalNodeStatus = 12
	ApprovalNodeStatusApprovedAndAddApprover ApprovalNodeStatus = 13
	ApprovalNodeStatusProcessing             ApprovalNodeStatus = 14
	ApprovalNodeStatusMoved                  ApprovalNodeStatus = 15
)

type ApprovalSubNodeStatus int

// 1-审批中；2-同意；3-驳回；4-转审；11-退回给指定审批人；12-加签；13-同意并加签；14-办理；15-转交
const (
	ApprovalSubNodeStatusWaiting                ApprovalSubNodeStatus = 1
	ApprovalSubNodeStatusApproved               ApprovalSubNodeStatus = 2
	ApprovalSubNodeStatusRejected               ApprovalSubNodeStatus = 3
	ApprovalSubNodeStatusForwarded              ApprovalSubNodeStatus = 4
	ApprovalSubNodeStatusThrowBackForwarded     ApprovalSubNodeStatus = 11
	ApprovalSubNodeStatusAddApprover            ApprovalSubNodeStatus = 12
	ApprovalSubNodeStatusApprovedAndAddApprover ApprovalSubNodeStatus = 13
	ApprovalSubNodeStatusProcessing             ApprovalSubNodeStatus = 14
	ApprovalSubNodeStatusMoved                  ApprovalSubNodeStatus = 15
)

type ControlType string

const (
	// 文本
	ControlTypeText ControlType = "Text"
	// 多行文本
	ControlTypeTextArea ControlType = "Textarea"
	// 数字
	ControlTypeNumber ControlType = "Number"
	// 说明文字
	ControlTypeTips ControlType = "Tips"
)

const (
	LanguageCN = "zh_CN"
	LanguageEN = "en"
)

type generalResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type GeneralText struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

const (
	errCodeOK = 0
)

func (r *generalResponse) ToError() error {
	if r.ErrCode != errCodeOK {
		return fmt.Errorf(r.ErrMsg)
	}
	return nil
}

type getAccessTokenResp struct {
	generalResponse `json:",inline"`

	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type ListDepartmentResp struct {
	generalResponse `json:",inline"`

	Department []*Department `json:"department"`
}

type Department struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	NameEN           string   `json:"name_en"`
	DepartmentLeader []string `json:"department_leader"`
	ParentID         int      `json:"parentid"`
	Order            int      `json:"order"`
}

type ListDepartmentUserResp struct {
	generalResponse `json:",inline"`

	UserList []*UserBriefInfo `json:"userlist"`
}

type UserBriefInfo struct {
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Department []int  `json:"department"`
	OpenUserID string `json:"open_userid"`
}

type FindUserByPhoneResp struct {
	generalResponse `json:",inline"`

	UserID string `json:"userid"`
}

type createApprovalInstanceReq struct {
	CreatorUserID       string             `json:"creator_userid"`
	TemplateID          string             `json:"template_id"`
	UseTemplateApprover int                `json:"use_template_approver"`
	ChooseDepartment    int                `json:"choose_department"`
	ApplyData           *ApprovalApplyData `json:"apply_data"`
	Process             *ApprovalNodes     `json:"process"`
	SummaryList         []*ApprovalSummary `json:"summary_list"`
}

type ApprovalApplyData struct {
	Contents []*ApplyDataContent `json:"contents"`
}

type ApplyDataContent struct {
	Control string      `json:"control"`
	Id      string      `json:"id"`
	Value   interface{} `json:"value"`
}

type TextApplyData struct {
	Text string `json:"text"`
}

type ApprovalNodes struct {
	NodeList []*ApprovalNode `json:"node_list"`
}

type ApprovalNode struct {
	Type     ApprovalType       `json:"type"                xml:"NodeType"    bson:"type"      yaml:"type"`
	ApvRel   ApprovalRel        `json:"apv_rel"             xml:"ApvRel"      bson:"apv_rel"   yaml:"apv_rel"`
	UserID   []string           `json:"userid"              xml:"-"           bson:"user_id"   yaml:"user_id"`
	Status   ApprovalNodeStatus `json:"status,omitempty"    xml:"SpStatus"    bson:"status"    yaml:"status"`
	SubNodes []*ApprovalSubNode `json:"sub_nodes,omitempty" xml:"SubNodeList" bson:"sub_nodes" yaml:"sub_nodes"`
}

type ApprovalSubNode struct {
	UserInfo struct {
		UserID string `json:"user_id" xml:"UserId" bson:"user_id" yaml:"user_id"`
	} `json:"user_info" xml:"UserInfo" bson:"user_info" yaml:"user_id"`
	Speech    string                `json:"speech"    xml:"Speech"   bson:"speech"    yaml:"speech"`
	Status    ApprovalSubNodeStatus `json:"status"    xml:"SpYj"     bson:"status"    yaml:"status"`
	Timestamp int64                 `json:"timestamp" xml:"Sptime"   bson:"timestamp" yaml:"timestamp"`
}

type ApprovalSummary struct {
	SummaryInfo []*GeneralText `json:"summary_info"`
}

type createApprovalInstanceResp struct {
	generalResponse `json:",inline"`

	ApprovalInstanceID string `json:"sp_no"`
}

type EncryptedWebhookMessage struct {
	ToUserName string `xml:"ToUserName"`
	AgentID    string `xml:"AgentID"`
	Encrypt    string `xml:"Encrypt"`
}

type DecodedWebhookMessage struct {
	ToUserName   string                  `xml:"ToUserName"`
	FromUserName string                  `xml:"FromUserName"`
	CreateTime   int64                   `xml:"CreateTime"`
	MsgType      string                  `xml:"MsgType"`
	Event        EventType               `xml:"Event"`
	AgentID      string                  `xml:"AgentID"`
	ApprovalInfo *ApprovalWebhookMessage `xml:"ApprovalInfo,omitempty"`
}

type ApprovalWebhookMessage struct {
	ID           string         `xml:"SpNo"           json:"id"`
	TemplateName string         `xml:"SpName"         json:"template_name"`
	TemplateID   string         `xml:"TemplateId"     json:"template_id"`
	Status       ApprovalStatus `xml:"SpStatus"       json:"status"`
	ApplyTime    int64          `xml:"ApplyTime"      json:"apply_time"`
	Applyer      struct {
		UserID       string `xml:"UserId"            json:"user_id"`
		DepartmentID string `xml:"Party"             json:"department_id"`
	} `xml:"Applyer" json:"applyer"`
	ProcessList []*ApprovalNode `xml:"ProcessList" json:"process_list"`
}

type ApprovalTemplateContent struct {
	Controls []*ApprovalControl `json:"controls"`
}

type ApprovalControl struct {
	Property *ApprovalControlProperty `json:"property"`
}

type ApprovalControlProperty struct {
	Type        ControlType    `json:"control"`
	ID          string         `json:"id"`
	Title       []*GeneralText `json:"title"`
	Placeholder []*GeneralText `json:"placeholder,omitempty"`
	Require     int            `json:"require"`
	UnPrint     int            `json:"un_print"`
}

type createApprovalTemplateResponse struct {
	generalResponse `json:",inline"`

	TemplateID string `json:"template_id"`
}
