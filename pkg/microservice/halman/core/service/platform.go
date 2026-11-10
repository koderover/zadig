package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/tool/language"
)

var resourceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

var sensitiveName = regexp.MustCompile(`(?i)(password|passwd|secret|token|private.?key|api.?key|access.?key|authorization|credential)`)

var sensitiveText = regexp.MustCompile(`(?i)((?:bearer\s+)|(?:(?:password|passwd|token|secret|api[_-]?key)\s*[=:]\s*["']?))[^\s"',;]+`)

var sensitiveURL = regexp.MustCompile(`(?i)(https?://)[^\s/@]+:[^\s/@]+@`)

type PlatformError struct {
	Status    int
	Uncertain bool
	Message   string
}

func (e *PlatformError) Error() string {
	return e.UserMessage("")
}

func (e *PlatformError) UserMessage(locale string) string {
	lang := language.Normalize(locale)
	if e.Uncertain {
		message := lang.Text("Zadig 操作结果待确认，不可直接重试", "The Zadig operation outcome is unknown; do not retry directly")
		if e.Message != "" {
			message += ": " + e.Message
		}
		return message
	}
	if e.Message != "" && e.Status != http.StatusUnauthorized && e.Status != http.StatusForbidden {
		return e.Message
	}

	if e.Status == http.StatusForbidden || e.Status == http.StatusUnauthorized {
		return lang.Text("当前用户没有资源权限或登录态已失效", "You do not have permission to access this resource, or your login has expired")
	}

	if e.Status == http.StatusNotFound {
		return lang.Text("指定的 Zadig 资源不存在", "The specified Zadig resource does not exist")
	}

	return fmt.Sprintf(lang.Text("Zadig 接口请求失败（状态 %d）", "Zadig API request failed (status %d)"), e.Status)
}

// business 仅供固定业务适配器调用，不注册成模型可选择的 HTTP 工具。
func (p *PlatformClient) business(ctx context.Context, authorization, method, path string, query url.Values, body any) (json.RawMessage, error) {
	if err := p.checkUserAuthorization(ctx, authorization); err != nil {
		return nil, &PlatformError{Status: http.StatusUnauthorized}
	}
	if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/openapi/") {
		return nil, common.ErrInvalidRequest
	}
	if strings.HasPrefix(path, "/api/v1/authorization/") {
		if method != http.MethodGet || body != nil {
			return nil, common.ErrInvalidRequest
		}
		switch path {
		case "/api/v1/authorization/auth-info", "/api/v1/authorization/collaboration-action", "/api/v1/authorization/collaboration-permission":
		default:
			return nil, common.ErrInvalidRequest
		}
		if p.Users == nil {
			return nil, fmt.Errorf("用户权限客户端未配置")
		}
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		request := p.Users.R().SetContext(requestCtx).SetHeader("Authorization", authorization).SetQueryParamsFromValues(query).SetDoNotParseResponse(true)
		if current, ok := ctx.Value(executionKey{}).(*execution); ok {
			request.SetHeader("X-Request-ID", current.Run.ID)
		}
		response, err := request.Get(strings.TrimPrefix(path, "/api/v1"))
		if response == nil {
			return nil, &PlatformError{}
		}
		if err != nil {
			if response.RawResponse != nil && response.RawResponse.Body != nil {
				response.RawResponse.Body.Close()
			}
			return nil, &PlatformError{Status: response.StatusCode()}
		}
		return readBusinessResponse(response.RawResponse, false, path)
	}

	cache := phaseReads(ctx, method, path)
	if body != nil {
		cache = nil
	}
	key := readKey{authorization, path, query.Encode()}
	if cache != nil {
		if raw, found := cache.responses[key]; found {
			return raw, nil
		}
	}
	var data []byte
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		data = raw
	}

	u := config.AslanServiceAddress() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("业务请求参数无效")
	}

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	if e, ok := ctx.Value(executionKey{}).(*execution); ok {
		req.Header.Set("X-Request-ID", e.Run.ID)
	}

	if p.HTTP == nil {
		return nil, fmt.Errorf("平台 HTTP 客户端未配置")
	}
	client := *p.HTTP
	report := isOriginalReportRequest(method, path)
	if report {
		client.Timeout = 3 * time.Minute
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		if report {
			return nil, &PlatformError{Message: "原报告调用失败或等待超时，原模块可能仍在处理；请稍后查看原功能结果，本轮不自动重试"}
		}
		return nil, &PlatformError{Uncertain: method != http.MethodGet && !report}
	}
	raw, err := readBusinessResponse(resp, method != http.MethodGet && !report, path)
	if err == nil && cache != nil && len(raw) <= (1<<20)-cache.bytes {
		cache.responses[key] = raw
		cache.bytes += len(raw)
	}
	return raw, err
}

