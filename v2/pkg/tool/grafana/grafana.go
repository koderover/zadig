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

package grafana

type ListAlertInstanceResp struct {
	Annotations  Annotations `json:"annotations"`
	Fingerprint  string      `json:"fingerprint"`
	Receivers    []Receivers `json:"receivers"`
	Status       Status      `json:"status"`
	GeneratorURL string      `json:"generatorURL"`
	Labels       Labels      `json:"labels"`
}
type Annotations struct {
	OrgID       string `json:"__orgId__"`
	ValueString string `json:"__value_string__"`
	Values      string `json:"__values__"`
}
type Receivers struct {
	Active       interface{} `json:"active"`
	Integrations interface{} `json:"integrations"`
	Name         string      `json:"name"`
}
type Status struct {
	InhibitedBy []interface{} `json:"inhibitedBy"`
	SilencedBy  []interface{} `json:"silencedBy"`
	State       string        `json:"state"`
}
type Labels struct {
	AlertRuleUID  string `json:"__alert_rule_uid__"`
	Alertname     string `json:"alertname"`
	GrafanaFolder string `json:"grafana_folder"`
	Handler       string `json:"handler"`
	Instance      string `json:"instance"`
	Job           string `json:"job"`
	Method        string `json:"method"`
	Status        string `json:"status"`
}

type ListAlertResp struct {
	ID           int    `json:"id"`
	UID          string `json:"uid"`
	OrgID        int    `json:"orgID"`
	FolderUID    string `json:"folderUID"`
	RuleGroup    string `json:"ruleGroup"`
	Title        string `json:"title"`
	Condition    string `json:"condition"`
	Data         []Data `json:"data"`
	NoDataState  string `json:"noDataState"`
	ExecErrState string `json:"execErrState"`
	For          string `json:"for"`
	IsPaused     bool   `json:"isPaused"`
}
type RelativeTimeRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}
type Model struct {
	DisableTextWrap     bool   `json:"disableTextWrap"`
	EditorMode          string `json:"editorMode"`
	Expr                string `json:"expr"`
	FullMetaSearch      bool   `json:"fullMetaSearch"`
	IncludeNullMetadata bool   `json:"includeNullMetadata"`
	Instant             bool   `json:"instant"`
	IntervalMs          int    `json:"intervalMs"`
	LegendFormat        string `json:"legendFormat"`
	MaxDataPoints       int    `json:"maxDataPoints"`
	Range               bool   `json:"range"`
	RefID               string `json:"refId"`
	UseBackend          bool   `json:"useBackend"`
}
type Evaluator struct {
	Params []int  `json:"params"`
	Type   string `json:"type"`
}
type Operator struct {
	Type string `json:"type"`
}
type Query struct {
	Params []string `json:"params"`
}
type Reducer struct {
	Params []interface{} `json:"params"`
	Type   string        `json:"type"`
}
type Conditions struct {
	Evaluator Evaluator `json:"evaluator"`
	Operator  Operator  `json:"operator"`
	Query     Query     `json:"query"`
	Reducer   Reducer   `json:"reducer"`
	Type      string    `json:"type"`
}
type Datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

type Data struct {
	RefID             string            `json:"refId"`
	QueryType         string            `json:"queryType"`
	RelativeTimeRange RelativeTimeRange `json:"relativeTimeRange"`
	DatasourceUID     string            `json:"datasourceUid"`
}

func (c *Client) ListAlertInstance() (resp []*ListAlertInstanceResp, err error) {
	_, err = c.R().SetSuccessResult(&resp).Get("/api/alertmanager/grafana/api/v2/alerts")
	return
}

func (c *Client) ListAlert() (resp []*ListAlertResp, err error) {
	_, err = c.R().SetSuccessResult(&resp).Get("/api/v1/provisioning/alert-rules")
	return
}
