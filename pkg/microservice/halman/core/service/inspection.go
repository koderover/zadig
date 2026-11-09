package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

type reportResult struct {
	Text string
	Err  error
	Time time.Time
}

// 同次运行复用相同报告及失败结果，不因模型重复调用再次消耗原模块额度。
func (s *AgentService) originalReport(ctx context.Context, e *execution, path string, query url.Values, body any) reportResult {
	key := path + "?" + query.Encode() + string(common.JSON(body))
	if e.Reports == nil {
		e.Reports = map[string]reportResult{}
	}
	if result, found := e.Reports[key]; found {
		return result
	}
	raw, err := s.Platform.business(ctx, e.Authorization, http.MethodPost, path, query, body)
	result := reportResult{Err: err, Time: time.Now().UTC()}
	if err == nil {
		var report struct {
			Result string `json:"result"`
			Answer string `json:"answer"`
		}
		if json.Unmarshal(raw, &report) != nil {
			result.Err = fmt.Errorf("原报告格式无效")
		} else {
			result.Text = report.Result
			if path == "/api/stat/v2/ai/analysis" {
				result.Text = report.Answer
			}
			if len(result.Text) > 64<<10 {
				result.Text = ""
				result.Err = fmt.Errorf("原报告过大，请缩小查询范围")
			}
		}
	}
	e.Reports[key] = result
	return result
}

func inspectionMissing(input models.EnvironmentInspectionInput) []string {
	missing := []string{}
	if input.ProjectKey == "" {
		missing = append(missing, "项目")
	}
	if input.EnvName == "" {
		missing = append(missing, "环境")
	}
	if input.Production == nil {
		missing = append(missing, "是否生产环境")
	}
	return missing
}

func inspectionTarget(input models.EnvironmentInspectionInput) models.PageContext {
	return models.PageContext{PageType: "environment", ProjectKey: input.ProjectKey, EnvName: input.EnvName, Production: *input.Production}
}

func inspectionQuery(target models.PageContext) url.Values {
	return url.Values{"projectName": {target.ProjectKey}, "production": {strconv.FormatBool(target.Production)}}
}

func (p *PlatformClient) authorizeEnvironment(ctx context.Context, authorization string, target models.PageContext) error {
	if err := checkNames(target.ProjectKey, target.EnvName); err != nil {
		return err
	}
	return p.authorizeContext(ctx, authorization, target)
}

func (s *AgentService) inspectEnvironment(ctx context.Context, input models.EnvironmentInspectionInput) (any, error) {
	e, err := s.skillExecution(ctx, "environment_inspection")
	if err != nil {
		return nil, err
	}
	input, err = resolveSkillInput(ctx, s, e, input)
	if err != nil {
		return nil, err
	}
	if missing := inspectionMissing(input); len(missing) > 0 {
		return s.saveSkillInput(ctx, e, input, missing)
	}
	target := inspectionTarget(input)
	if err := s.Platform.authorizeEnvironment(ctx, e.Authorization, target); err != nil {
		return nil, err
	}
	if err := s.recordScope(ctx, e, target); err != nil {
		return nil, err
	}
	if _, err := s.saveSkillInput(ctx, e, input, nil); err != nil {
		return nil, err
	}
	query := inspectionQuery(target)
	query.Set("triggerName", "ai-assistant")
	path := "/api/environment/environments/" + target.EnvName + "/analysis"
	result := s.originalReport(ctx, e, path, query, nil)
	if result.Err != nil {
		return nil, result.Err
	}
	if strings.TrimSpace(result.Text) == "" {
		return map[string]any{"status": "no_report", "target": target, "message": "原巡检未返回报告，可能未配置巡检项；不能据此认定环境健康"}, nil
	}
	value, err := safeJSON(common.JSON(map[string]any{"report": result.Text, "target": target, "collected_at": result.Time, "source": path, "original_model": true, "read_only": true}))
	if err != nil {
		return nil, err
	}
	e.EvidenceRead.Store(true)
	e.Outcome = ""
	return value, nil
}

