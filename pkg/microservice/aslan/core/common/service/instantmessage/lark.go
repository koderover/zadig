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
	"strings"
	"sync"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

const (
	feiShuType                    = "feishu"
	feishuCardType                = "interactive"
	feishuHeaderTemplateTurquoise = "turquoise"
	feishuHeaderTemplateGreen     = "green"
	feishuHeaderTemplateRed       = "red"
	feiShuTagText                 = "plain_text"
	feishuTagMd                   = "lark_md"
	feishuTagAction               = "action"
	feishuTagDiv                  = "div"
	feishuTagButton               = "button"
)

type LarkCardReq struct {
	MsgType string    `json:"msg_type"`
	Card    *LarkCard `json:"card"`
}

type LarkCard struct {
	Config       *Config       `json:"config"`
	Header       *Header       `json:"header"`
	I18NElements *I18NElements `json:"i18n_elements"`
}

type Config struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

type Title struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

type Header struct {
	Template string `json:"template"`
	Title    Title  `json:"title"`
}

type TextElem struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

type Field struct {
	IsShort bool     `json:"is_short"`
	Text    TextElem `json:"text"`
}

type Action struct {
	Tag  string   `json:"tag"`
	Text TextElem `json:"text"`
	Type string   `json:"type"`
	URL  string   `json:"url"`
}

type ZhCn struct {
	Fields  []*Field  `json:"fields,omitempty"`
	Tag     string    `json:"tag"`
	Actions []*Action `json:"actions,omitempty"`
}

type I18NElements struct {
	ZhCn []*ZhCn `json:"zh_cn"`
}

type FeiShuMessage struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type FeiShuMessageV2 struct {
	MsgType string          `json:"msg_type"`
	Content FeiShuContentV2 `json:"content"`
}

type FeiShuContentV2 struct {
	Text string `json:"text"`
}

func NewLarkCard() *LarkCard {
	return &LarkCard{
		Config:       &Config{},
		Header:       &Header{},
		I18NElements: &I18NElements{ZhCn: make([]*ZhCn, 0)},
	}
}

func (lc *LarkCard) SetConfig(wideScreenMode bool) {
	if lc.Config == nil {
		lc.Config = &Config{}
	}
	lc.Config.WideScreenMode = wideScreenMode
}

func (lc *LarkCard) SetHeader(template, title, tag string) {
	if lc.Header == nil {
		lc.Header = &Header{}
	}
	lc.Header.Template = template
	lc.Header.Title = Title{
		Content: title,
		Tag:     tag,
	}
}

func (lc *LarkCard) AddI18NElementsZhcnFeild(content string, isCreatefield bool) {
	if lc.I18NElements == nil {
		lc.I18NElements = &I18NElements{
			ZhCn: make([]*ZhCn, 0),
		}
	}

	field := &Field{
		IsShort: false,
		Text: TextElem{
			Content: content,
			Tag:     feishuTagMd,
		},
	}
	var mutex sync.RWMutex
	mutex.Lock()
	defer mutex.Unlock()
	lengthZhCn := len(lc.I18NElements.ZhCn)
	if isCreatefield || lengthZhCn == 0 {
		zhcnElem := &ZhCn{
			Fields: []*Field{field},
			Tag:    feishuTagDiv,
		}
		lc.I18NElements.ZhCn = append(lc.I18NElements.ZhCn, zhcnElem)
		return
	}
	lengthFields := len(lc.I18NElements.ZhCn[lengthZhCn-1].Fields)
	if lengthFields > 0 {
		lc.I18NElements.ZhCn[lengthZhCn-1].Fields = append(lc.I18NElements.ZhCn[lengthZhCn-1].Fields, field)
	}
}

func (lc *LarkCard) AddI18NElementsZhcnAction(content, url string) {
	if lc.I18NElements == nil {
		lc.I18NElements = &I18NElements{
			ZhCn: make([]*ZhCn, 0),
		}
	}
	action := &Action{
		Tag:  feishuTagButton,
		Text: TextElem{Content: content, Tag: feiShuTagText},
		Type: "primary",
		URL:  url,
	}
	zhcnElem := &ZhCn{
		Actions: []*Action{action},
		Tag:     feishuTagAction,
	}
	lc.I18NElements.ZhCn = append(lc.I18NElements.ZhCn, zhcnElem)
}

func (w *Service) sendFeishuMessage(uri string, lcMsg *LarkCard) error {
	message := LarkCardReq{
		MsgType: feishuCardType,
		Card:    lcMsg,
	}
	_, err := w.SendMessageRequest(uri, message)
	return err
}

func (w *Service) sendFeishuMessageOfSingleType(title, uri, content string) error {
	if content == "" {
		return nil
	}
	var message interface{}
	message = &FeiShuMessage{
		Title: title,
		Text:  content,
	}
	if strings.Contains(uri, "bot/v2/hook") {
		message = &FeiShuMessageV2{
			MsgType: "text",
			Content: FeiShuContentV2{
				Text: content,
			},
		}
	}
	_, err := w.SendMessageRequest(uri, message)
	return err
}

func getColorTemplateWithStatus(status config.Status) string {
	if status == config.StatusPassed || status == config.StatusCreated {
		return feishuHeaderTemplateGreen
	}
	return feishuHeaderTemplateRed
}
