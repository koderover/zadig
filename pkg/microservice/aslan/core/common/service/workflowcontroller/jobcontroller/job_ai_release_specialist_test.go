package jobcontroller

import (
	"testing"
	"time"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestNormalizeAIResultValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "pass", input: "pass", want: "pass"},
		{name: "warning", input: "warning", want: "warning"},
		{name: "fail", input: "fail", want: "fail"},
		{name: "failed alias", input: "FAILED", want: "fail"},
		{name: "warn alias", input: "warn", want: "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAIResultValue(tt.input); got != tt.want {
				t.Fatalf("normalizeAIResultValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseAIReleaseSpecialistResultPreservesFailConclusion(t *testing.T) {
	answer := `{"conclusion":"fail","summary":"存在高风险变更","checks":[{"name":"变更检查","result":"fail","evidence":"涉及生产环境核心服务","suggestion":"停止发布"}]}`

	result, err := ParseAIReleaseSpecialistResult(answer)
	if err != nil {
		t.Fatalf("ParseAIReleaseSpecialistResult returned error: %v", err)
	}
	if result.Conclusion != "fail" {
		t.Fatalf("result.Conclusion = %q, want fail", result.Conclusion)
	}
	if result.Checks[0].Result != "fail" {
		t.Fatalf("result.Checks[0].Result = %q, want fail", result.Checks[0].Result)
	}
}

func TestBuildAIReleaseSpecialistPromptForDebug(t *testing.T) {
	input := &commonmodels.AIReleaseSpecialistInput{
		ChangeSummary: &commonmodels.AIChangeSummary{
			Remark: "本次发布包含 2 个服务变更",
		},
	}

	result, err := BuildAIReleaseSpecialistPromptForDebug("请重点关注生产风险。", "你是一个谨慎的发布助手。", input)
	if err != nil {
		t.Fatalf("BuildAIReleaseSpecialistPromptForDebug returned error: %v", err)
	}
	if result.SystemPrompt == "" {
		t.Fatal("result.SystemPrompt is empty")
	}
	if result.Prompt == "" {
		t.Fatal("result.Prompt is empty")
	}
	if result.PromptTokens <= 0 {
		t.Fatalf("result.PromptTokens = %d, want > 0", result.PromptTokens)
	}
}

func TestGetRemainingTimeout(t *testing.T) {
	ctl := &AIReleaseSpecialistJobCtl{
		jobTaskSpec: &commonmodels.JobTaskAIReleaseSpecialistSpec{
			Timeout: 10,
		},
	}
	remaining := ctl.getRemainingTimeout(time.Now().Add(-2 * time.Minute))
	if remaining > 8 || remaining < 7 {
		t.Fatalf("remaining timeout = %d, want around 8", remaining)
	}
}
