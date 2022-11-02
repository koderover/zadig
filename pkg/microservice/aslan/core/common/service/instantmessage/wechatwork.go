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
	weChatWorkType                  = "wechat"
	markdownColorInfo               = "info"
	markdownColorComment            = "comment"
	markdownColorWarning            = "warning"
	weChatTextTypeText     TextType = "text"
	weChatTextTypeMarkdown TextType = "markdown"
)

type TextType string

type WeChatWorkCard struct {
	MsgType  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown"`
}
type Markdown struct {
	Content             string   `json:"content"`
	MentionedMobileList []string `json:"mentioned_mobile_list"`
}

type Messsage struct {
	MsgType string `json:"msgtype"`
	Text    *Text  `json:"text"`
}

type Text struct {
	Content             string   `json:"content"`
	MentionedMobileList []string `json:"mentioned_mobile_list"`
}

func (w *Service) SendWeChatWorkMessage(textType TextType, uri, content string, atMobiles []string) error {
	var message interface{}
	if textType == weChatTextTypeText {
		message = &Messsage{
			MsgType: msgType,
			Text: &Text{
				Content:             content,
				MentionedMobileList: atMobiles,
			},
		}
	} else if textType == weChatTextTypeMarkdown {
		message = &WeChatWorkCard{
			MsgType: msgType,
			Markdown: Markdown{
				Content:             content,
				MentionedMobileList: atMobiles,
			},
		}
	} else {
		return fmt.Errorf("SendWeChatWorkMessage err:%s", "WeChatWork textType is invalid")
	}

	_, err := w.SendMessageRequest(uri, message)
	return err
}
