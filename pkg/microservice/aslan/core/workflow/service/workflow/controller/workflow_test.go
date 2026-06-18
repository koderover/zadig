package controller

import (
	"math"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestBuildRuntimeReferableVariablesIncludesTriggerRuntimeVariables(t *testing.T) {
	workflow := &commonmodels.WorkflowV4{
		Name:        "workflow-demo",
		DisplayName: "Workflow Demo",
	}

	variables := buildRuntimeReferableVariables(workflow)
	keySet := make(map[string]struct{}, len(variables))
	for _, variable := range variables {
		keySet[variable.Key] = struct{}{}
	}

	expectedKeys := []string{
		"workflow.task.creator",
		"workflow.trigger.branch",
		"workflow.trigger.target_branch",
		"workflow.trigger.pr",
		"workflow.trigger.commit_id",
		"workflow.trigger.commit_sha",
		"workflow.trigger.commit_message",
		"workflow.trigger.committer",
		"workflow.trigger.event",
	}

	for _, key := range expectedKeys {
		if _, ok := keySet[key]; !ok {
			t.Fatalf("expected runtime variable %s to be exposed", key)
		}
	}
}

func TestRenderJobTaskPreservesNotificationDynamicRecipients(t *testing.T) {
	task := &commonmodels.JobTask{
		JobType: string(config.JobNotification),
		Spec: &commonmodels.JobTaskNotificationSpec{
			WebHookType: setting.NotifyWebHookTypeMSTeam,
			Title:       "notify {{.workflow.trigger.branch}}",
			Content:     "reviewer {{.workflow.params.reviewer}}",
			MSTeamsNotificationConfig: &commonmodels.MSTeamsNotificationConfig{
				AtEmails: []string{"{{.workflow.params.reviewer}}"},
				DynamicRecipients: commonmodels.DynamicRecipients{
					"{{.payload.commits.0.author.email}}",
				},
			},
		},
	}

	err := RenderJobTaskWithGlobalVariables(task, map[string]string{
		"workflow.trigger.branch":        "feature/demo",
		"workflow.params.reviewer":       "reviewer@example.com",
		"payload.commits.0.author.email": "dev@example.com",
	})
	if err != nil {
		t.Fatalf("RenderJobTaskWithGlobalVariables returned error: %v", err)
	}

	spec, ok := task.Spec.(*commonmodels.JobTaskNotificationSpec)
	if !ok {
		t.Fatalf("expected notification spec type, got %T", task.Spec)
	}

	if spec.Title != "notify feature/demo" {
		t.Fatalf("expected notification title to be rendered, got %q", spec.Title)
	}

	if spec.Content != "reviewer reviewer@example.com" {
		t.Fatalf("expected notification content to be rendered, got %q", spec.Content)
	}

	if len(spec.MSTeamsNotificationConfig.AtEmails) != 1 {
		t.Fatalf("expected 1 rendered static recipient, got %d", len(spec.MSTeamsNotificationConfig.AtEmails))
	}

	if got := spec.MSTeamsNotificationConfig.AtEmails[0]; got != "reviewer@example.com" {
		t.Fatalf("expected rendered static recipient, got %q", got)
	}

	if len(spec.MSTeamsNotificationConfig.DynamicRecipients) != 1 {
		t.Fatalf("expected 1 dynamic recipient, got %d", len(spec.MSTeamsNotificationConfig.DynamicRecipients))
	}

	if got := spec.MSTeamsNotificationConfig.DynamicRecipients[0]; got != "{{.payload.commits.0.author.email}}" {
		t.Fatalf("expected dynamic recipient template to be preserved, got %q", got)
	}
}

func TestRestoreWorkflowNotificationRuntimeRenderFieldsRestoresOnlyDynamicRecipients(t *testing.T) {
	spec := &commonmodels.NotificationJobSpec{
		WebHookType: setting.NotifyWebHookTypeMSTeam,
		Title:       "notify {{.workflow.trigger.branch}}",
		Content:     "reviewer {{.workflow.params.reviewer}}",
		MSTeamsNotificationConfig: &commonmodels.MSTeamsNotificationConfig{
			AtEmails: []string{"{{.workflow.params.reviewer}}"},
			DynamicRecipients: commonmodels.DynamicRecipients{
				"{{.payload.commits.0.author.email}}",
			},
		},
	}
	workflow := &commonmodels.WorkflowV4{
		Stages: []*commonmodels.WorkflowStage{
			{
				Jobs: []*commonmodels.Job{
					{
						Name:    "notify",
						JobType: config.JobNotification,
						Spec:    spec,
					},
				},
			},
		},
	}

	backups, err := backupWorkflowNotificationRuntimeRenderFields(workflow)
	if err != nil {
		t.Fatalf("backupWorkflowNotificationRuntimeRenderFields returned error: %v", err)
	}

	spec.Title = "notify feature/demo"
	spec.Content = "reviewer reviewer@example.com"
	spec.MSTeamsNotificationConfig.AtEmails = []string{"reviewer@example.com"}
	spec.MSTeamsNotificationConfig.DynamicRecipients = commonmodels.DynamicRecipients{"dev@example.com"}

	if err := restoreWorkflowNotificationRuntimeRenderFields(workflow, backups); err != nil {
		t.Fatalf("restoreWorkflowNotificationRuntimeRenderFields returned error: %v", err)
	}

	restoredSpec, ok := workflow.Stages[0].Jobs[0].Spec.(*commonmodels.NotificationJobSpec)
	if !ok {
		t.Fatalf("expected notification spec type, got %T", workflow.Stages[0].Jobs[0].Spec)
	}

	if restoredSpec.Title != "notify feature/demo" {
		t.Fatalf("expected title to stay rendered, got %q", restoredSpec.Title)
	}
	if restoredSpec.Content != "reviewer reviewer@example.com" {
		t.Fatalf("expected content to stay rendered, got %q", restoredSpec.Content)
	}
	if len(restoredSpec.MSTeamsNotificationConfig.AtEmails) != 1 || restoredSpec.MSTeamsNotificationConfig.AtEmails[0] != "reviewer@example.com" {
		t.Fatalf("expected static recipients to stay rendered, got %#v", restoredSpec.MSTeamsNotificationConfig.AtEmails)
	}
	if len(restoredSpec.MSTeamsNotificationConfig.DynamicRecipients) != 1 || restoredSpec.MSTeamsNotificationConfig.DynamicRecipients[0] != "{{.payload.commits.0.author.email}}" {
		t.Fatalf("expected dynamic recipients template to be restored, got %#v", restoredSpec.MSTeamsNotificationConfig.DynamicRecipients)
	}
}

func TestRenderJobTaskWithGlobalVariablesReturnsMarshalError(t *testing.T) {
	task := &commonmodels.JobTask{
		Name:    "notify-task",
		JobType: string(config.JobFreestyle),
		Spec: map[string]interface{}{
			"invalid": math.NaN(),
		},
	}

	err := RenderJobTaskWithGlobalVariables(task, map[string]string{
		"workflow.trigger.branch": "feature/demo",
	})
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to marshal task notify-task") {
		t.Fatalf("expected marshal error message, got %v", err)
	}
}
