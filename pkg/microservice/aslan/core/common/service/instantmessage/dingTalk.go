/*
Copyright 2022 The KodeRover Authors.

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

package instantmessage

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type DingDingMessage struct {
	MsgType    string              `json:"msgtype"`
	MarkDown   *DingDingMarkDown   `json:"markdown"`
	ActionCard *DingDingActionCard `json:"actionCard"`
	At         *DingDingAt         `json:"at"`
}

type DingDingMarkDown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// DingDingActionCard API ref: https://open.dingtalk.com/document/robots/custom-robot-access
type DingDingActionCard struct {
	HideAvatar        string            `json:"hideAvatar,omitempty"`     // 0: show, 1: hide
	ButtonOrientation string            `json:"btnOrientation,omitempty"` // 0: vertical, 1: horizontal
	SingleURL         string            `json:"singleURL,omitempty"`
	SingleTitle       string            `json:"singleTitle,omitempty"`
	Text              string            `json:"text,omitempty"`
	Title             string            `json:"title,omitempty"`
	Buttons           []*DingDingButton `json:"btns,omitempty"`
}

type DingDingButton struct {
	ActionURL string `json:"actionURL,omitempty"`
	Title     string `json:"title,omitempty"`
}

type DingDingAt struct {
	AtMobiles []string `json:"atMobiles"`
	IsAtAll   bool     `json:"isAtAll"`
}

// ValidateDingDingResponse checks the business result returned by a DingTalk
// custom bot. DingTalk may return HTTP 200 for a rejected message, so the
// response body must be checked separately from the transport error.
func ValidateDingDingResponse(body []byte) error {
	var response struct {
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse DingTalk response: %w", err)
	}
	if response.ErrCode == nil {
		return fmt.Errorf("DingTalk response is missing errcode")
	}
	if *response.ErrCode != 0 {
		return fmt.Errorf("DingTalk response error: errcode=%d, errmsg=%s", *response.ErrCode, response.ErrMsg)
	}
	return nil
}

const (
	DingDingMsgType         = "actionCard"
	DingDingMarkdownMsgType = "markdown"
	dingDingAtContentPrefix = "##### **相关人员**:"
)

func (w *Service) sendDingDingMessage(uri, title, content, actionURL string, atMobiles []string, isAtAll bool) error {
	message := BuildDingDingMessage(title, content, actionURL, atMobiles, isAtAll)
	response, err := w.SendMessageRequest(uri, message)
	if err != nil {
		return err
	}
	return ValidateDingDingResponse(response)
}

func BuildDingDingMessage(title, content, actionURL string, atMobiles []string, isAtAll bool) *DingDingMessage {
	if len(atMobiles) > 0 || isAtAll {
		return &DingDingMessage{
			MsgType: DingDingMarkdownMsgType,
			MarkDown: &DingDingMarkDown{
				Title: title,
				Text:  buildDingDingMarkdownText(content, actionURL, atMobiles, isAtAll),
			},
			At: &DingDingAt{
				AtMobiles: atMobiles,
				IsAtAll:   isAtAll,
			},
		}
	}

	// reference: https://open.dingtalk.com/document/orgapp/message-link-description
	dingtalkRedirectURL := fmt.Sprintf("dingtalk://dingtalkclient/page/link?url=%s&pc_slide=false",
		url.QueryEscape(actionURL),
	)

	return &DingDingMessage{
		MsgType: DingDingMsgType,
		ActionCard: &DingDingActionCard{
			HideAvatar:        "0",
			ButtonOrientation: "0",
			Text:              content,
			Title:             title,
			Buttons: []*DingDingButton{
				{
					Title:     "点击查看更多信息",
					ActionURL: dingtalkRedirectURL,
				},
			},
		},
		At: &DingDingAt{
			AtMobiles: atMobiles,
			IsAtAll:   isAtAll,
		},
	}
}

func buildDingDingMarkdownText(content, actionURL string, atMobiles []string, isAtAll bool) string {
	text := strings.TrimSpace(content)
	if actionURL != "" {
		text = strings.TrimSpace(fmt.Sprintf("%s\n\n[点击查看更多信息](%s)", text, actionURL))
	}
	if strings.Contains(text, dingDingAtContentPrefix) {
		return text
	}

	if len(atMobiles) > 0 {
		return strings.TrimSpace(fmt.Sprintf("%s\n%s @%s", text, dingDingAtContentPrefix, strings.Join(atMobiles, "@")))
	}
	if isAtAll {
		return strings.TrimSpace(fmt.Sprintf("%s\n%s @所有人", text, dingDingAtContentPrefix))
	}
	return text
}
