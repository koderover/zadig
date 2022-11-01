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

package sonar

import (
	"encoding/base64"
	"fmt"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Client struct {
	*httpclient.Client

	host  string
	token string
}

func NewSonarClient(host, token string) *Client {
	encodeToken := base64.StdEncoding.EncodeToString([]byte(token + ":"))
	c := httpclient.New(
		httpclient.SetAuthToken(encodeToken),
		httpclient.SetAuthScheme("Basic"),
		httpclient.SetHostURL(host),
	)

	return &Client{
		Client: c,
		host:   host,
		token:  token,
	}
}

type CETaskStatus string

const (
	CETaskPending    CETaskStatus = "PENDING"
	CETaskInProgress CETaskStatus = "IN_PROGRESS"
	CETaskSuccess    CETaskStatus = "SUCCESS"
	CETaskFailed     CETaskStatus = "FAILED"
	CETaskCanceled   CETaskStatus = "CANCELED"
)

type CETaskInfo struct {
	Task CETask `json:"task"`
}

type CETask struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	ComponentID    string       `json:"componentId"`
	ComponentKey   string       `json:"componentKey"`
	AnalysisID     string       `json:"analysisId"`
	Status         CETaskStatus `json:"status"`
	SubmitterLogin string       `json:"submitterLogin"`
	WarningCount   int          `json:"warningCount"`
}

func (c *Client) GetCETaskInfo(taskID string) (*CETaskInfo, error) {
	url := "/api/ce/task"
	res := &CETaskInfo{}
	if _, err := c.Client.Get(url, httpclient.SetQueryParam("id", taskID), httpclient.SetResult(res)); err != nil {
		return nil, fmt.Errorf("get sonar compute engine task: %s info error: %v", taskID, err)
	}
	return res, nil
}

type QualityGateStatus string

const (
	QualityGateError QualityGateStatus = "ERROR"
	QualityGateOK    QualityGateStatus = "OK"
	QualityGateWarn  QualityGateStatus = "WARN"
	QualityGateNone  QualityGateStatus = "None"
)

type ProjectInfo struct {
	ProjectStatus ProjectStatus
}

type ProjectStatus struct {
	Status     QualityGateStatus `json:"status"`
	Conditions []Condition       `json:"conditions"`
}

type Condition struct {
	Status         QualityGateStatus `json:"status"`
	MetricKey      string            `json:"metricKey"`
	Comparator     string            `json:"comparator"`
	PeriodIndex    int               `json:"periodIndex"`
	ErrorThreshold string            `json:"errorThreshold"`
	ActualValue    string            `json:"actualValue"`
}

func (c *Client) GetQualityGateInfo(analysisID string) (*ProjectInfo, error) {
	url := "/api/qualitygates/project_status"
	res := &ProjectInfo{}
	if _, err := c.Client.Get(url, httpclient.SetQueryParam("analysisId", analysisID), httpclient.SetResult(res)); err != nil {
		return nil, fmt.Errorf("get sonar quality gate: %s info error: %v", analysisID, err)
	}
	return res, nil
}