func (s *AgentService) environmentInspectionHistory(ctx context.Context, input models.EnvironmentHistoryInput) (any, error) {
	e, err := s.skillExecution(ctx, "environment_inspection")
	if err != nil {
		return nil, err
	}
	input, err = resolveSkillInput(ctx, s, e, input)
	if err != nil {
		return nil, err
	}
	environment := models.EnvironmentInspectionInput{ProjectKey: input.ProjectKey, EnvName: input.EnvName, Production: input.Production}
	if missing := inspectionMissing(environment); len(missing) > 0 {
		return s.saveSkillInput(ctx, e, input, missing)
	}
	target := inspectionTarget(environment)
	if input.Page == 0 {
		input.Page = 1
	}
	if input.PageSize == 0 {
		input.PageSize = 5
	}
	if input.Page < 1 || input.Page > 10000 || input.PageSize < 1 || input.PageSize > 20 {
		return nil, common.ErrInvalidRequest
	}
	if err := s.Platform.authorizeEnvironment(ctx, e.Authorization, target); err != nil {
		return nil, err
	}
	if _, err := s.saveSkillInput(ctx, e, input, nil); err != nil {
		return nil, err
	}
	query := inspectionQuery(target)
	query.Set("envName", target.EnvName)
	query.Set("pageNum", strconv.Itoa(input.Page))
	query.Set("pageSize", strconv.Itoa(input.PageSize))
	path := "/api/environment/environments/analysis/history"
	raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	var history struct {
		Total  int64 `json:"total"`
		Result []struct {
			Project    string `json:"project_name"`
			Env        string `json:"env_name"`
			Production bool   `json:"production"`
			Status     string `json:"status"`
			Report     string `json:"result"`
			Start      int64  `json:"start_time"`
			End        int64  `json:"end_time"`
			Source     string `json:"trigger_name"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &history) != nil || len(history.Result) > input.PageSize {
		return nil, fmt.Errorf("巡检历史格式或数量无效")
	}
	valid := false
	for _, item := range history.Result {
		if item.Project != target.ProjectKey || item.Env != target.EnvName || item.Production != target.Production {
			return nil, fmt.Errorf("巡检历史范围与目标环境不一致")
		}
		if item.Status == "success" && strings.TrimSpace(item.Report) != "" {
			valid = true
		}
	}
	value, err := safeJSON(common.JSON(map[string]any{"target": target, "history": true, "source": path, "total": history.Total, "reports": history.Result}))
	if err != nil {
		return nil, err
	}
	if err := s.recordScope(ctx, e, target); err != nil {
		return nil, err
	}
	if valid {
		e.EvidenceRead.Store(true)
		e.Outcome = ""
	}
	return value, nil
}

func (s *AgentService) environmentServiceEvidence(ctx context.Context, target models.ServiceTarget) (any, error) {
	if e, ok := ctx.Value(executionKey{}).(*execution); ok && e.SkillID == "" {
		return nil, fmt.Errorf("请先调用环境巡检或巡检历史工具；仅查看服务日志请调用 service_diagnosis，goal=diagnose")
	}
	e, err := s.skillExecution(ctx, "environment_inspection")
	if err != nil {
		return nil, err
	}
	page := e.Run.Context
	if page.EnvName == "" || page.ProjectKey == "" {
		return nil, fmt.Errorf("请先确定本次巡检环境")
	}
	if target.ProjectKey == "" {
		target.ProjectKey = page.ProjectKey
	}
	if target.EnvName == "" {
		target.EnvName = page.EnvName
	}
	if target.ProjectKey != page.ProjectKey || target.EnvName != page.EnvName {
		return nil, fmt.Errorf("补充证据只能读取本次巡检环境")
	}
	target.Production = page.Production
	if e.InspectedServices == nil {
		e.InspectedServices = map[string]bool{}
	}
	if !e.InspectedServices[target.ServiceName] && len(e.InspectedServices) >= 3 {
		return nil, fmt.Errorf("每轮最多补充三个服务，请缩小范围")
	}
	if err := checkNames(target.ServiceName); err != nil {
		return nil, err
	}
	e.InspectedServices[target.ServiceName] = true
	value, err := s.collectServiceEvidence(ctx, e, target)
	if err != nil {
		return nil, err
	}
	scope := models.ResourceScope{PageContext: models.PageContext{PageType: "service", ProjectKey: target.ProjectKey, EnvName: target.EnvName, ServiceName: target.ServiceName, Production: target.Production}}
	if err := s.Store.RecordResourceScope(ctx, e.User, e.Run.ID, scope); err != nil {
		return nil, err
	}
	return value, nil
}
