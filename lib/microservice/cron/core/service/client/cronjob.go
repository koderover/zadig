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

package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/koderover/zadig/lib/microservice/cron/core/service"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/rsa"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// TriggerCleanjobs ...
func (c *Client) TriggerCleanjobs(log *xlog.Logger) error {
	url := fmt.Sprintf("%s/cron/cron/cleanjob", c.ApiBase)
	log.Info("start clean jobs..")
	err := c.sendRequest(url)
	if err != nil {
		log.Errorf("trigger clean jobs error :%v", err)
	}

	return c.TriggerCleanconfigmaps(log)
}

// TriggerCleanconfigmaps ...
func (c *Client) TriggerCleanconfigmaps(log *xlog.Logger) error {
	url := fmt.Sprintf("%s/cron/cron/cleanconfigmap", c.ApiBase)
	log.Info("start clean configmaps..")
	err := c.sendRequest(url)
	if err != nil {
		log.Errorf("trigger clean configmaps error :%v", err)
	}
	return err
}

// RunPipelineTask ...
func (c *Client) RunPipelineTask(args *service.TaskArgs, log *xlog.Logger) error {
	url := fmt.Sprintf("%s/workflow/v2/tasks", c.ApiBase)
	log.Info("start run scheduled task..")
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}
	result, err := c.sendPostRequest(url, bytes.NewBuffer(body), log)
	if err == nil {
		log.Infof("run workflow %s result %s", args.PipelineName, result)
	}
	return err
}

func (c *Client) RunWorkflowTask(args *service.WorkflowTaskArgs, log *xlog.Logger) error {
	url := fmt.Sprintf("%s/workflow/workflowtask", c.ApiBase)
	log.Info("start run scheduled task..")
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}
	result, err := c.sendPostRequest(url, bytes.NewBuffer(body), log)
	if err == nil {
		log.Infof("run workflow %s result %s", args.WorkflowName, result)
	}
	return err
}

func (c *Client) RunTestTask(args *service.TestTaskArgs, log *xlog.Logger) error {
	url := fmt.Sprintf("%s/testing/testtask", c.ApiBase)
	log.Info("start run test scheduled task..")
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}
	result, err := c.sendPostRequest(url, bytes.NewBuffer(body), log)
	if err == nil {
		log.Infof("run test %s result %s", args.TestName, result)
	}
	return err
}

func (c *Client) TriggerCleanCache(log *xlog.Logger) error {
	//c.TriggerCleanWorkflowCache(log)
	c.TriggerSystemGc(log)

	return nil
}

// 该定时任务会查询系统中的工作流，去s3查询相应的目录。所有未匹配到工作流的目录都会被清理掉。
// 即其他业务场景（如helm）添加的缓存目录，也会被清理掉。所以暂时移除此定时任务。
// 如有需要请手动调用backend接口进行清理。但清理时也需要谨慎，充分分析可能产生的影响。
//func (c *Client) TriggerCleanWorkflowCache(log *xlog.Logger) error {
//	url := fmt.Sprintf("%s/system/capacity/clean", c.ApiBase)
//	log.Info("Start to clean workflow cache..")
//
//	result, err := c.sendPostRequest(url, nil, log)
//	if err != nil {
//		log.Errorf("trigger clean workflow cache error :%v", err)
//		return err
//	}
//
//	log.Infof("trigger clean workflow cache: %v", result)
//	return nil
//}

// DryRunFlag indicates whether a run is a dry run or not.
// If it is a dry run, the relevant API is supposed to be no-op except logging.
type DryRunFlag struct {
	DryRun bool `json:"dryrun"`
}

func (c *Client) TriggerSystemGc(log *xlog.Logger) error {
	url := fmt.Sprintf("%s/system/capacity/gc", c.ApiBase)
	log.Info("Start system capacity garbage collection jobs..")

	args := &DryRunFlag{
		DryRun: false,
	}
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}
	result, err := c.sendPostRequest(url, bytes.NewBuffer(body), log)
	if err != nil {
		log.Errorf("trigger system GC jobs error :%v", err)
	} else {
		log.Infof("trigger system GC jobs: %v", result)
	}
	return err
}

func (c *Client) sendRequest(url string) error {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", fmt.Sprintf("%s %s", setting.TIMERAPIKEY, c.Token))
	resp, err := c.Conn.Do(request)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
	}
	return err
}

func (c *Client) sendPostRequest(url string, body io.Reader, log *xlog.Logger) (string, error) {
	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		log.Errorf("create post request error : %v", err)
		return "", err
	}
	request.Header.Set("Authorization", fmt.Sprintf("%s %s", setting.TIMERAPIKEY, c.Token))
	var resp *http.Response
	resp, err = c.Conn.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (c *Client) CreateWebHookUser(webHookUser *service.DomainWebHookUser, log *xlog.Logger) error {
	url := "http://api.koderover.com/api/operation/webHook/user"
	args := &service.DomainWebHookUser{
		Domain:    webHookUser.Domain,
		UserCount: webHookUser.UserCount,
		CreatedAt: time.Now().Unix(),
	}
	body, err := json.Marshal(args)
	if err != nil {
		log.Errorf("marshal json args error: %v", err)
		return err
	}

	rsaStr := base64.StdEncoding.EncodeToString(rsa.RSA_Encrypt(body))
	operationReq := &service.OperationRequest{
		Data: rsaStr,
	}
	operationReqBody, err := json.Marshal(operationReq)
	if err != nil {
		log.Errorf("operation marshal json args error: %v", err)
		return err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(operationReqBody))
	if err != nil {
		log.Errorf("create post request error : %v", err)
		return err
	}
	var resp *http.Response
	resp, err = c.Conn.Do(request)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if result, err := ioutil.ReadAll(resp.Body); err != nil {
			log.Infof("createWebHookUser result:%s", result)
		}
	}
	return err
}
