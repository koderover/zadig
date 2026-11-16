package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/handler"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
)

// @title Zadig halman service REST APIs
// @version 1.0
// @description Zadig AI 助手接口，使用当前用户登录态；查看历史快照与执行资源操作分别校验权限。
// @description 所有 /api/halman 接口均要求有效企业版许可证；先通过 user 的 gRPC 登录鉴权，再校验许可证，失败时拒绝处理。
// @description 运行事件统一使用 AG-UI，通过 text/event-stream 传输；操作清单必须通过指定版本的确认接口授权。
// @contact.email contact@koderover.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
func NewEngine(agentService *service.AgentService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.HandleMethodNotAllowed = true
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/readyz", func(c *gin.Context) {
		if agentService.Store.SQLDB.PingContext(c.Request.Context()) != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		c.Status(http.StatusOK)
	})
	(&handler.Router{Service: agentService}).Inject(r.Group(""))
	return r
}
