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
	"net/http"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

type Client struct {
	*lark.Client
}

func NewClient(appID, secret string) *Client {
	return &Client{
		Client: lark.NewClient(appID, secret,
			lark.WithReqTimeout(3*time.Second),
			lark.WithEnableTokenCache(true),
			lark.WithHttpClient(&http.Client{}),
		),
	}
}

func GetLarkClientByExternalApprovalID(id string) (*Client, error) {
	approval, err := mongodb.NewExternalApprovalColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, errors.Wrap(err, "get external approval data")
	}
	if approval.Type != setting.IMLark {
		return nil, errors.Errorf("unexpected approval type %s", approval.Type)
	}
	return NewClient(approval.AppID, approval.AppSecret), nil
}
