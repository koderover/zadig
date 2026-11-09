package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *AgentService) catalog(ctx context.Context, args models.CatalogRequest) (any, error) {
	e, ok := ctx.Value(executionKey{}).(*execution)
	if !ok {
		return nil, fmt.Errorf("运行上下文缺失")
	}

	raw, err := s.Platform.catalog(ctx, e.Authorization, args)
	if err != nil {
		return nil, err
	}

	if args.ProjectKey != "" {
		if err := s.recordScope(ctx, e, models.PageContext{ProjectKey: args.ProjectKey}); err != nil {
			return nil, err
		}
	} else {
		if err := s.Store.RecordCatalogScope(ctx, e.User, e.Run.ID, args.Kind); err != nil {
			return nil, err
		}
	}

	value, err := safeCatalogJSON(raw)
	if err != nil {
		return nil, err
	}
	switch args.Kind {
	case "yaml_templates", "build_templates":
		return map[string]any{"result": value, "keyword": args.Keyword, "limit": 50, "note": "按完整模板名精确匹配当前页，total 是未过滤的模板总数；未匹配且 next_page 非零时继续查询，不能认定模板不存在；名称不明确或有歧义时请用户确认"}, nil
	case "projects", "repositories", "branches", "codehosts", "helm_templates", "clusters", "registries", "helm_repositories":
		return map[string]any{"result": value, "keyword": args.Keyword, "limit": 50, "note": "最多返回五十项；未找到目标或候选过多时，请提供更准确的名称继续搜索，不能据此认定资源不存在"}, nil
	}
	return value, nil
}

