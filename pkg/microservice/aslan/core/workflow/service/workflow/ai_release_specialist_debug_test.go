package workflow

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestValidateAIReleaseSpecialistDebugRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *DebugAIReleaseSpecialistPromptRequest
		wantErr bool
	}{
		{
			name: "task",
			req: &DebugAIReleaseSpecialistPromptRequest{
				WorkflowName: "wf",
				TaskID:       1,
				JobName:      "ai-check",
			},
		},
		{
			name: "missing job name",
			req: &DebugAIReleaseSpecialistPromptRequest{
				WorkflowName: "wf",
				TaskID:       1,
			},
			wantErr: true,
		},
		{
			name: "invalid task mode",
			req: &DebugAIReleaseSpecialistPromptRequest{
				WorkflowName: "wf",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAIReleaseSpecialistDebugRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateAIReleaseSpecialistDebugRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureWorkflowPermission(t *testing.T) {
	ctx := &internalhandler.Context{
		UserID: "user-1",
		Resources: &user.AuthorizedResources{
			ProjectAuthInfo: map[string]*user.ProjectActions{
				"demo": {
					Workflow: &user.WorkflowActions{
						View: true,
					},
				},
			},
		},
	}
	if err := ensureWorkflowPermission(ctx, "demo", "wf-demo"); err != nil {
		t.Fatalf("ensureWorkflowPermission returned error: %v", err)
	}
}
