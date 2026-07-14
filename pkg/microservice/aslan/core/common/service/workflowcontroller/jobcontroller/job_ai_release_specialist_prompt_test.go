package jobcontroller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEffectiveAIReleaseSpecialistSystemPromptSuppressesMissingContextBoilerplate(t *testing.T) {
	prompt := GetEffectiveAIReleaseSpecialistSystemPrompt("自定义系统提示词")

	require.Contains(t, prompt, "summary 只写基于实际提供上下文的判断")
	require.Contains(t, prompt, "不要输出“本次输入未提供”这类缺失上下文清单")
	require.Contains(t, prompt, "checks[].evidence 必须逐项列出每个 env_name、service_name 的 pod_count 和 ready_pods")
}