func (p *PlatformClient) catalog(ctx context.Context, auth string, args models.CatalogRequest) (json.RawMessage, error) {
	args.Keyword = strings.TrimSpace(args.Keyword)
	if !utf8.ValidString(args.Keyword) || len(args.Keyword) > 128 {
		return nil, fmt.Errorf("搜索关键词须为最多一百二十八字节的 UTF-8 文本")
	}
	if args.Page < 0 || args.Page > int(^uint(0)>>1)/50 {
		return nil, fmt.Errorf("模板列表页码超出有效范围")
	}
	page := args.Page
	if page == 0 {
		page = 1
	}
	query := url.Values{"page_num": {"1"}, "page_size": {"50"}}
	if args.ProjectKey != "" {
		if err := checkNames(args.ProjectKey); err != nil {
			return nil, err
		}

		query.Set("projectName", args.ProjectKey)
	}

	var path string
	switch args.Kind {
	case "projects":
		path = "/api/v1/picket/projects"
		query.Set("verbosity", "detailed")
		query.Set("ungrouped", "false")
		query.Set("filter", regexp.QuoteMeta(args.Keyword))
	case "codehosts":
		path = "/api/code/codehost"
	case "repositories", "branches", "configuration_paths":
		if err := p.requireGitSource(ctx, auth, args.CodehostID); err != nil {
			return nil, err
		}
		query.Set("repoOwner", args.RepoOwner)
		query.Set("repoName", args.RepoName)
		query.Set("branchName", args.Branch)
		query.Set("page", "1")
		query.Set("per_page", "50")
		query.Set("key", args.Keyword)
		path = "/api/code/codehost/" + strconv.Itoa(args.CodehostID) + "/projects"
		if args.Kind == "branches" {
			path = "/api/code/codehost/" + strconv.Itoa(args.CodehostID) + "/branches"
		}
		if args.Kind == "configuration_paths" {
			if err := checkConfigPath(args.Path); err != nil {
				return nil, err
			}

			path = "/api/code/workspace/tree"
			query = url.Values{
				"codehost_id": {strconv.Itoa(args.CodehostID)},
				"owner":       {args.RepoOwner},
				"repo":        {args.RepoName},
				"branch":      {args.Branch},
				"path":        {args.Path},
			}
		}
	case "yaml_templates":
		path = "/api/template/yaml"
		query.Set("page_num", strconv.Itoa(page))
	case "yaml_template":
		if err := checkNames(args.ID); err != nil {
			return nil, err
		}
		path = "/api/template/yaml/" + args.ID
	case "helm_templates":
		path = "/api/template/charts"
	case "helm_template":
		if err := checkNames(args.ID); err != nil {
			return nil, err
		}
		path = "/api/template/charts/" + args.ID
		query.Set("metadata_only", "true")
	case "build_templates":
		path = "/api/template/build"
		query.Set("page_num", strconv.Itoa(page))
	case "build_template":
		if err := checkNames(args.ID); err != nil {
			return nil, err
		}
		path = "/api/template/build/" + args.ID
	case "clusters":
		path = "/api/cluster/clusters"
	case "registries":
		path = "/api/system/registry/namespaces"
	case "helm_repositories":
		path = "/api/system/helm/public"
		if args.ProjectKey != "" {
			path = "/api/system/helm/project"
		}
	case "chart_versions":
		if args.ID == "" {
			args.ID = args.RepoName
		}
		if args.ID == "" {
			return nil, fmt.Errorf("查询 Chart 版本须通过 repo_name（或 id）提供 Chart 仓库名称，例如 test；不使用仓库内部 ID")
		}
		if err := checkNames(args.ID); err != nil {
			return nil, err
		}
		path = "/api/system/helm/" + args.ID + "/index"
	default:
		return nil, fmt.Errorf("不支持的查询目录")
	}

	raw, err := p.business(ctx, auth, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	if args.Kind == "codehosts" {
		// 模型只需要选择代码源，不传递原接口中的凭据和连接配置。
		var hosts []struct {
			ID        int    `json:"id"`
			Alias     string `json:"alias"`
			Type      string `json:"type"`
			Address   string `json:"address"`
			Namespace string `json:"namespace"`
		}
		if err := json.Unmarshal(raw, &hosts); err != nil {
			return nil, err
		}
		raw = common.JSON(hosts)
	}
	if args.Kind == "helm_repositories" {
		// 原列表可能包含凭据，只向模型提供选择仓库所需的标识和名称。
		var repositories []struct {
			ID       string `json:"id"`
			RepoName string `json:"repo_name"`
		}
		if err := json.Unmarshal(raw, &repositories); err != nil {
			return nil, err
		}
		raw = common.JSON(repositories)
	}
	if args.Kind == "helm_template" {
		var version struct {
			Digest string `json:"sha1"`
		}
		if json.Unmarshal(raw, &version) != nil || version.Digest == "" {
			return nil, fmt.Errorf("Chart 模板缺少内容摘要，请先在模板管理中更新模板")
		}
	}
	switch args.Kind {
	case "yaml_templates", "build_templates":
		return filterTemplatePage(raw, args.Kind, args.Keyword, page)
	case "codehosts", "helm_templates", "clusters", "registries", "helm_repositories":
		return filterCatalogNames(raw, args.Keyword)
	}
	return raw, nil
}

// 复用原列表分页，在 halman 内按完整名称匹配，不扩展 aslan 的搜索能力。
func filterTemplatePage(raw json.RawMessage, kind, name string, page int) (json.RawMessage, error) {
	type entry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var response struct {
		YamlTemplates  []entry `json:"yaml_template"`
		BuildTemplates []entry `json:"build_templates"`
		Total          int     `json:"total"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	items := response.YamlTemplates
	if kind == "build_templates" {
		items = response.BuildTemplates
	}
	if response.Total < 0 || len(items) == 0 && response.Total > (page-1)*50 {
		return nil, fmt.Errorf("模板列表分页结果不完整，请从第一页重新查询")
	}
	matches := []entry{}
	for _, item := range items {
		if name == "" || item.Name == name {
			matches = append(matches, item)
		}
	}
	nextPage := 0
	if page*50 < response.Total {
		nextPage = page + 1
	}
	return common.JSON(map[string]any{"items": matches, "total": response.Total, "page": page, "next_page": nextPage}), nil
}

// 这些目录的原接口返回完整列表，仅按资源标识和名称过滤，不搜索配置正文。
func filterCatalogNames(raw json.RawMessage, keyword string) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	keyword = strings.ToLower(keyword)
	filter := func(items []any) []any {
		result := []any{}
		for _, item := range items {
			matched := keyword == ""
			if name, ok := item.(string); ok {
				matched = strings.Contains(strings.ToLower(name), keyword)
			}
			if object, ok := item.(map[string]any); ok {
				for _, key := range []string{"name", "repo_name", "alias", "id", "namespace", "address", "reg_addr"} {
					if name, ok := object[key].(string); ok && strings.Contains(strings.ToLower(name), keyword) {
						matched = true
					}
				}
			}
			if matched {
				result = append(result, item)
				if len(result) == 50 {
					break
				}
			}
		}
		return result
	}
	switch typed := value.(type) {
	case []any:
		value = filter(typed)
	case map[string]any:
		found := false
		for _, key := range []string{"chartTemplates", "clusters", "registries", "namespaces", "codehosts", "items", "data", "list", "result"} {
			if items, ok := typed[key].([]any); ok {
				typed[key] = filter(items)
				found = true
			}
		}
		if !found {
			if keyword == "" {
				return raw, nil
			}
			return nil, fmt.Errorf("该目录的响应不支持名称搜索，请选择已有候选或提供资源标识")
		}
	default:
		return nil, fmt.Errorf("目录响应格式无效")
	}
	return common.JSON(value), nil
}

func (p *PlatformClient) requireGitSource(ctx context.Context, auth string, id int) error {
	_, err := p.gitSource(ctx, auth, id)
	return err
}

func (p *PlatformClient) gitSource(ctx context.Context, auth string, id int) (string, error) {
	if id < 1 {
		return "", fmt.Errorf("请选择已集成的 GitHub 或 GitLab 代码源")
	}

	raw, err := p.business(ctx, auth, http.MethodGet, "/api/code/codehost", nil, nil)
	if err != nil {
		return "", err
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}

	var find func(any) string
	find = func(v any) string {
		switch x := v.(type) {
		case []any:
			for _, entry := range x {
				if kind := find(entry); kind != "" {
					return kind
				}
			}
		case map[string]any:
			if n, ok := x["id"].(float64); ok && int(n) == id {
				kind, _ := x["type"].(string)
				if kind == "github" || kind == "gitlab" {
					return kind
				}

				return ""
			}
			for _, entry := range x {
				if kind := find(entry); kind != "" {
					return kind
				}
			}
		}

		return ""
	}
	kind := find(value)
	if kind == "" {
		return "", fmt.Errorf("当前只允许 API 读取型 GitHub/GitLab 来源，不支持 clone 或公开仓库链接")
	}

	return kind, nil
}
