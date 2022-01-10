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

func (w *Service) sendDingDingMessage(uri string, title string, content string, atMobiles []string) error {
	message := &DingDingMessage{
		MsgType: msgType,
		MarkDown: &DingDingMarkDown{
			Title: title,
			Text:  content,
		},
	}
	if len(atMobiles) > 0 {
		message.At = &DingDingAt{
			AtMobiles: atMobiles,
			IsAtAll:   false,
		}
	} else {
		message.At = &DingDingAt{
			IsAtAll: true,
		}
	}

	_, err := w.SendMessageRequest(uri, message)
	return err
}
