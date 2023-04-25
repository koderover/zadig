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

package meego

import (
	"errors"
	"fmt"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type CreateCommentReq struct {
	Content string `json:"content"`
}

type CreateCommentResp struct {
	Data         int64       `json:"data"`
	ErrorMessage string      `json:"err_msg"`
	ErrorCode    int         `json:"err_code"`
	Error        interface{} `json:"error"`
}

func (c *Client) Comment(projectKey, workItemTypeKey string, workItemID int64, comment string) (int64, error) {
	createCommentAPI := fmt.Sprintf("%s/open_api/%s/work_item/%s/%d/comment/create", c.Host, projectKey, workItemTypeKey, workItemID)

	request := &CreateCommentReq{
		Content: comment,
	}

	result := new(CreateCommentResp)

	_, err := httpclient.Post(createCommentAPI,
		httpclient.SetBody(request),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(result),
	)

	if err != nil {
		log.Errorf("failed to create comment, error: %s", err)
		return 0, err
	}

	if result.ErrorCode != 0 {
		errorMsg := fmt.Sprintf("error response when creating comment error code: %d, error message: %s, err: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errorMsg)
		return 0, errors.New(errorMsg)
	}

	return result.Data, nil
}
