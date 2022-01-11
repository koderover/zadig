package instantmessage

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

const (
	weChatWorkType                  = "wechat"
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

func getColorWithStatus(status config.Status) string {
	if status == config.StatusPassed {
		return markdownColorInfo
	} else if status == config.StatusTimeout || status == config.StatusCancelled || status == config.StatusNotRun {
		return markdownColorComment
	} else if status == config.StatusFailed {
		return markdownColorWarning
	}
	return markdownColorComment
}
