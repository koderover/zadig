package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestBatchConfirmationReadsEachTemplateOnceAndSubmissionStillRefreshes(t *testing.T) {
	digest := "original"
	reads := map[string]int{}
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		reads[r.URL.Path]++
		switch r.URL.Path {
		case "/api/template/charts/base", "/api/template/build/base":
			w.Write(common.JSON(map[string]any{"sha1": digest, "variables": []any{}}))
		case "/api/project/products/p/services":
			w.Write([]byte(`{"services":[]}`))
		default:
			t.Fatalf("模板变化后不能提交写请求: %s", r.URL)
		}
	})
	raw := common.JSON(map[string]any{"sha1": digest, "variables": []any{}})
	plan := models.OperationPlan{}
	for i := 0; i < 20; i++ {
		plan.Items = append(plan.Items,
			models.OperationItem{Action: "create_service", ProjectKey: "p", Params: common.JSON(models.ServiceInput{Name: fmt.Sprintf("svc-%d", i), Source: "helm_template", TemplateID: "base", TemplateFingerprint: fingerprint(raw)})},
			models.OperationItem{Action: "create_build", ProjectKey: "p", Params: common.JSON(models.BuildInput{TemplateID: "base", TemplateFingerprint: fingerprint(raw)})},
		)
	}
	s := &AgentService{Platform: platform}
	e := reportExecution("service_onboarding")
	for phase := 1; phase <= 2; phase++ {
		if err := s.verifyPlan(context.Background(), e, plan); err != nil {
			t.Fatal(err)
		}
		if reads["/api/template/charts/base"] != phase || reads["/api/template/build/base"] != phase {
			t.Fatalf("每次确认各读一次模板，不能跨确认复用: %+v", reads)
		}
	}
	ctx := withReadPhase(context.Background())
	if _, err := platform.catalog(ctx, e.Authorization, models.CatalogRequest{Kind: "helm_template", ProjectKey: "p", ID: "base"}); err != nil {
		t.Fatal(err)
	}
	digest = "changed"
	if _, err := s.submitOperation(ctx, e, plan.Items[0]); err == nil || reads["/api/template/charts/base"] != 4 {
		t.Fatalf("提交前必须重读最新模板并拒绝变更: %+v %v", reads, err)
	}
}

func TestReadPhaseMergesCodehostsAndExistingServicesOnlyWithinSameScope(t *testing.T) {
	calls := map[string]int{}
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		if r.URL.Path == "/api/code/codehost" {
			w.Write([]byte(`[{"id":1,"type":"github"}]`))
		} else {
			w.Write([]byte(`{"services":[]}`))
		}
	})
	ctx := withReadPhase(context.Background())
	e := reportExecution("service_onboarding")
	s := &AgentService{Platform: platform}
	for i := 0; i < 20; i++ {
		if err := platform.requireGitSource(ctx, e.Authorization, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.preflightService(ctx, e, "p", models.ServiceInput{Name: fmt.Sprintf("svc-%d", i), Source: "yaml_template"}, true); err != nil {
			t.Fatal(err)
		}
	}
	if calls["/api/code/codehost"] != 1 || calls["/api/project/products/p/services"] != 1 {
		t.Fatalf("同一批应合并目录和服务列表读取: %+v", calls)
	}
	if err := platform.requireGitSource(ctx, token("other-user"), 1); err != nil || calls["/api/code/codehost"] != 2 {
		t.Fatal("不能跨用户复用读取结果")
	}
	if err := platform.requireGitSource(withReadPhase(ctx), e.Authorization, 1); err != nil || calls["/api/code/codehost"] != 3 {
		t.Fatal("新阶段必须重新读取")
	}
	for _, query := range []url.Values{{"id": {"1"}}, {"id": {"2"}}} {
		if _, err := platform.business(ctx, e.Authorization, http.MethodGet, "/api/code/codehost", query, nil); err != nil {
			t.Fatal(err)
		}
	}
	if calls["/api/code/codehost"] != 5 {
		t.Fatal("不同查询参数不能复用响应")
	}
}

func TestReadPhaseDoesNotCacheFailuresOversizeOrBypassTokenRevocation(t *testing.T) {
	calls := 0
	oversize := true
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if oversize {
			w.Write(common.JSON(strings.Repeat("x", 1<<20)))
		} else {
			w.Write([]byte(`{}`))
		}
	})
	ctx := withReadPhase(context.Background())
	for i := 0; i < 3; i++ {
		_, err := platform.business(ctx, token("user"), http.MethodGet, "/api/template/yaml/base", nil, nil)
		if (err != nil) != (i == 0) {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatal("失败及超过阶段内存预算的响应不能缓存")
	}
	oversize = false
	for i := 0; i < 2; i++ {
		if _, err := platform.business(ctx, token("user"), http.MethodGet, "/api/template/yaml/base", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 4 {
		t.Fatal("应已取得可命中的阶段内缓存")
	}
	platform.Auth = fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) { return nil, errors.New("revoked") }}
	if _, err := platform.business(ctx, token("user"), http.MethodGet, "/api/template/yaml/base", nil, nil); err == nil || calls != 4 {
		t.Fatal("命中缓存前仍须检查登录态")
	}
}

func TestReadPhaseHasCumulativeMemoryBudget(t *testing.T) {
	reads := map[string]int{}
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		reads[r.URL.Path]++
		w.Write(common.JSON(strings.Repeat("x", 600<<10)))
	})
	ctx := withReadPhase(context.Background())
	for i := 0; i < 2; i++ {
		for _, path := range []string{"/api/template/yaml/one", "/api/template/yaml/two"} {
			if _, err := platform.business(ctx, token("user"), http.MethodGet, path, nil, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if reads["/api/template/yaml/one"] != 1 || reads["/api/template/yaml/two"] != 2 {
		t.Fatalf("超过累计预算后保留原读取行为，不能无限保留大模板: %+v", reads)
	}
}
