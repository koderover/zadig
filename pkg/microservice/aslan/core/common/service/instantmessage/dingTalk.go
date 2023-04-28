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

const (
	dingDingType = "dingding"
)

type DingDingMessage struct {
	MsgType  string            `json:"msgtype"`
	MarkDown *DingDingMarkDown `json:"markdown"`
	At       *DingDingAt       `json:"at"`
}

type DingDingMarkDown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type DingDingAt struct {
	AtMobiles []string `json:"atMobiles"`
	IsAtAll   bool     `json:"isAtAll"`
}

func (w *Service) sendDingDingMessage(uri, title, content string, atMobiles []string, isAtAll bool) error {
	message := &DingDingMessage{
		MsgType: msgType,
		MarkDown: &DingDingMarkDown{
			Title: title,
			Text:  content,
		},
	}
	message.At = &DingDingAt{
		AtMobiles: atMobiles,
		IsAtAll:   isAtAll,
	}

	_, err := w.SendMessageRequest(uri, message)
	return err
}
