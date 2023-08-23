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

package guanceyun

import (
	"strconv"

	"github.com/pkg/errors"
)

const MonitorPageSize = 100

type MonitorResponse struct {
	Code      int              `json:"code"`
	Content   []MonitorContent `json:"content"`
	ErrorCode string           `json:"errorCode"`
	Message   string           `json:"message"`
	PageInfo  PageInfo         `json:"pageInfo"`
	Success   bool             `json:"success"`
	TraceID   string           `json:"traceId"`
}
type CrontabInfo struct {
	Crontab string `json:"crontab"`
	ID      string `json:"id"`
}
type Filters struct {
	ID    string `json:"id"`
	Logic string `json:"logic"`
	Name  string `json:"name"`
	Op    string `json:"op"`
	Type  string `json:"type"`
	Value string `json:"value"`
}
type Query struct {
	Alias       string        `json:"alias"`
	Code        string        `json:"code"`
	DataSource  string        `json:"dataSource"`
	Field       string        `json:"field"`
	FieldFunc   string        `json:"fieldFunc"`
	FieldType   string        `json:"fieldType"`
	Filters     []Filters     `json:"filters"`
	FuncList    []interface{} `json:"funcList"`
	GroupBy     []interface{} `json:"groupBy"`
	GroupByTime string        `json:"groupByTime"`
	Namespace   string        `json:"namespace"`
	Q           string        `json:"q"`
	Type        string        `json:"type"`
}
type Querylist struct {
	Datasource string `json:"datasource"`
	Qtype      string `json:"qtype"`
	Query      Query  `json:"query"`
	UUID       string `json:"uuid"`
}
type Conditions struct {
	Alias    string   `json:"alias"`
	Operands []string `json:"operands"`
	Operator string   `json:"operator"`
}
type Rules struct {
	ConditionLogic string       `json:"conditionLogic"`
	Conditions     []Conditions `json:"conditions"`
	Status         string       `json:"status"`
}
type Extend struct {
	FuncName  string      `json:"funcName"`
	Querylist []Querylist `json:"querylist"`
	Rules     []Rules     `json:"rules"`
}

type JSONScript struct {
	AtAccounts             []string      `json:"atAccounts"`
	AtNoDataAccounts       []interface{} `json:"atNoDataAccounts"`
	Channels               []interface{} `json:"channels"`
	CheckerOpt             CheckerOpt    `json:"checkerOpt"`
	Every                  string        `json:"every"`
	GroupBy                []interface{} `json:"groupBy"`
	Interval               int           `json:"interval"`
	Message                string        `json:"message"`
	Name                   string        `json:"name"`
	NoDataMessage          string        `json:"noDataMessage"`
	NoDataTitle            string        `json:"noDataTitle"`
	RecoverNeedPeriodCount int           `json:"recoverNeedPeriodCount"`
	Title                  string        `json:"title"`
	Type                   string        `json:"type"`
}
type UpdatorInfo struct {
	AcntWsNickname string `json:"acntWsNickname"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	UserIconURL    string `json:"userIconUrl"`
}
type MonitorContent struct {
	CreateAt      int         `json:"createAt"`
	CreatedWay    string      `json:"createdWay"`
	Creator       string      `json:"creator"`
	CrontabInfo   CrontabInfo `json:"crontabInfo"`
	DeleteAt      int         `json:"deleteAt"`
	Extend        Extend      `json:"extend"`
	ID            int         `json:"id"`
	IsSLI         bool        `json:"isSLI"`
	JSONScript    JSONScript  `json:"jsonScript"`
	MonitorName   string      `json:"monitorName"`
	MonitorUUID   string      `json:"monitorUUID"`
	RefKey        string      `json:"refKey"`
	Status        int         `json:"status"`
	Type          string      `json:"type"`
	UpdateAt      int         `json:"updateAt"`
	Updator       string      `json:"updator"`
	UpdatorInfo   UpdatorInfo `json:"updatorInfo"`
	UUID          string      `json:"uuid"`
	WorkspaceUUID string      `json:"workspaceUUID"`
}
type PageInfo struct {
	Count      int `json:"count"`
	PageIndex  int `json:"pageIndex"`
	PageSize   int `json:"pageSize"`
	TotalCount int `json:"totalCount"`
}

func (c *Client) ListAllMonitor(search string) (resp []MonitorContent, err error) {
	size := MonitorPageSize
	for index := 1; ; index++ {
		result, total, err := c.ListMonitor(search, size, index)
		if err != nil {
			return nil, errors.Wrapf(err, "list monitor %d index failed", index)
		}
		resp = append(resp, result...)
		if index*size >= total {
			break
		}
	}
	return resp, nil
}

func (c *Client) ListMonitor(search string, pageSize, pageIndex int) ([]MonitorContent, int, error) {
	resp := new(MonitorResponse)
	params := map[string]string{
		"pageSize":  strconv.Itoa(pageSize),
		"pageIndex": strconv.Itoa(pageIndex),
	}
	if search != "" {
		params["search"] = search
	}
	_, err := c.R().SetQueryParams(params).SetSuccessResult(resp).
		Get(c.BaseURL + "/api/v1/monitor/check/list")
	if err != nil {
		return nil, 0, err
	}
	return resp.Content, resp.PageInfo.TotalCount, nil
}
