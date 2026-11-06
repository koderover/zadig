package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	envservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestLogWindowBounds(t *testing.T) {
	for _, input := range []struct {
		offset, limit string
		valid         bool
	}{
		{"0", "200", true}, {"5000", "500", true}, {"0", "1", true},
		{"-1", "200", false}, {"5001", "200", false}, {"0", "0", false},
		{"0", "501", false}, {"", "200", false}, {"0", "bad", false},
	} {
		_, _, err := parseLogWindow(input.offset, input.limit)
		if (err == nil) != input.valid {
			t.Fatalf("窗口 %q/%q 的校验结果错误: %v", input.offset, input.limit, err)
		}
	}
}

func TestExcerptRejectsInvalidOrAnonymousRequests(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	r := gin.New()
	r.GET("/workflow/:workflowName/:taskID/:jobName", GetWorkflowV4JobContainerLogs)
	r.GET("/pods/:name", GetContainerLogs)
	for _, input := range []struct {
		query  string
		status int
	}{
		{"/workflow/w/1/j?format=excerpt&offset=5001", http.StatusBadRequest},
		{"/workflow/w/1/j?format=excerpt", http.StatusForbidden},
		{"/pods/p?format=excerpt&tailLines=501", http.StatusBadRequest},
		{"/pods/p?format=excerpt&previous=bad", http.StatusBadRequest},
		{"/pods/p?format=excerpt&follow=true", http.StatusBadRequest},
		{"/pods/p?format=excerpt", http.StatusBadRequest},
		{"/pods/p?format=excerpt&projectName=p&envName=dev&serviceName=s&container=c", http.StatusForbidden},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, input.query, nil))
		if w.Code != input.status {
			t.Errorf("%s 应在访问数据库前拒绝，实际 %d", input.query, w.Code)
		}
	}
}

func TestContainerMustBelongToExactServicePod(t *testing.T) {
	pods := []*envservice.OpenAPIServicePodInfo{nil, {
		PodName: "api-1", Containers: []envservice.OpenAPIServicePodContainerInfo{{Name: "api"}},
	}, {
		PodName: "api-2", Containers: []envservice.OpenAPIServicePodContainerInfo{{Name: "sidecar"}},
	}}
	if !containsServiceContainer(pods, "api-1", "api") ||
		containsServiceContainer(pods, "api-1", "sidecar") ||
		containsServiceContainer(pods, "other-pod", "api") {
		t.Fatal("不能跨 Pod 或读取目标服务以外的容器")
	}
}
