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

type UserInfo struct {
	ID     string `json:"id" yaml:"id" bson:"id"`
	IDType string `json:"id_type" yaml:"id_type" bson:"id_type"`
	Name   string `json:"name" yaml:"name" bson:"name"`
	Avatar string `json:"avatar,omitempty" yaml:"avatar,omitempty" bson:"avatar,omitempty"`
	// IsExecutor marks if the user is the executor of the workflow
	IsExecutor bool `json:"is_executor" yaml:"is_executor" bson:"is_executor"`
}

type DepartmentInfo struct {
	ID           string `json:"id" yaml:"id" bson:"id"`
	DepartmentID string `json:"department_id" yaml:"department_id" bson:"department_id"`
	Name         string `json:"name" yaml:"name" bson:"name"`
}

type ContactRange struct {
	UserIDs       []string `json:"user_id_list"`
	DepartmentIDs []string `json:"department_id_list"`
	GroupIDs      []string `json:"group_id_list,omitempty"`
}

type pageInfo struct {
	token   string
	hasMore bool
}

type formData struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ApprovalInstanceData struct {
	ApprovalName *string `json:"approval_name,omitempty" yaml:"approval_name,omitempty" bson:"approval_name,omitempty"` // 审批名称

	StartTime *string `json:"start_time,omitempty" yaml:"start_time,omitempty" bson:"start_time,omitempty"` // 审批创建时间

	EndTime *string `json:"end_time,omitempty" yaml:"end_time,omitempty" bson:"end_time,omitempty"` // 审批完成时间，未完成为 0

	UserId *string `json:"user_id,omitempty" yaml:"user_id,omitempty" bson:"user_id,omitempty"` // 发起审批用户

	OpenId *string `json:"open_id,omitempty" yaml:"open_id,omitempty" bson:"open_id,omitempty"` // 发起审批用户 open id

	SerialNumber *string `json:"serial_number,omitempty" yaml:"serial_number,omitempty" bson:"serial_number,omitempty"` // 审批单编号

	DepartmentId *string `json:"department_id,omitempty" yaml:"department_id,omitempty" bson:"department_id,omitempty"` // 发起审批用户所在部门

	Status *string `json:"status,omitempty" yaml:"status,omitempty" bson:"status,omitempty"` // 审批实例状态

	Uuid *string `json:"uuid,omitempty" yaml:"uuid,omitempty" bson:"uuid,omitempty"` // 用户的唯一标识id

	Form *string `json:"form,omitempty" yaml:"form,omitempty" bson:"form,omitempty"` // json字符串，控件值详情见下方

	TaskList []*InstanceTask `json:"task_list,omitempty" yaml:"task_list,omitempty" bson:"task_list,omitempty"` // 审批任务列表

	ModifiedInstanceCode *string `json:"modified_instance_code,omitempty" yaml:"modified_instance_code,omitempty" bson:"modified_instance_code,omitempty"` // 修改的原实例 code,仅在查询修改实例时显示该字段

	RevertedInstanceCode *string `json:"reverted_instance_code,omitempty" yaml:"reverted_instance_code,omitempty" bson:"reverted_instance_code,omitempty"` // 撤销的原实例 code,仅在查询撤销实例时显示该字段

	ApprovalCode *string `json:"approval_code,omitempty" yaml:"approval_code,omitempty" bson:"approval_code,omitempty"` // 审批定义 Code

	Reverted *bool `json:"reverted,omitempty" yaml:"reverted,omitempty" bson:"reverted,omitempty"` // 单据是否被撤销

	InstanceCode *string `json:"instance_code,omitempty" yaml:"instance_code,omitempty" bson:"instance_code,omitempty"` // 审批实例 Code

	CommentList []*InstanceComment `json:"comment_list,omitempty" yaml:"comment_list,omitempty" bson:"comment_list,omitempty"` // 评论列表

	Timeline []*InstanceTimeline `json:"timeline,omitempty" yaml:"timeline,omitempty" bson:"timeline,omitempty"` // 审批动态

	// 以下是 Zadig 添加的用于展示的字段
	UserName   *string `json:"user_name,omitempty" yaml:"user_name,omitempty" bson:"user_name,omitempty"`     // 发起人姓名
	UserAvatar *string `json:"user_avatar,omitempty" yaml:"user_avatar,omitempty" bson:"user_avatar,omitempty"` // 发起人头像
}

type InstanceTask struct {
	Id *string `json:"id,omitempty" yaml:"id,omitempty" bson:"id,omitempty"` // task id

	UserId *string `json:"user_id,omitempty" yaml:"user_id,omitempty" bson:"user_id,omitempty"` // 审批人的用户id，自动通过、自动拒绝 时为空

	OpenId *string `json:"open_id,omitempty" yaml:"open_id,omitempty" bson:"open_id,omitempty"` // 审批人 open id

	Status *string `json:"status,omitempty" yaml:"status,omitempty" bson:"status,omitempty"` // 任务状态

	NodeId *string `json:"node_id,omitempty" yaml:"node_id,omitempty" bson:"node_id,omitempty"` // task 所属节点 id

	NodeName *string `json:"node_name,omitempty" yaml:"node_name,omitempty" bson:"node_name,omitempty"` // task 所属节点名称

	CustomNodeId *string `json:"custom_node_id,omitempty" yaml:"custom_node_id,omitempty" bson:"custom_node_id,omitempty"` // task 所属节点自定义 id, 如果没设置自定义 id, 则不返回该字段

	Type *string `json:"type,omitempty" yaml:"type,omitempty" bson:"type,omitempty"` // 审批方式

	StartTime *string `json:"start_time,omitempty" yaml:"start_time,omitempty" bson:"start_time,omitempty"` // task 开始时间

	EndTime *string `json:"end_time,omitempty" yaml:"end_time,omitempty" bson:"end_time,omitempty"` // task 完成时间, 未完成为 0

	// 以下是 Zadig 添加的用于展示的字段
	UserName   *string `json:"user_name,omitempty" yaml:"user_name,omitempty" bson:"user_name,omitempty"`     // 审批人姓名
	UserAvatar *string `json:"user_avatar,omitempty" yaml:"user_avatar,omitempty" bson:"user_avatar,omitempty"` // 审批人头像
}

