package instantmessage

import (
	"strings"
)

const (
	feiShuType           = "feishu"
	feishuCardType       = "interactive"
	feishuHeaderTemplate = "turquoise"
	feiShuTagText        = "plain_text"
	feishuTagMd          = "lark_md"
	feishuTagAction      = "action"
	feishuTagDiv         = "div"
	feishuTagButton      = "button"
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

func (lc *LarkCard) AddI18NElementsZhcnFeild(content string) {
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
	zhcnElem := &ZhCn{
		Fields: []*Field{field},
		Tag:    feishuTagDiv,
	}
	lc.I18NElements.ZhCn = append(lc.I18NElements.ZhCn, zhcnElem)
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
	var message interface{}
	message = LarkCardReq{
		MsgType: feishuCardType,
		Card:    lcMsg,
	}
	_, err := w.SendMessageRequest(uri, message)
	return err
}

func (w *Service) sendFeishuMessageOfSingleType(title, uri, content string) error {
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
