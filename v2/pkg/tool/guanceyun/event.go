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
	"encoding/json"

	"github.com/pkg/errors"
)

type Level string

var (
	Critical Level = "critical"
	Error    Level = "error"
	Warning  Level = "warning"

	LevelMap = map[Level]int{
		Critical: 4,
		Error:    3,
		Warning:  2,
	}
)

type EventResponse struct {
	Code      int           `json:"code"`
	Content   *EventContent `json:"content"`
	ErrorCode string        `json:"errorCode"`
	Message   string        `json:"message"`
	Success   bool          `json:"success"`
	TraceID   string        `json:"traceId"`
}
type Data struct {
	Docid                    string        `json:"__docid"`
	AlertTimeRanges          []interface{} `json:"alert_time_ranges"`
	Date                     int64         `json:"date"`
	DfBotObsDetail           interface{}   `json:"df_bot_obs_detail"`
	DfDimensionTags          string        `json:"df_dimension_tags"`
	DfEventID                string        `json:"df_event_id"`
	DfMessage                string        `json:"df_message"`
	DfMeta                   string        `json:"df_meta"`
	DfMonitorChecker         string        `json:"df_monitor_checker"`
	DfMonitorCheckerEventRef string        `json:"df_monitor_checker_event_ref"`
	DfMonitorCheckerName     string        `json:"df_monitor_checker_name"`
	DfMonitorType            string        `json:"df_monitor_type"`
	DfSource                 string        `json:"df_source"`
	DfStatus                 string        `json:"df_status"`
	DfTitle                  string        `json:"df_title"`
}

type MonitorMeta struct {
	AlertInfo     AlertInfo      `json:"alert_info"`
	CheckTargets  []CheckTargets `json:"check_targets"`
	CheckerOpt    CheckerOpt     `json:"checker_opt"`
	DimensionTags DimensionTags  `json:"dimension_tags"`
	ExtraData     ExtraData      `json:"extra_data"`
	MonitorOpt    MonitorOpt     `json:"monitor_opt"`
}
type Targets struct {
	HasSecret    bool          `json:"hasSecret"`
	IgnoreReason string        `json:"ignoreReason"`
	IsIgnored    bool          `json:"isIgnored"`
	MinInterval  int           `json:"minInterval"`
	Status       []interface{} `json:"status"`
	SubStatus    []string      `json:"subStatus"`
	To           []string      `json:"to,omitempty"`
	Type         string        `json:"type"`
	BodyType     string        `json:"bodyType,omitempty"`
	Name         string        `json:"name,omitempty"`
	URL          string        `json:"url,omitempty"`
}
type AlertInfo struct {
	MatchedSilentRule interface{} `json:"matchedSilentRule"`
	Targets           []Targets   `json:"targets"`
}
type CheckTargets struct {
	Alias     string `json:"alias"`
	Dql       string `json:"dql"`
	QueryType string `json:"queryType"`
	Range     int    `json:"range"`
}

type CheckerOpt struct {
	ID                   string      `json:"id"`
	InfoEvent            bool        `json:"infoEvent"`
	Interval             int         `json:"interval"`
	Message              interface{} `json:"message"`
	Name                 string      `json:"name"`
	NoDataAction         string      `json:"noDataAction"`
	NoDataInterval       int         `json:"noDataInterval"`
	NoDataMessage        interface{} `json:"noDataMessage"`
	NoDataRecoverMessage interface{} `json:"noDataRecoverMessage"`
	NoDataRecoverTitle   interface{} `json:"noDataRecoverTitle"`
	NoDataScale          int         `json:"noDataScale"`
	NoDataTitle          interface{} `json:"noDataTitle"`
	RecoverInterval      int         `json:"recoverInterval"`
	RecoverMessage       interface{} `json:"recoverMessage"`
	RecoverScale         int         `json:"recoverScale"`
	RecoverTitle         string      `json:"recoverTitle"`
	Rules                []Rules     `json:"rules"`
	Title                string      `json:"title"`
}
type DimensionTags struct {
}
type ExtraData struct {
	Type string `json:"type"`
}
type MonitorOpt struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type EventContent struct {
	Data       []Data `json:"data"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	TotalCount int    `json:"total_count"`
}

type SearchEventBody struct {
	TimeRange []int64        `json:"timeRange"`
	Filters   []EventFilters `json:"filters"`
	Limit     int            `json:"limit"`
}

type EventFilters struct {
	Name      string   `json:"name"`
	Condition string   `json:"condition"`
	Operation string   `json:"operation"`
	Value     []string `json:"value"`
}

type SearchEventByMonitorArg struct {
	CheckerName string
	CheckerID   string
}

type EventResp struct {
	CheckerName string
	CheckerID   string
	EventLevel  Level
}

// SearchEventByChecker startTime and endTime are Millisecond timestamps
func (c *Client) SearchEventByChecker(args []*SearchEventByMonitorArg, startTime, endTime int64) ([]*EventResp, error) {
	body := SearchEventBody{
		TimeRange: []int64{startTime, endTime},
		Limit:     100,
	}
	for _, arg := range args {
		body.Filters = append(body.Filters, EventFilters{
			Name:      "df_title",
			Condition: "or",
			Operation: "=",
			Value:     []string{arg.CheckerName},
		})
	}
	resp := new(EventResponse)
	_, err := c.R().SetBodyJsonMarshal(body).SetSuccessResult(resp).
		Post(c.BaseURL + "/api/v1/events/abnormal/list")
	if err != nil {
		return nil, err
	}

	eventRespMap := make(map[string]*EventResp, 0)
	for _, data := range resp.Content.Data {
		meta := new(MonitorMeta)
		err = json.Unmarshal([]byte(data.DfMeta), meta)
		if err != nil {
			return nil, errors.Wrapf(err, "SearchEventByChecker unmarshal %s meta failed", data.DfTitle)
		}
		if e, ok := eventRespMap[meta.CheckerOpt.ID]; ok {
			if LevelMap[e.EventLevel] < LevelMap[Level(data.DfStatus)] {
				e.EventLevel = Level(data.DfStatus)
			}
		} else {
			eventRespMap[meta.CheckerOpt.ID] = &EventResp{
				CheckerName: data.DfTitle,
				CheckerID:   meta.CheckerOpt.ID,
				EventLevel:  Level(data.DfStatus),
			}
		}
	}

	result := make([]*EventResp, 0)
	for id, eventResp := range eventRespMap {
		for _, arg := range args {
			if arg.CheckerID == id {
				result = append(result, eventResp)
			}
		}
	}
	return result, nil
}
