package service

import (
	"encoding/json"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func TestCatalogClarificationAfterExhaustedTemplateSearch(t *testing.T) {
	for _, kind := range []string{"yaml_templates", "build_templates"} {
		t.Run(kind, func(t *testing.T) {
			e := &execution{}
			args := models.CatalogRequest{Kind: kind, Keyword: "general"}
			recordCatalogResult(e, args, json.RawMessage(`{"items":[],"next_page":2}`), nil)
			if e.CatalogNeedsInput {
				t.Fatal("must allow subsequent pages")
			}
			recordCatalogResult(e, args, json.RawMessage(`{"items":[],"next_page":0}`), nil)
			// A successful independent query in the same batch must not reopen guessing.
			recordCatalogResult(e, models.CatalogRequest{Kind: "clusters"}, json.RawMessage(`[]`), nil)
			req := &model.Request{Tools: map[string]tool.Tool{"query_catalog": nil, "project_onboarding": nil}}
			applyCatalogClarification(e, req)
			if _, ok := req.Tools["query_catalog"]; ok || len(req.Messages) != 1 {
				t.Fatal("must stop catalog calls and instruct the model to clarify")
			}
			if _, ok := req.Tools["project_onboarding"]; !ok {
				t.Fatal("must retain draft saving")
			}
		})
	}
}

func TestCatalogClarificationAllowsMatchesAndUnfilteredLists(t *testing.T) {
	for _, tc := range []struct{ keyword, result string }{
		{"general", `{"items":[{"id":"template","name":"general"}],"next_page":0}`},
		{"", `{"items":[],"next_page":0}`},
	} {
		e := &execution{}
		recordCatalogResult(e, models.CatalogRequest{Kind: "yaml_templates", Keyword: tc.keyword}, json.RawMessage(tc.result), nil)
		req := &model.Request{Tools: map[string]tool.Tool{"query_catalog": nil}}
		applyCatalogClarification(e, req)
		if _, ok := req.Tools["query_catalog"]; !ok || len(req.Messages) != 0 {
			t.Fatal("normal lookups must remain available")
		}
	}
}
