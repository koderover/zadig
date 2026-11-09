package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

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
