package instantmessage

import (
	"testing"

	"github.com/stretchr/testify/require"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestIsWorkflowHookEventEnabled(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		hookSetting *commonmodels.WorkflowHookSettings
		hookEvent   commonmodels.WorkflowHookEvent
		expected    bool
	}{
		{
			name:        "nil setting",
			hookSetting: nil,
			hookEvent:   commonmodels.WorkflowHookEventStartExecute,
			expected:    false,
		},
		{
			name: "disabled setting",
			hookSetting: &commonmodels.WorkflowHookSettings{
				Enable:     false,
				HookEvents: []commonmodels.WorkflowHookEvent{commonmodels.WorkflowHookEventStartExecute},
			},
			hookEvent: commonmodels.WorkflowHookEventStartExecute,
			expected:  false,
		},
		{
			name: "event not configured",
			hookSetting: &commonmodels.WorkflowHookSettings{
				Enable:     true,
				HookEvents: []commonmodels.WorkflowHookEvent{commonmodels.WorkflowHookEventStartExecute},
			},
			hookEvent: commonmodels.WorkflowHookEventCompleteExecute,
			expected:  false,
		},
		{
			name: "start event configured",
			hookSetting: &commonmodels.WorkflowHookSettings{
				Enable: true,
				HookEvents: []commonmodels.WorkflowHookEvent{
					commonmodels.WorkflowHookEventStartExecute,
					commonmodels.WorkflowHookEventCompleteExecute,
				},
			},
			hookEvent: commonmodels.WorkflowHookEventStartExecute,
			expected:  true,
		},
		{
			name: "complete event configured",
			hookSetting: &commonmodels.WorkflowHookSettings{
				Enable:     true,
				HookEvents: []commonmodels.WorkflowHookEvent{commonmodels.WorkflowHookEventCompleteExecute},
			},
			hookEvent: commonmodels.WorkflowHookEventCompleteExecute,
			expected:  true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.expected, isWorkflowHookEventEnabled(testCase.hookSetting, testCase.hookEvent))
		})
	}
}
