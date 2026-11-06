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
