package common

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

var (
	ErrInvalidRequest   = errors.New("请求参数不正确")
	ErrNotFound         = errors.New("会话或运行不存在")
	ErrBusy             = errors.New("用户有正在处理的运行，请等待或取消后重试")
	ErrRequestConflict  = errors.New("请求标识已用于其他参数")
	ErrModelUnavailable = errors.New("AI 助手模型不可用")
	ErrCapacity         = errors.New("当前请求较多，暂时无法开始处理，请稍后重试")
	ErrAIDisabled       = errors.New("管理员已关闭 AI 助手")
	ErrInvalidCursor    = errors.New("分页游标无效")
)

type Cursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func EncodeCursor(at time.Time, id string) string {
	b, _ := json.Marshal(Cursor{At: at, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeCursor(value string) (Cursor, error) {
	var c Cursor
	if value == "" {
		return c, nil
	}

	if len(value) > 512 {
		return c, ErrInvalidCursor
	}

	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(b, &c) != nil || c.At.IsZero() || c.ID == "" {
		return c, ErrInvalidCursor
	}

	return c, nil
}

func NewID() string { return uuid.NewString() }

func Terminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "cancelled" || status == "interrupted" || status == "awaiting_input" ||
		status == "pending_confirmation" ||
		status == "unknown" ||
		status == "expired"
}

func ValidateRequest(req models.RunRequest) error {
	if len(req.RequestID) < 1 || len(req.RequestID) > 64 {
		return fmt.Errorf("request_id 长度必须在 1 到 64 之间")
	}

	for _, r := range req.RequestID {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return fmt.Errorf("request_id 仅支持字母、数字、横线和下划线")
		}
	}

	if !utf8.ValidString(req.Message) || strings.TrimSpace(req.Message) == "" || len(req.Message) > 16000 {
		return fmt.Errorf("消息不能为空且不能超过 16000 字节")
	}

	if req.Context.TaskID < 0 {
		return fmt.Errorf("任务 ID 无效")
	}

	return nil
}

func JSON(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
