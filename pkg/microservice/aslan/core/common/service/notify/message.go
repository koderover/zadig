/*
Copyright 2023 The KodeRover Authors.

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

package notify

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func SendErrorMessage(sender, title, requestID string, err error, log *zap.SugaredLogger) {
	content := fmt.Sprintf("错误信息: %s", err)
	SendMessage(sender, title, content, requestID, log)
}

func SendMessage(sender, title, content, requestID string, log *zap.SugaredLogger) {
	nf := &models.Notify{
		Type:     config.Message,
		Receiver: sender,
		Content: &models.MessageCtx{
			ReqID:   requestID,
			Title:   title,
			Content: content,
		},
		CreateTime: time.Now().Unix(),
		IsRead:     false,
	}

	notifyClient := NewNotifyClient()
	if err := notifyClient.CreateNotify(sender, nf); err != nil {
		log.Errorf("create message notify error: %v", err)
	}
}

func SendFailedTaskMessage(username, productName, name, requestID string, workflowType config.PipelineType, err error, log *zap.SugaredLogger) {
	title := "创建工作流任务失败"

	errStr := err.Error()
	_, messageMap := e.ErrorMessage(err)
	if description, isExist := messageMap["description"]; isExist {
		if desc, ok := description.(string); ok {
			errStr = desc
		}
	}

	content := fmt.Sprintf("%s, 创建人：%s, 项目：%s, 名称：%s, 错误信息：%s", title, username, productName, name, errStr)

	if username != setting.CronTaskCreator {
		SendMessage(username, title, content, requestID, log)
		return
	}
}
