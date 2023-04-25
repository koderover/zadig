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

package jira

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
)

const commentTmpl = `{"body": {
  "version": 1,
  "type": "doc",
  "content": [
    {
      "type": "paragraph",
      "content": [
        {
          "type": "text",
          "text": "%s"
        },
        {
          "type": "text",
          "text": "%s",
          "marks": [
            {
              "type": "link",
              "attrs": {
                "href": "%s"
              }
            }
          ]
        }
      ]
    }
  ]
}}`

// IssueService ...
type IssueService struct {
	client *Client
}

// GetByKeyOrID https://developer.atlassian.com/cloud/jira/platform/rest/#api-api-2-issue-issueIdOrKey-get
func (s *IssueService) GetByKeyOrID(keyOrID, fields string) (*Issue, error) {
	if fields == "" {
		fields = NormalIssueFields.String()
	}

	url := s.client.Host + "/rest/api/2/issue/" + keyOrID

	issue := &Issue{}
	resp, err := s.client.R().AddQueryParam("fields", fields).Get(url)
	if err != nil {
		return nil, err
	}
	if err = resp.UnmarshalJson(issue); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	return issue, nil
}

type IssueTypeDefinition struct {
	Self     string `json:"self"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subtask  bool   `json:"subtask"`
	Statuses []struct {
		Self             string `json:"self"`
		Description      string `json:"description"`
		IconURL          string `json:"iconUrl"`
		Name             string `json:"name"`
		UntranslatedName string `json:"untranslatedName"`
		ID               string `json:"id"`
		StatusCategory   struct {
			Self      string `json:"self"`
			ID        int    `json:"id"`
			Key       string `json:"key"`
			ColorName string `json:"colorName"`
			Name      string `json:"name"`
		} `json:"statusCategory"`
	} `json:"statuses"`
}

type IssueTypeWithStatus struct {
	Type   string   `json:"type"`
	Status []string `json:"status"`
}

func (s *IssueService) GetTypes(project string) ([]*IssueTypeWithStatus, error) {
	url := s.client.Host + "/rest/api/2/project/" + project + "/statuses"
	resp, err := s.client.R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.GetStatusCode()/100 != 2 {
		return nil, errors.Errorf("unexpected status code %d", resp.GetStatusCode())
	}
	var list []*IssueTypeDefinition
	if err = resp.UnmarshalJson(&list); err != nil {
		return nil, err
	}
	var result []*IssueTypeWithStatus
	for _, v := range list {
		result = append(result, &IssueTypeWithStatus{
			Type: v.Name,
			Status: func() []string {
				var re []string
				for _, s := range v.Statuses {
					re = append(re, s.Name)
				}
				return re
			}(),
		})
	}
	return result, nil
}

func (s *IssueService) SearchByJQL(jql string, findAll bool) ([]*Issue, error) {
	var re []*Issue
	start := 0
	for {
		list, next, err := s.searchByJQL(jql, start)
		if err != nil {
			return nil, err
		}
		re = append(re, list...)
		if next == 0 || !findAll {
			return re, nil
		}
		start = next
	}
	return re, nil
}

func (s *IssueService) searchByJQL(jql string, start int) ([]*Issue, int, error) {
	url := s.client.Host + "/rest/api/2/search"
	resp, err := s.client.R().SetQueryParams(map[string]string{
		"jql":     jql,
		"startAt": strconv.Itoa(start),
	}).Get(url)
	if err != nil {
		return nil, 0, err
	}
	if resp.GetStatusCode()/100 != 2 {
		return nil, 0, errors.Errorf("unexpected status code %d", resp.GetStatusCode())
	}
	type tmp struct {
		StartAt    int      `json:"startAt"`
		MaxResults int      `json:"maxResults"`
		Total      int      `json:"total"`
		Issues     []*Issue `json:"issues"`
	}
	var re tmp
	if err = resp.UnmarshalJson(&re); err != nil {
		return nil, 0, errors.Wrap(err, "unmarshal")
	}
	next := 0
	if re.StartAt+re.MaxResults < re.Total {
		next = re.StartAt + re.MaxResults
	}
	return re.Issues, next, nil
}

type updateBody struct {
	Transition struct {
		ID string `json:"id"`
	} `json:"transition"`
}

func (s *IssueService) UpdateStatus(key, statusID string) error {
	url := s.client.Host + "/rest/api/2/issue/" + key + "/transitions"
	body := updateBody{Transition: struct {
		ID string `json:"id"`
	}{ID: statusID}}
	resp, err := s.client.R().SetBodyJsonMarshal(body).Post(url)
	if err != nil {
		return err
	}
	if resp.GetStatusCode() != http.StatusOK && resp.GetStatusCode() != http.StatusNoContent {
		return errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	return nil
}

type Transition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	To          To     `json:"to"`
	IsAvailable *bool  `json:"isAvailable"`
}

type To struct {
	Self           string         `json:"self"`
	Description    string         `json:"description"`
	IconURL        string         `json:"iconUrl"`
	Name           string         `json:"name"`
	ID             string         `json:"id"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

func (s *IssueService) GetTransitions(key string) ([]*Transition, error) {
	url := s.client.Host + "/rest/api/2/issue/" + key + "/transitions?expand=transitions.fields"
	resp, err := s.client.R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.GetStatusCode()/100 != 2 {
		return nil, errors.Errorf("unexpected status code %d", resp.GetStatusCode())
	}
	type tmp struct {
		Transitions []*Transition `json:"transitions"`
	}
	t := &tmp{}
	if err := resp.UnmarshalJson(t); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}
	var list []*Transition
	for _, transition := range t.Transitions {
		if transition.IsAvailable != nil && !*transition.IsAvailable {
			continue
		}
		list = append(list, transition)
	}
	return list, nil
}