type InstanceComment struct {
	Id *string `json:"id,omitempty" yaml:"id,omitempty" bson:"id,omitempty"` // 评论 id

	UserId *string `json:"user_id,omitempty" yaml:"user_id,omitempty" bson:"user_id,omitempty"` // 发表评论用户

	OpenId *string `json:"open_id,omitempty" yaml:"open_id,omitempty" bson:"open_id,omitempty"` // 发表评论用户 open id

	Comment *string `json:"comment,omitempty" yaml:"comment,omitempty" bson:"comment,omitempty"` // 评论内容

	CreateTime *string `json:"create_time,omitempty" yaml:"create_time,omitempty" bson:"create_time,omitempty"` // 1564590532967

	Files []*File `json:"files,omitempty" yaml:"files,omitempty" bson:"files,omitempty"` // 评论附件
}

type File struct {
	Url *string `json:"url,omitempty" yaml:"url,omitempty" bson:"url,omitempty"` // 附件路径

	FileSize *int `json:"file_size,omitempty" yaml:"file_size,omitempty" bson:"file_size,omitempty"` // 附件大小

	Title *string `json:"title,omitempty" yaml:"title,omitempty" bson:"title,omitempty"` // 附件标题

	Type *string `json:"type,omitempty" yaml:"type,omitempty" bson:"type,omitempty"` // 附件类别
}

type InstanceTimeline struct {
	Type *string `json:"type,omitempty" yaml:"type,omitempty" bson:"type,omitempty"` // 动态类型，不同类型 ext 内的 user_id_list 含义不一样

	CreateTime *string `json:"create_time,omitempty" yaml:"create_time,omitempty" bson:"create_time,omitempty"` // 发生时间

	UserId *string `json:"user_id,omitempty" yaml:"user_id,omitempty" bson:"user_id,omitempty"` // 动态产生用户

	OpenId *string `json:"open_id,omitempty" yaml:"open_id,omitempty" bson:"open_id,omitempty"` // 动态产生用户 open id

	UserIdList []string `json:"user_id_list,omitempty" yaml:"user_id_list,omitempty" bson:"user_id_list,omitempty"` // 被抄送人列表

	OpenIdList []string `json:"open_id_list,omitempty" yaml:"open_id_list,omitempty" bson:"open_id_list,omitempty"` // 被抄送人列表

	TaskId *string `json:"task_id,omitempty" yaml:"task_id,omitempty" bson:"task_id,omitempty"` // 产生动态关联的task_id

	Comment *string `json:"comment,omitempty" yaml:"comment,omitempty" bson:"comment,omitempty"` // 理由

	CcUserList []*InstanceCcUser `json:"cc_user_list,omitempty" yaml:"cc_user_list,omitempty" bson:"cc_user_list,omitempty"` // 抄送人列表

	Ext *string `json:"ext,omitempty" yaml:"ext,omitempty" bson:"ext,omitempty"` // 动态其他信息，json格式，目前包括 user_id_list, user_id，open_id_list，open_id

	NodeKey *string `json:"node_key,omitempty" yaml:"node_key,omitempty" bson:"node_key,omitempty"` // 产生task的节点key

	Files []*File `json:"files,omitempty" yaml:"files,omitempty" bson:"files,omitempty"` // 审批附件

	// 以下是 Zadig 添加的用于展示的字段
	UserName   *string `json:"user_name,omitempty" yaml:"user_name,omitempty" bson:"user_name,omitempty"`     // 动态产生用户姓名
	UserAvatar *string `json:"user_avatar,omitempty" yaml:"user_avatar,omitempty" bson:"user_avatar,omitempty"` // 动态产生用户头像
}

type InstanceCcUser struct {
	UserId *string `json:"user_id,omitempty" yaml:"user_id,omitempty" bson:"user_id,omitempty"` // 抄送人 user id

	CcId *string `json:"cc_id,omitempty" yaml:"cc_id,omitempty" bson:"cc_id,omitempty"` // 审批实例内抄送唯一标识

	OpenId *string `json:"open_id,omitempty" yaml:"open_id,omitempty" bson:"open_id,omitempty"` // 抄送人 open id

	// 以下是 Zadig 添加的用于展示的字段
	UserName   *string `json:"user_name,omitempty" yaml:"user_name,omitempty" bson:"user_name,omitempty"`     // 抄送人姓名
	UserAvatar *string `json:"user_avatar,omitempty" yaml:"user_avatar,omitempty" bson:"user_avatar,omitempty"` // 抄送人头像
}