func isOriginalReportRequest(method, path string) bool {
	return method == http.MethodPost && (path == "/api/stat/v2/ai/analysis" ||
		(strings.HasPrefix(path, "/api/environment/environments/") && strings.HasSuffix(path, "/analysis")))
}

func readBusinessResponse(resp *http.Response, write bool, path string) (json.RawMessage, error) {
	if resp == nil || resp.Body == nil {
		return nil, &PlatformError{Uncertain: write}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var reply struct {
			Code        int    `json:"code"`
			Description string `json:"description"`
		}
		message := ""
		if json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&reply) == nil && len(reply.Description) <= 2000 {
			message = redactText(reply.Description)
		}
		if resp.StatusCode == http.StatusBadRequest && isOriginalReportRequest(http.MethodPost, path) {
			if reply.Code == 400 && message != "" {
				return nil, &PlatformError{Status: 400, Message: message}
			}
		}
		return nil, &PlatformError{
			Status:  resp.StatusCode,
			Message: message,
			Uncertain: write && resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden &&
				resp.StatusCode != http.StatusNotFound,
		}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return nil, &PlatformError{Uncertain: write}
	}

	if len(data) == 0 {
		return json.RawMessage(`{}`), nil
	}

	if !json.Valid(data) {
		if !write && strings.HasPrefix(path, "/api/code/workspace/getcontents/") {
			return common.JSON(string(data)), nil
		}

		return nil, &PlatformError{Uncertain: write}
	}

	var envelope struct {
		Code        int    `json:"code"`
		Description string `json:"description"`
	}
	if json.Unmarshal(data, &envelope) == nil && envelope.Code != 0 && envelope.Code != 200 {
		message := ""
		if len(envelope.Description) <= 2000 {
			message = redactText(envelope.Description)
		}
		return nil, &PlatformError{Status: 400, Uncertain: write, Message: message}
	}

	return json.RawMessage(data), nil
}

func redactText(text string) string {
	return sensitiveURL.ReplaceAllString(sensitiveText.ReplaceAllString(text, "${1}[已隐藏]"), "${1}[已隐藏]@")
}

func sanitized(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		secretValue := false
		for _, key := range []string{"key", "name"} {
			if name, ok := v[key].(string); ok && sensitiveName.MatchString(name) {
				secretValue = true
			}
		}
		for key, item := range v {
			if sensitiveName.MatchString(key) || (secretValue && (key == "value" || key == "default")) {
				result[key] = "[已隐藏]"
			} else {
				result[key] = sanitized(item)
			}
		}
		return result
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			result = append(result, sanitized(item))
		}
		return result
	case string:
		return redactText(v)
	default:
		return value
	}
}

func safeJSON(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}

	return safeJSONValue(value)
}

func safeJSONValue(value any) (any, error) {
	value = sanitized(value)
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	if len(data) > 64<<10 {
		return nil, fmt.Errorf("查询结果过大，请缩小查询范围")
	}

	return value, nil
}

func safeCatalogJSON(raw json.RawMessage) (any, error) {
	return filterEvidenceJSON(raw, 0, func(key string) bool {
		lower := strings.ToLower(key)
		return strings.Contains(lower, "yaml") || strings.Contains(lower, "script") ||
			lower == "content" || lower == "dockerfile" || lower == "build_steps"
	}, nil)
}

// 目录和诊断共用遍历，分别保留字段过滤规则及数组读取上限。
func filterEvidenceJSON(raw json.RawMessage, limit int, exclude func(string) bool, leaf func(any) any) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	var trim func(any) any
	trim = func(value any) any {
		switch v := value.(type) {
		case map[string]any:
			result := make(map[string]any)
			for key, item := range v {
				if !exclude(key) {
					result[key] = trim(item)
				}
			}
			return result
		case []any:
			if limit > 0 && len(v) > limit {
				v = v[:limit]
			}
			result := make([]any, 0, len(v))
			for _, item := range v {
				result = append(result, trim(item))
			}
			return result
		default:
			if leaf != nil {
				return leaf(value)
			}
			return value
		}
	}
	return safeJSONValue(trim(value))
}

func checkNames(values ...string) error {
	for _, value := range values {
		if !resourceName.MatchString(value) {
			return fmt.Errorf("项目、环境或资源标识不合法")
		}
	}

	return nil
}
