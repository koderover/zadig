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
	"fmt"
)

const (
	markdownColorInfo                   = "info"
	markdownColorComment                = "comment"
	markdownColorWarning                = "warning"
	weChatTextTypeText         TextType = "text"
	WeChatTextTypeMarkdown     TextType = "markdown"
	WeChatTextTypeTemplateCard TextType = "template_card"
)

type TextType string

type WeChatWorkCard struct {
	MsgType      string       `json:"msgtype"`
	Markdown     Markdown     `json:"markdown,omitempty"`
	TemplateCard TemplateCard `json:"template_card,omitempty"`
}
type Markdown struct {
	Content string `json:"content"`
}

type TemplateCard struct {
	CardType     string                `json:"card_type"`
	MainTitle    *TemplateCardTitle    `json:"main_title"`
	SubTitleText string                `json:"sub_title_text"`
	JumpList     []*WechatWorkLink     `json:"jump_list"`
	CardAction   *WechatWorkCardAction `json:"card_action"`
}

type TemplateCardTitle struct {
	Title       string `json:"title"`
	Description string `json:"desc"`
}

type WechatWorkLink struct {
	Type  int    `json:"type"` // 0 - non-link, 1 - url, 2 - application
	Title string `json:"title"`
	URL   string `json:"url"`
}

type WechatWorkCardAction struct {
	Type int    `json:"type"` // 1 - url, 2 - application
	URL  string `json:"url"`
}

type Messsage struct {
	MsgType string `json:"msgtype"`
	Text    *Text  `json:"markdown"`
}

type Text struct {
	Content string `json:"content"`
}

func (w *Service) SendWeChatWorkMessage(textType TextType, uri, link, title, content string) error {
	var message interface{}
	if textType == weChatTextTypeText {
		message = &Messsage{
			MsgType: msgType,
			Text: &Text{
				Content: content,
			},
		}
	} else if textType == WeChatTextTypeMarkdown {
		message = &WeChatWorkCard{
			MsgType: msgType,
			Markdown: Markdown{
				Content: content,
			},
		}
	} else if textType == WeChatTextTypeTemplateCard {
		message = &WeChatWorkCard{
			MsgType: string(WeChatTextTypeTemplateCard),
			TemplateCard: TemplateCard{
				CardType: "text_notice",
				MainTitle: &TemplateCardTitle{
					Title: title,
				},
				SubTitleText: content,
				JumpList: []*WechatWorkLink{
					{
						Type:  1,
						URL:   link,
						Title: "点击查看更多信息",
					},
				},
				CardAction: &WechatWorkCardAction{
					Type: 1,
					URL:  link,
				},
			},
		}
	} else {
		return fmt.Errorf("SendWeChatWorkMessage err:%s", "WeChatWork textType is invalid")
	}

	_, err := w.SendMessageRequest(uri, message)
	return err
}
