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
	"context"
	"net/url"
	"strconv"
)

// IssueService ...
type IssueService struct {
	client *Client
}

// GetByKeyOrID https://developer.atlassian.com/cloud/jira/platform/rest/#api-api-2-issue-issueIdOrKey-get
func (s *IssueService) GetByKeyOrID(keyOrID, fields string) (*Issue, error) {
	if fields == "" {
		fields = NormalIssueFields.String()
	}

	params := url.Values{}
	params.Add("fields", fields)

	url := s.client.Host + "/rest/api/2/issue/" + keyOrID + "?" + params.Encode()

	var issue *Issue
	err := s.client.Conn.CallWithJson(context.Background(), &issue, "GET", url, "")
	return issue, err
}

// GetIssuesCountByJQL ...
func (s *IssueService) GetIssuesCountByJQL(jql string) (int, error) {
	if jql == "" {
		return 0, nil
	}
	fields := "*none"
	params := url.Values{}
	params.Add("jql", string(jql))
	params.Add("maxResults", "0")
	params.Add("fields", fields)
	params.Add("startAt", "0")
	// params.Add("expand", "changelog")
	url := s.client.Host + "/rest/api/2/search?" + params.Encode()
	//log.Info("ScheduleCall jira api:", url)
	issues := &IssueResult{}
	err := s.client.Conn.CallWithJson(context.Background(), &issues, "GET", url, "")
	return int(issues.Total), err
}

// GetIssuesByJQL ...
func (s *IssueService) GetIssuesByJQL(jql, fields string) ([]*Issue, error) {
	resp := make([]*Issue, 0)
	if jql == "" {
		return resp, nil
	}
	if fields == "" {
		fields = NormalIssueFields.String()
	}
	params := url.Values{}
	params.Add("jql", string(jql))
	params.Add("maxResults", "50")
	params.Add("fields", fields)
	params.Add("startAt", "0")
	// params.Add("expand", "changelog")
	startAt := 0
	//log.Info("ScheduleCall jira api:", url)
	for {
		issues := &IssueResult{}
		params.Set("startAt", strconv.Itoa(startAt))
		url := s.client.Host + "/rest/api/2/search?" + params.Encode()
		err := s.client.Conn.CallWithJson(context.Background(), &issues, "GET", url, "")
		if err != nil {
			return resp, err
		}
		resp = append(resp, issues.Issues...)
		startAt += issues.MaxResults
		if startAt >= issues.Total {
			break
		}
	}
	return resp, nil
}

//GetIncidentFollowIssues 用来返回所有事故跟进项和未完成的事故跟进项
func (s *IssueService) GetIncidentFollowIssues(jql string) (allFollowIssues, unfinishedFollowIssues []string, err error) {
	issues, err := s.GetIssuesByJQL(jql, "")
	if err != nil {
		return allFollowIssues, unfinishedFollowIssues, err
	}
	repetition := make(map[string]bool)
	for _, issue := range issues {
		for _, link := range issue.Fields.IssueLinks {
			// InwardIssue
			if link.InwardIssue != nil {
				_, ok := repetition[link.InwardIssue.Key]
				if !ok {
					repetition[link.InwardIssue.Key] = true
				} else {
					continue
				}
				if link.InwardIssue.Fields.IssueType.Name != "事故" && link.InwardIssue.Fields.IssueType.Name != "发布" && link.InwardIssue.Fields.IssueType.Name != "Deploy" {
					allFollowIssues = append(allFollowIssues, link.InwardIssue.Key)
					if link.InwardIssue.Fields.Status.Name != "完成" && link.InwardIssue.Fields.Status.Name != "已处理" && link.InwardIssue.Fields.Status.Name != "DONE" &&
						link.InwardIssue.Fields.Status.Name != "无需处理" && link.InwardIssue.Fields.Status.Name != "已关闭" && link.InwardIssue.Fields.Status.Name != "关闭" &&
						link.InwardIssue.Fields.Status.Name != "WON'T FIX" && link.InwardIssue.Fields.Status.Name != "Done" {
						unfinishedFollowIssues = append(unfinishedFollowIssues, link.InwardIssue.Key)
					}
				}
			}
			// OutwardIssue
			if link.OutwardIssue != nil {
				_, ok := repetition[link.OutwardIssue.Key]
				if !ok {
					repetition[link.OutwardIssue.Key] = true
				} else {
					continue
				}
				if link.OutwardIssue.Fields.IssueType.Name != "事故" && link.OutwardIssue.Fields.IssueType.Name != "发布" && link.OutwardIssue.Fields.IssueType.Name != "Deploy" {
					allFollowIssues = append(allFollowIssues, link.OutwardIssue.Key)
					if link.OutwardIssue.Fields.Status.Name != "完成" && link.OutwardIssue.Fields.Status.Name != "已处理" && link.OutwardIssue.Fields.Status.Name != "DONE" &&
						link.OutwardIssue.Fields.Status.Name != "无需处理" && link.OutwardIssue.Fields.Status.Name != "已关闭" && link.OutwardIssue.Fields.Status.Name != "关闭" &&
						link.OutwardIssue.Fields.Status.Name != "WON'T FIX" && link.OutwardIssue.Fields.Status.Name != "Done" {
						unfinishedFollowIssues = append(unfinishedFollowIssues, link.OutwardIssue.Key)
					}
				}
			}
		}
	}

	return
}
