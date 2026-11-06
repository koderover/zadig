package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestTemplateCatalogMatchesFullNameAcrossOriginalPages(t *testing.T) {
	for _, tc := range []struct{ kind, path, key string }{
		{"yaml_templates", "/api/template/yaml", "yamlTemplates"},
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
	if _, err := filterTemplatePage(json.RawMessage(`{"yamlTemplates":[],"total":51}`), "yaml_templates", "Go", 1); err == nil {
		t.Fatal("总量表明仍有模板时，不能将不完整响应当作模板不存在")
	}
	if _, err := filterTemplatePage(json.RawMessage(`{"build_templates":null}`), "build_templates", "Go", 1); err != nil {
		t.Fatalf("原接口空列表可能不返回 total，应正常处理: %v", err)
	}
}
