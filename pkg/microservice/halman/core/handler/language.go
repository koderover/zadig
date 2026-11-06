package handler

import (
	"github.com/gin-gonic/gin"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

// 只翻译助手专属错误文案，保留公共错误码及平台原始错误响应。
func respondHTTPError(c *gin.Context, err *e.HTTPError) {
	status, body := e.ErrorMessage(err)
	if responseLanguage(c) == language.English {
		messages := map[int]string{
			7213: "AI assistant request failed",
			7214: "The administrator has disabled the AI assistant",
			7215: "You have an active run",
			7216: "AI assistant request or plan state conflict",
			7217: "The AI assistant is busy; please try again shortly",
			7218: "The AI assistant model is unavailable",
			7219: "The AI assistant runtime configuration is unavailable",
			7220: "The AI assistant could not access the Zadig resource",
			7221: "The operation outcome is unknown; do not retry directly",
		}
		if message := messages[err.Code()]; message != "" {
			body["message"] = message
		}
	}
	c.AbortWithStatusJSON(status, body)
}

func responseLanguage(c *gin.Context) language.Language {
	if c.Request == nil {
		return language.Chinese
	}
	return language.FromContext(c.Request.Context())
}
