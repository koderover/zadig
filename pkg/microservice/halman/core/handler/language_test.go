package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

func TestAssistantErrorLanguageKeepsProtocol(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request = c.Request.WithContext(language.WithContext(c.Request.Context(), "en-us"))
	respondError(c, common.ErrBusy)
	var body struct {
		Code                 int
		Message, Description string
	}
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || w.Code != 400 || body.Code != 7215 || body.Message != "You have an active run" || body.Description != "You have an active run; wait or cancel it before trying again" {
		t.Fatalf("错误只切换展示语言，协议不变: %s", w.Body.String())
	}
	if common.ErrBusy.Error() != "用户有正在处理的运行，请等待或取消后重试" {
		t.Fatal("不得修改共享错误对象或历史文案")
	}
}

func TestDeleteUnknownResultShowsDeleteMessage(t *testing.T) {
	for _, tc := range []struct{ locale, message string }{
		{"zh-CN", "无法删除会话，请先核实操作结果"},
		{"en-US", "Cannot delete this conversation. Verify the operation outcome first."},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest("DELETE", "/", nil)
			c.Request = req.WithContext(language.WithContext(req.Context(), tc.locale))
			respondError(c, common.ErrResultUnknown)
			var body struct {
				Code    int
				Message string
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 400 || body.Code != 7221 || body.Message != tc.message {
				t.Fatalf("删除会话应返回专用展示文案: %d %s", w.Code, w.Body)
			}
		})
	}
}
