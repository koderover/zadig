package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

const analysisTimeLayout = "2006-01-02 15:04:05"

func parseAnalysisPrompt(question string, client llm.ILLM, projects []string, now time.Time) (*UserPromptParseInput, error) {
	now = now.In(time.Local)
	questionJSON, err := json.Marshal(question)
	if err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf(ParseUserPromptPrompt, now.Format(analysisTimeLayout), now.Location().String(), now.Format("-07:00"), questionJSON)
	model := client.GetModel()
	if model == "" {
		model = AnalysisModel
	}
	response, err := client.GetCompletion(context.TODO(), prompt, llm.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("解析分析范围失败: %w", err)
	}
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSpace(strings.TrimSuffix(response, "```"))
	}
	var parsed struct {
		ProjectList []string `json:"project_list"`
		JobList     []string `json:"job_list"`
		StartTime   string   `json:"start_time"`
		EndTime     string   `json:"end_time"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, fmt.Errorf("分析范围解析结果格式无效，请明确项目和分析时间范围")
	}
	if parsed.StartTime == "" || parsed.EndTime == "" {
		return nil, fmt.Errorf("请明确分析时间范围")
	}
	start, startErr := time.ParseInLocation(analysisTimeLayout, parsed.StartTime, time.Local)
	end, endErr := time.ParseInLocation(analysisTimeLayout, parsed.EndTime, time.Local)
	if startErr != nil || endErr != nil || start.Format(analysisTimeLayout) != parsed.StartTime || end.Format(analysisTimeLayout) != parsed.EndTime {
		return nil, fmt.Errorf("分析时间格式无效，请明确分析时间范围（YYYY-MM-DD HH:mm:ss）")
	}
	input := &UserPromptParseInput{ProjectList: parsed.ProjectList, JobList: parsed.JobList, StartTime: start.Unix(), EndTime: min(end.Unix(), now.Unix())}
	if input.StartTime <= 0 || input.EndTime <= input.StartTime {
		return nil, fmt.Errorf("分析时间范围无效，起始时间必须早于结束时间且不能位于未来")
	}
	if err := checkInputData(input, []string{"build", "test", "deploy", "release"}, projects); err != nil {
		return nil, fmt.Errorf("分析范围无效: %w", err)
	}
	return input, nil
}
