package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestProjectDetailRejectsAnonymousBeforeDatabaseAccess(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	r := gin.New()
	r.GET("/products/:name", GetProductTemplate)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/products/private-project", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("项目详情必须在读取 MongoDB 前拒绝匿名请求，实际 %d", w.Code)
	}
}
