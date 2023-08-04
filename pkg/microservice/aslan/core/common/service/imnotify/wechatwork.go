/*
Copyright 2023 The KodeRover Authors.

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

package imnotify

import (
	"fmt"
)

const (
	WeChatWorkType                  = "wechat"
	MarkdownColorInfo               = "info"
	MarkdownColorComment            = "comment"
	MarkdownColorWarning            = "warning"
	WeChatTextTypeText     TextType = "text"
	WeChatTextTypeMarkdown TextType = "markdown"
)

type TextType string

type WeChatWorkCard struct {
	MsgType  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown"`
}
type Markdown struct {
	Content string `json:"content"`
}

type Messsage struct {
	MsgType string `json:"msgtype"`
	Text    *Text  `json:"markdown"`
}

type Text struct {
	Content string `json:"content"`
}

func (w *IMNotifyService) SendWeChatWorkMessage(textType TextType, uri, content string) error {
	var message interface{}
	if textType == WeChatTextTypeText {
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
	} else {
		return fmt.Errorf("SendWeChatWorkMessage err:%s", "WeChatWork textType is invalid")
	}

	_, err := w.SendMessageRequest(uri, message)
	return err
}
