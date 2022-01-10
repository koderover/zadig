package instantmessage

import (
	"fmt"
)

const (
	markdownColorInfo               = "info"
	markdownColorComment            = "comment"
	markdownColorWarning            = "warning"
	weChatTextTypeText     TextType = "text"
	weChatTextTypeMarkdown TextType = "markdown"
)

type WeChatWorkCard struct {
	MsgType  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown"`
}
type Markdown struct {
	Content string `json:"content"`
}

//wechat
type Messsage struct {
	MsgType string `json:"msgtype"`
	Text    *Text  `json:"markdown"`
}

type Text struct {
	Content string `json:"content"`
}

func (w *Service) SendWeChatWorkMessage(textType TextType, uri, content string) error {
	var message interface{}
	if textType == weChatTextTypeText {
		message = &Messsage{
			MsgType: msgType,
			Text: &Text{
				Content: content,
			},
		}
	} else if textType == weChatTextTypeMarkdown {
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