func (s *IssueService) AddCommentV3(key, comment, link, linkTitle string) error {
	url := s.client.Host + "/rest/api/3/issue/" + key + "/comment"

	resp, err := s.client.R().SetContentType("application/json").SetBody(fmt.Sprintf(commentTmpl, comment, linkTitle, link)).Post(url)
	if err != nil {
		return err
	}
	if resp.GetStatusCode()/100 != 2 {
		return errors.Errorf("get unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	return nil
}

func (s *IssueService) AddCommentV2(key, comment string) error {
	url := s.client.Host + "/rest/api/2/issue/" + key + "/comment"

	resp, err := s.client.R().SetBodyJsonMarshal(map[string]string{
		"body": comment,
	}).Post(url)
	if err != nil {
		return err
	}
	if resp.GetStatusCode()/100 != 2 {
		return errors.Errorf("get unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	return nil
}

//// GetIssuesCountByJQL ...
//func (s *IssueService) GetIssuesCountByJQL(jql string) (int, error) {
//	if jql == "" {
//		return 0, nil
//	}
//	fields := "*none"
//	params := url.Values{}
//	params.Add("jql", string(jql))
//	params.Add("maxResults", "0")
//	params.Add("fields", fields)
//	params.Add("startAt", "0")
//	// params.Add("expand", "changelog")
//	url := s.client.Host + "/rest/api/2/search?" + params.Encode()
//	//log.Info("ScheduleCall jira api:", url)
//	issues := &IssueResult{}
//	err := s.client.Conn.CallWithJson(context.Background(), &issues, "GET", url, "")
//	return int(issues.Total), err
//}

//// GetIssuesByJQL ...
//func (s *IssueService) GetIssuesByJQL(jql, fields string) ([]*Issue, error) {
//	resp := make([]*Issue, 0)
//	if jql == "" {
//		return resp, nil
//	}
//	if fields == "" {
//		fields = NormalIssueFields.String()
//	}
//	params := url.Values{}
//	params.Add("jql", string(jql))
//	params.Add("maxResults", "50")
//	params.Add("fields", fields)
//	params.Add("startAt", "0")
//	// params.Add("expand", "changelog")
//	startAt := 0
//	//log.Info("ScheduleCall jira api:", url)
//	for {
//		issues := &IssueResult{}
//		params.Set("startAt", strconv.Itoa(startAt))
//		url := s.client.Host + "/rest/api/2/search?" + params.Encode()
//		err := s.client.Conn.CallWithJson(context.Background(), &issues, "GET", url, "")
//		if err != nil {
//			return resp, err
//		}
//		resp = append(resp, issues.Issues...)
//		startAt += issues.MaxResults
//		if startAt >= issues.Total {
//			break
//		}
//	}
//	return resp, nil
//}

////GetIncidentFollowIssues 用来返回所有事故跟进项和未完成的事故跟进项
//func (s *IssueService) GetIncidentFollowIssues(jql string) (allFollowIssues, unfinishedFollowIssues []string, err error) {
//	issues, err := s.GetIssuesByJQL(jql, "")
//	if err != nil {
//		return allFollowIssues, unfinishedFollowIssues, err
//	}
//	repetition := make(map[string]bool)
//	for _, issue := range issues {
//		for _, link := range issue.Fields.IssueLinks {
//			// InwardIssue
//			if link.InwardIssue != nil {
//				_, ok := repetition[link.InwardIssue.Key]
//				if !ok {
//					repetition[link.InwardIssue.Key] = true
//				} else {
//					continue
//				}
//				if link.InwardIssue.Fields.IssueType.Name != "事故" && link.InwardIssue.Fields.IssueType.Name != "发布" && link.InwardIssue.Fields.IssueType.Name != "Deploy" {
//					allFollowIssues = append(allFollowIssues, link.InwardIssue.Key)
//					if link.InwardIssue.Fields.Status.Name != "完成" && link.InwardIssue.Fields.Status.Name != "已处理" && link.InwardIssue.Fields.Status.Name != "DONE" &&
//						link.InwardIssue.Fields.Status.Name != "无需处理" && link.InwardIssue.Fields.Status.Name != "已关闭" && link.InwardIssue.Fields.Status.Name != "关闭" &&
//						link.InwardIssue.Fields.Status.Name != "WON'T FIX" && link.InwardIssue.Fields.Status.Name != "Done" {
//						unfinishedFollowIssues = append(unfinishedFollowIssues, link.InwardIssue.Key)
//					}
//				}
//			}
//			// OutwardIssue
//			if link.OutwardIssue != nil {
//				_, ok := repetition[link.OutwardIssue.Key]
//				if !ok {
//					repetition[link.OutwardIssue.Key] = true
//				} else {
//					continue
//				}
//				if link.OutwardIssue.Fields.IssueType.Name != "事故" && link.OutwardIssue.Fields.IssueType.Name != "发布" && link.OutwardIssue.Fields.IssueType.Name != "Deploy" {
//					allFollowIssues = append(allFollowIssues, link.OutwardIssue.Key)
//					if link.OutwardIssue.Fields.Status.Name != "完成" && link.OutwardIssue.Fields.Status.Name != "已处理" && link.OutwardIssue.Fields.Status.Name != "DONE" &&
//						link.OutwardIssue.Fields.Status.Name != "无需处理" && link.OutwardIssue.Fields.Status.Name != "已关闭" && link.OutwardIssue.Fields.Status.Name != "关闭" &&
//						link.OutwardIssue.Fields.Status.Name != "WON'T FIX" && link.OutwardIssue.Fields.Status.Name != "Done" {
//						unfinishedFollowIssues = append(unfinishedFollowIssues, link.OutwardIssue.Key)
//					}
//				}
//			}
//		}
//	}
//
//	return
//}
