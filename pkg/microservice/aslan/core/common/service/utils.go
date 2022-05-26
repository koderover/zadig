/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

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

	notifyClient := notify.NewNotifyClient()
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

func SendErrorMessage(sender, title, requestID string, err error, log *zap.SugaredLogger) {
	content := fmt.Sprintf("错误信息: %s", err)
	SendMessage(sender, title, content, requestID, log)
}

func GetGitlabAddress(URL string) (string, error) {
	if !strings.Contains(URL, "https") && !strings.Contains(URL, "http") {
		return "", fmt.Errorf("url is illegal")
	}
	uri, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("url prase failed")
	}
	return fmt.Sprintf("%s://%s", uri.Scheme, uri.Host), nil
}

// parseOwnerRepoBranchPath parse owner、repo、branch and path from gitlab repo path
func parseOwnerRepoBranchPath(URL string) (string, string, string, string, string, string, error) {
	if !strings.Contains(URL, "https") && !strings.Contains(URL, "http") {
		return "", "", "", "", "", "", fmt.Errorf("url is illegal:%s", URL)
	}
	//适配公网的gitlab
	if strings.Contains(URL, "-") {
		URL = strings.Replace(URL, "-/", "", -1)
	}

	pathType := "tree"
	if strings.Contains(URL, "blob") {
		pathType = "blob"
	}

	urlPathArray := strings.Split(URL, "/")
	if len(urlPathArray) < 8 {
		return "", "", "", "", "", "", fmt.Errorf("url is illegal")
	}

	address, err := GetGitlabAddress(URL)
	if err != nil {
		return "", "", "", "", "", "", err
	}
	// 如果是非根文件夹或文件
	if strings.Contains(URL, "tree") || strings.Contains(URL, "blob") {
		pathIndex := strings.Index(URL, urlPathArray[6]) + len(urlPathArray[6]) + 1
		return address, urlPathArray[3], urlPathArray[4], urlPathArray[6], URL[pathIndex:], pathType, nil
	}
	return address, urlPathArray[3], urlPathArray[4], "", "", pathType, nil
}

// fillServiceRepoInfo ensures the value of repo is set
func fillServiceRepoInfo(svc *models.Service) error {
	if svc.RepoOwner != "" && svc.RepoName != "" && svc.BranchName != "" && svc.LoadPath != "" {
		svc.RepoNamespace = svc.GetRepoNamespace()
		return nil
	}
	var err error
	pathType := ""
	_, svc.RepoOwner, svc.RepoNamespace, svc.BranchName, svc.LoadPath, pathType, err = parseOwnerRepoBranchPath(svc.SrcPath)
	if err != nil {
		return err
	}

	if svc.Source == setting.SourceFromGitlab {
		svc.LoadFromDir = pathType == "tree"
	} else if svc.Source == setting.SourceFromGithub {
		svc.LoadFromDir = true
	}
	svc.RepoNamespace = svc.GetRepoNamespace()
	return nil
}
