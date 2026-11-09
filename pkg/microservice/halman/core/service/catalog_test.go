package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestHelmRepositoryCatalogUsesReadOnlyListsAndMatchesRepositoryName(t *testing.T) {
	for _, project := range []string{"", "aiceshi-helm"} {
		t.Run("project="+project, func(t *testing.T) {
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				path := "/api/system/helm/public"
				if project != "" {
					path = "/api/system/helm/project"
				}
				if r.Method != http.MethodGet || r.URL.Path != path || r.URL.Query().Get("projectName") != project || r.Header.Get("Authorization") != token("user") || r.URL.Query().Has("encryptedKey") {
					t.Fatalf("应复用实际用户可读的仓库列表，不调用需要解密参数的管理接口: %s %s", r.Method, r.URL)
				}
				w.Write([]byte(`[{"id":"507f1f77bcf86cd799439011","repo_name":"test","username":"hidden-user","password":"hidden-password","url":"https://hidden-url","projects":["hidden-project"]},{"id":"other","repo_name":"other"}]`))
			})
			raw, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: "helm_repositories", ProjectKey: project, Keyword: "test"})
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != `[{"id":"507f1f77bcf86cd799439011","repo_name":"test"}]` {
				t.Fatalf("应按 repo_name 匹配，只保留选择所需元数据: %s", raw)
			}
		})
	}
}

func TestHelmRepositoryCatalogDoesNotFallBackAfterPermissionDenied(t *testing.T) {
	calls := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/system/helm/project" {
			t.Fatalf("项目列表拒绝访问后不能退回全局列表: %s", r.URL)
		}
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: "helm_repositories", ProjectKey: "aiceshi-helm"})
	var denied *PlatformError
	if !errors.As(err, &denied) || denied.Status != http.StatusForbidden || calls != 1 {
		t.Fatalf("必须保留权限错误，不重试其他入口: %v calls=%d", err, calls)
	}
}

func TestChartVersionsAcceptRepositoryNameAndReturnAllCharts(t *testing.T) {
	index := `{"entries":{"service1":[{"chart_name":"service1","version":"v20260908183547"},{"chart_name":"service1","version":"1.0.0"}],"service2":[{"chart_name":"service2","version":"v20260908183547"}],"service3":[{"chart_name":"service3","version":"4.2"}]}}`
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/system/helm/test/index" {
			t.Fatalf("Chart 索引须使用仓库名称: %s %s", r.Method, r.URL)
		}
		w.Write([]byte(index))
	})
	for _, args := range []models.CatalogRequest{
		{Kind: "chart_versions", RepoName: "test"},
		{Kind: "chart_versions", ID: "test"},
	} {
		raw, err := platform.catalog(context.Background(), token("user"), args)
		if err != nil || string(raw) != index {
			t.Fatalf("一次返回全部 Chart 的版本并保留排序，兼容已有 id 调用: %s %v", raw, err)
		}
	}
	for _, version := range []string{"v20260908183547", "4.2"} {
		name := "service1"
		if version == "4.2" {
			name = "service3"
		}
		if err := platform.checkConfiguration(context.Background(), token("user"), models.ServiceInput{Name: name, Source: "helm_repository", ChartRepo: "test", ChartName: name, ChartVersion: version}); err != nil {
			t.Fatalf("清单预检应继续使用同一仓库名称查询接口: %v", err)
		}
	}
}

func TestChartVersionsMissingRepositoryExplainsRequiredParameter(t *testing.T) {
	platform := testPlatform(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("缺少仓库名称时不发送业务请求")
	})
	_, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: "chart_versions", Keyword: "service1"})
	if err == nil || !strings.Contains(err.Error(), "repo_name") {
		t.Fatalf("应提示仓库参数，不返回笼统的资源标识错误: %v", err)
	}
}

func TestTemplateCatalogMatchesFullNameAcrossOriginalPages(t *testing.T) {
	for _, tc := range []struct{ kind, path, key string }{
		{"yaml_templates", "/api/template/yaml", "yaml_template"},
		{"build_templates", "/api/template/build", "build_templates"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				if r.URL.Path != tc.path || query.Has("keyword") || query.Get("page_size") != "50" {
					t.Fatalf("必须复用原列表，不传入新增搜索参数: %s", r.URL)
				}
				items := []map[string]string{}
				switch query.Get("page_num") {
				case "1":
					for range 50 {
						items = append(items, map[string]string{"id": "similar", "name": "Go.[标准] v2"})
					}
				case "2":
					items = append(items, map[string]string{"id": "target", "name": "Go.[标准]", "content": "不得传给模型"})
				default:
					t.Fatalf("意外页码: %s", r.URL)
				}
				w.Write(common.JSON(map[string]any{tc.key: items, "total": 51}))
			})
			for _, page := range []int{0, 2} {
				raw, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: tc.kind, Keyword: "Go.[标准]", Page: page})
				if err != nil {
					t.Fatal(err)
				}
				value, err := safeCatalogJSON(raw)
				if err != nil {
					t.Fatal(err)
				}
				var response struct {
					Items    []struct{ ID, Name string }
					Total    int
					Page     int
					NextPage int `json:"next_page"`
				}
				if err := json.Unmarshal(common.JSON(value), &response); err != nil {
					t.Fatal(err)
				}
				if response.Total != 51 {
					t.Fatalf("必须保留原分页总量: %s", raw)
				}
				if page == 0 {
					if len(response.Items) != 0 || response.Page != 1 || response.NextPage != 2 {
						t.Fatalf("不能将相似名称当成指定模板，未找到时应能继续查询: %s", raw)
					}
				} else if len(response.Items) != 1 || response.Items[0].ID != "target" || response.NextPage != 0 {
					t.Fatalf("后续页必须可按完整名称解析 ID: %s", raw)
				}
			}
			raw, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: tc.kind})
			if err != nil {
				t.Fatal(err)
			}
			var listed struct{ Items []any }
			if json.Unmarshal(raw, &listed) != nil || len(listed.Items) != 50 {
				t.Fatalf("未指定名称时应提供当前页候选: %s", raw)
			}
			raw, err = platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: tc.kind, Keyword: "不存在", Page: 2})
			if err != nil {
				t.Fatal(err)
			}
			var missing struct {
				Items    []any
				NextPage int `json:"next_page"`
			}
			if json.Unmarshal(raw, &missing) != nil || len(missing.Items) != 0 || missing.NextPage != 0 {
				t.Fatalf("读到末页后才能结束查找: %s", raw)
			}
		})
	}
}

