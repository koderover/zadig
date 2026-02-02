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

package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	larktool "github.com/koderover/zadig/v2/pkg/tool/lark"
)

func GetLarkDepartment(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	approvalID, departmentID := c.Param("id"), c.Param("department_id")
	userIDType := c.Query("user_id_type")
	if userIDType == "" {
		userIDType = setting.LarkUserOpenID
	}
	if departmentID == "root" {
		ctx.Resp, ctx.RespErr = lark.GetLarkAppContactRange(approvalID, userIDType)
	} else {
		ctx.Resp, ctx.RespErr = lark.GetLarkDepartment(approvalID, departmentID, userIDType)
	}
}

func GetLarkUserID(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	userIDType := c.Query("user_id_type")
	if userIDType == "" {
		userIDType = setting.LarkUserOpenID
	}
	id, err := lark.GetLarkUserID(c.Param("id"), c.Query("type"), c.Query("value"), userIDType)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp = map[string]string{"id": id}
}

type GetLarkUserGroupsResp struct {
	UserGroups []*lark.LarkUserGroup `json:"user_groups"`
	PageToken  string                `json:"page_token"`
	HasMore    bool                  `json:"has_more"`
}

// @Summary 获取飞书用户组
// @Description
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id			    path		string							true	"飞书应用 ID"
// @Param 	type			query		string							true	"用户组类型"
// @Param 	page_token		query		string							false	"分页 token"
// @Success 200 			{object} 	GetLarkUserGroupsResp
// @Router /api/aslan/system/lark/{id}/user_group [get]
func GetLarkUserGroups(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	userGroups, pageToken, hasMore, err := lark.GetLarkUserGroups(c.Param("id"), c.Query("type"), c.Query("page_token"))
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = GetLarkUserGroupsResp{
		UserGroups: userGroups,
		PageToken:  pageToken,
		HasMore:    hasMore,
	}
}

type GetLarkUserGroupMembersResp struct {
	Members   []*larktool.UserInfo `json:"members"`
	PageToken string               `json:"page_token"`
	HasMore   bool                 `json:"has_more"`
}

// @Summary 获取飞书用户组成员
// @Description
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id			    path		string							true	"飞书应用 ID"
// @Param 	user_group_id	query		string							true	"用户组 ID"
// @Param 	member_type		query		string							true	"成员类型"
// @Param 	member_id_type	query		string							true	"成员 ID 类型"
// @Param 	page_token		query		string							true	"分页 token"
// @Success 200 			{object} 	GetLarkUserGroupMembersResp
// @Router /api/aslan/system/lark/{id}/user_group/members [get]
func GetLarkUserGroupMembers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	members, err := lark.GetLarkUserGroupMembersInfo(c.Param("id"), c.Query("user_group_id"), c.Query("member_type"), c.Query("member_id_type"), c.Query("page_token"))
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = GetLarkUserGroupMembersResp{Members: members}
}

type listChatResp struct {
	Chats []*models.LarkChat `json:"chats"`
}

func ListAvailableLarkChat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	chatList, err := lark.ListAvailableLarkChat(c.Param("id"))
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = listChatResp{Chats: chatList}
}

func SearchLarkChat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	chatList, err := lark.SearchLarkChat(c.Param("id"), c.Query("query"))
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = listChatResp{Chats: chatList}
}

type listMembersResp struct {
	Members []*larktool.UserInfo `json:"members"`
}

func ListLarkChatMembers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	members, err := lark.ListLarkChatMembers(c.Param("id"), c.Param("chat_id"))
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = listMembersResp{Members: members}
}
