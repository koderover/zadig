/*
Copyright 2024 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lark

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/util"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func (client *Client) ListAvailableChats(pageSize int) ([]*larkim.ListChat, bool, string, error) {
	getChatReq := larkim.NewListChatReqBuilder().PageSize(pageSize).Build()

	apiResp, err := client.Im.Chat.List(context.TODO(), getChatReq)
	if err != nil {
		return nil, false, "", err
	}
	if !apiResp.Success() {
		return nil, false, "", apiResp.CodeError
	}

	return apiResp.Data.Items, util.GetBoolFromPointer(apiResp.Data.HasMore), util.GetStringFromPointer(apiResp.Data.PageToken), nil
}

func (client *Client) SearchAvailableChats(query string, pageSize int, pageToken string) ([]*larkim.ListChat, bool, string, error) {
	searchChatReqBuilder := larkim.NewSearchChatReqBuilder()
	if pageSize != 0 {
		if pageSize > 100 {
			return nil, false, "", fmt.Errorf("page size cannot be larger than 100")
		}
		searchChatReqBuilder.PageSize(pageSize)
	}

	if pageToken != "" {
		searchChatReqBuilder.PageToken(pageToken)
	}

	if query == "" {
		return make([]*larkim.ListChat, 0), false, "", nil
	}

	searchChatReqBuilder.Query(query)

	apiResp, err := client.Im.Chat.Search(context.TODO(), searchChatReqBuilder.Build())
	if err != nil {
		return nil, false, "", err
	}
	if !apiResp.Success() {
		return nil, false, "", apiResp.CodeError
	}

	return apiResp.Data.Items, util.GetBoolFromPointer(apiResp.Data.HasMore), util.GetStringFromPointer(apiResp.Data.PageToken), nil
}

func (client *Client) ListAllChatMembers(chatID string) ([]*larkim.ListMember, error) {
	searchChatMemberReqBuilder := larkim.NewGetChatMembersReqBuilder().ChatId(chatID).PageSize(100)

	apiResp, err := client.Im.ChatMembers.Get(context.TODO(), searchChatMemberReqBuilder.Build())
	if err != nil {
		return nil, err
	}
	if !apiResp.Success() {
		return nil, apiResp.CodeError
	}

	resp := make([]*larkim.ListMember, 0)
	for _, member := range apiResp.Data.Items {
		resp = append(resp, member)
	}

	hasMore := util.GetBoolFromPointer(apiResp.Data.HasMore)
	pageToken := util.GetStringFromPointer(apiResp.Data.PageToken)
	for hasMore {
		reqBuilder := larkim.NewGetChatMembersReqBuilder().ChatId(chatID).PageSize(2).PageToken(pageToken)
		apiResp, err := client.Im.ChatMembers.Get(context.TODO(), reqBuilder.Build())
		if err != nil {
			return nil, err
		}
		if !apiResp.Success() {
			return nil, apiResp.CodeError
		}

		for _, member := range apiResp.Data.Items {
			resp = append(resp, member)
		}

		hasMore = util.GetBoolFromPointer(apiResp.Data.HasMore)
		pageToken = util.GetStringFromPointer(apiResp.Data.PageToken)
	}

	return resp, nil
}
