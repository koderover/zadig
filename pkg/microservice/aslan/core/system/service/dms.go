/*
 * Copyright 2025 The KodeRover Authors.
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
	dms "github.com/alibabacloud-go/dms-enterprise-20181101/v3/client"
	"github.com/alibabacloud-go/tea/tea"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	cloudservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/cloudservice"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/aliyun"
)

type DMSOrder struct {
	ID         int64  `json:"id"`
	PluginType string `json:"plugin_type"`
	Comment    string `json:"comment"`
	StatusCode string `json:"status_code"`
	StatusDesc string `json:"status_desc"`
	Committer  string `json:"committer"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
}

func ListDMSOrders(ctx *internalhandler.Context, id string, keyword string) ([]*DMSOrder, error) {
	dmsInfo, err := mongodb.NewCloudServiceColl().Find(ctx.Context, &mongodb.CloudServiceCollFindOption{Id: id})
	if err != nil {
		return nil, err
	}

	client, err := cloudservice.NewDMSClient(dmsInfo)
	if err != nil {
		return nil, err
	}

	listOrdersRequest := &dms.ListOrdersRequest{
		PluginType:      tea.String("DATA_CORRECT"),
		OrderResultType: tea.String("AS_ADMIN"),
		SearchContent:   tea.String(keyword),
		OrderStatus:     tea.String("RUNNING"),
	}

	orderList, err := client.ListOrders(listOrdersRequest)
	if err != nil {
		err = aliyun.HandleError(err)
		return nil, err
	}

	ret := make([]*DMSOrder, 0)
	for _, order := range orderList.Body.Orders.GetOrder() {
		ret = append(ret, &DMSOrder{
			ID:         tea.Int64Value(order.GetOrderId()),
			PluginType: tea.StringValue(order.GetPluginType()),
			Comment:    tea.StringValue(order.GetComment()),
			StatusCode: tea.StringValue(order.GetStatusCode()),
			StatusDesc: tea.StringValue(order.GetStatusDesc()),
			Committer:  tea.StringValue(order.GetCommitter()),
			CreateTime: tea.StringValue(order.GetCreateTime()),
			UpdateTime: tea.StringValue(order.GetLastModifyTime()),
		})
	}
	return ret, nil
}
