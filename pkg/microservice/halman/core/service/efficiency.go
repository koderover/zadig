package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

const efficiencyInsightAction = "efficiency_over"

func (p *PlatformClient) authorizeScope(ctx context.Context, authorization string, scope models.ResourceScope) error {
	return p.authorizeScopes(ctx, authorization, []models.ResourceScope{scope})
}

func (p *PlatformClient) authorizeEfficiency(ctx context.Context, authorization string) error {
	return p.authorizeScope(ctx, authorization, models.ResourceScope{SystemAction: efficiencyInsightAction})
}

func validateEfficiencyInput(input *models.EfficiencyDiagnosisInput, now time.Time) error {
	if len(input.Projects) == 0 || len(input.Projects) > 10 {
		return fmt.Errorf("请明确指定一到十个项目，不支持自动扫描全部项目")
	}
	for i, name := range input.Projects {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 256 {
			return common.ErrInvalidRequest
		}
		input.Projects[i] = name
	}
	if input.StartTime == 0 && input.EndTime == 0 {
		input.EndTime = now.Unix()
		input.StartTime = now.AddDate(0, 0, -30).Unix()
	}
	if input.StartTime <= 0 || input.EndTime <= input.StartTime || input.EndTime > now.Unix() || input.EndTime-input.StartTime > 90*86400 {
		return fmt.Errorf("请选择不包含未来时间、最多九十天的完整时间范围")
	}
	if input.Question == "" {
		input.Question = "分析指定项目的质量和效率，找出瓶颈并给出可操作的优化建议"
	}
	if len(input.Question) > 4000 {
		return fmt.Errorf("分析问题最多四千字节")
	}
	return nil
}

func (s *AgentService) diagnoseEfficiency(ctx context.Context, input models.EfficiencyDiagnosisInput) (any, error) {
	e, err := s.skillExecution(ctx, "efficiency_diagnosis")
	if err != nil {
		return nil, err
	}
	if err := s.Platform.authorizeEfficiency(ctx, e.Authorization); err != nil {
		return nil, err
	}
	if err := s.Store.RecordSystemScope(ctx, e.User, e.Run.ID, efficiencyInsightAction); err != nil {
		return nil, err
	}
	e.Run.Context = models.PageContext{PageType: "efficiency"}
	input, err = resolveSkillInput(ctx, s, e, input)
	if err != nil {
		return nil, err
	}
	if input.StartTime == 0 && input.EndTime == 0 {
		now := e.Run.CreatedAt
		if now.IsZero() {
			now = time.Now()
		}
		input.StartTime = now.AddDate(0, 0, -30).Unix()
		input.EndTime = now.Unix()
	}
	missing := []string{}
	if len(input.Projects) == 0 {
		missing = append(missing, "需要分析的项目名称或标识")
	}
	if input.StartTime == 0 {
		missing = append(missing, "分析开始时间")
	}
	if input.EndTime == 0 {
		missing = append(missing, "分析结束时间")
	}
	if len(missing) > 0 {
		return s.saveSkillInput(ctx, e, input, missing)
	}
	if err := validateEfficiencyInput(&input, time.Now()); err != nil {
		return nil, err
	}
	if _, err := s.saveSkillInput(ctx, e, input, nil); err != nil {
		return nil, err
	}
	path := "/api/stat/v2/ai/analysis"
	result := s.originalReport(ctx, e, path, nil, map[string]any{"prompt": input.Question, "project_list": input.Projects, "start_time": input.StartTime, "end_time": input.EndTime})
	if result.Err != nil {
		return nil, result.Err
	}
	if strings.TrimSpace(result.Text) == "" {
		return nil, fmt.Errorf("原效能分析未返回有效报告")
	}
	value, err := safeJSON(common.JSON(map[string]any{"report": result.Text, "requested_projects": input.Projects, "start_time": input.StartTime, "end_time": input.EndTime, "collected_at": result.Time, "source": path, "original_model": true, "read_only": true}))
	if err != nil {
		return nil, err
	}
	e.EvidenceRead.Store(true)
	e.Outcome = ""
	return value, nil
}
