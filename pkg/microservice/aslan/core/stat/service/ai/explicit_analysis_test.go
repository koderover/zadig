package ai

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestExplicitAnalysisDoesNotExpandProjectsOrOverrideTime(t *testing.T) {
	now := time.Now().Unix()
	args := &AiAnalysisReq{Prompt: "分析质量和效率", ProjectList: []string{"展示名", "one"}, StartTime: now - 30*86400, EndTime: now}
	if err := ValidateExplicitAnalysisArgs(args); err != nil {
		t.Fatal(err)
	}
	projects, err := resolveAnalysisProjects(args.ProjectList, map[string]string{"one": "展示名", "two": "另一个项目"})
	if err != nil || !reflect.DeepEqual(projects, []string{"one"}) || args.StartTime != now-30*86400 || args.EndTime != now {
		t.Fatalf("显式范围必须保持且不扩展: %v %v", projects, err)
	}
	if _, err := resolveAnalysisProjects([]string{"重复名称"}, map[string]string{"a": "重复名称", "b": "重复名称"}); err == nil || !strings.Contains(err.Error(), "a、b") {
		t.Fatal("歧义必须返回候选，不能自动选择或扩展")
	}
	if _, err := resolveAnalysisProjects([]string{"missing"}, map[string]string{"one": "展示名"}); err == nil {
		t.Fatal("未知项目不能替换成全部项目")
	}
	if err := ValidateExplicitAnalysisArgs(&AiAnalysisReq{Prompt: "原有自由提问"}); err != nil {
		t.Fatalf("仅提供问题时应保留旧解析流程: %v", err)
	}
	for _, invalid := range []*AiAnalysisReq{
		{Prompt: "问题", StartTime: now - 1, EndTime: now},
		{Prompt: "问题", ProjectList: []string{"one"}},
		{Prompt: "问题", ProjectList: []string{""}, StartTime: now - 1, EndTime: now},
		{Prompt: "问题", ProjectList: []string{"one"}, StartTime: now, EndTime: now + 1},
	} {
		if err := ValidateExplicitAnalysisArgs(invalid); err == nil {
			t.Fatalf("不完整范围不能回退为自由提问: %+v", invalid)
		}
	}
}