func TestTemplateCatalogRejectsInvalidPageBeforeRequest(t *testing.T) {
	platform := testPlatform(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("非法页码不得请求 aslan")
	})
	for _, page := range []int{-1, int(^uint(0) >> 1)} {
		if _, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: "yaml_templates", Page: page}); err == nil {
			t.Fatal("必须拒绝负页码和会导致分页溢出的页码")
		}
	}
}

func TestTemplateCatalogRejectsIncompletePage(t *testing.T) {
	if _, err := filterTemplatePage(json.RawMessage(`{"yaml_template":[],"total":51}`), "yaml_templates", "Go", 1); err == nil {
		t.Fatal("总量表明仍有模板时，不能将不完整响应当作模板不存在")
	}
	if _, err := filterTemplatePage(json.RawMessage(`{"build_templates":null}`), "build_templates", "Go", 1); err != nil {
		t.Fatalf("原接口空列表可能不返回 total，应正常处理: %v", err)
	}
}

func TestYAMLTemplateNameResolvesToIDBeforeReadingDetails(t *testing.T) {
	const id = "507f1f77bcf86cd799439011"
	calls := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/api/template/yaml":
			// 与 aslan 的 ListYamlResp 一致：列表字段为 yaml_template，名称与 ID 不相同。
			w.Write([]byte(`{"system_variables":[],"yaml_template":[{"id":"507f1f77bcf86cd799439011","name":"general"},{"id":"507f1f77bcf86cd799439012","name":"general-chart"}],"total":2}`))
		case "/api/template/yaml/" + id:
			w.Write([]byte(`{"id":"507f1f77bcf86cd799439011","name":"general","service_variable_kvs":[{"key":"IMAGE","value":"example/app:v1","type":"string"}]}`))
		default:
			t.Fatalf("只能按列表返回的真实 ID 查详情，不能将 general 当成 ID: %s", r.URL)
		}
	})
	raw, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: "yaml_templates", Keyword: "general"})
	if err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Items    []struct{ ID, Name string }
		Total    int
		NextPage int `json:"next_page"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil || len(listed.Items) != 1 || listed.Items[0].ID != id || listed.Items[0].Name != "general" || listed.Total != 2 || listed.NextPage != 0 {
		t.Fatalf("真实响应应能按完整名称解析 ID，不能误报分页失败或选择相似名称: %s %v", raw, err)
	}
	s := &AgentService{Platform: platform}
	detail, err := s.readTemplate(context.Background(), &execution{Authorization: token("user")}, "p", "yaml_template", listed.Items[0].ID, nil)
	if err != nil || !strings.Contains(string(detail), "service_variable_kvs") || calls != 2 {
		t.Fatalf("应能读取匹配模板的变量，不能反复重试列表: %s %v calls=%d", detail, err, calls)
	}
}

func TestCodehostCatalogAndGitValidationUseBusinessList(t *testing.T) {
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/codehosts" {
			http.Error(w, "missing encryptedKey", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/api/code/codehost" || r.Header.Get("Authorization") != token("user") || r.URL.Query().Has("encryptedKey") {
			t.Fatalf("应以实际用户查询已有业务代码源列表: %s %s", r.Method, r.URL)
		}
		w.Write([]byte(`[{"id":7,"alias":"gitlab","type":"gitlab","address":"https://gitlab.example.com","access_token":"hidden-token","ssh_key":"hidden-ssh","client_secret":"hidden-secret"},{"id":9,"alias":"other","type":"github"}]`))
	})
	raw, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: "codehosts", Keyword: "gitlab"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "hidden-") || strings.Contains(string(raw), "ssh_key") {
		t.Fatalf("代码源目录只应返回选择所需元数据: %s", raw)
	}
	var hosts []struct {
		ID    int    `json:"id"`
		Alias string `json:"alias"`
	}
	if err := json.Unmarshal(raw, &hosts); err != nil || len(hosts) != 1 || hosts[0].ID != 7 || hosts[0].Alias != "gitlab" {
		t.Fatalf("应从名称解析真实代码源 ID: %s %v", raw, err)
	}
	kind, err := platform.gitSource(context.Background(), token("user"), hosts[0].ID)
	if err != nil || kind != "gitlab" {
		t.Fatalf("构建及仓库预检必须复用可用的业务列表: %q %v", kind, err)
	}
}
