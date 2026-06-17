package jobcontroller

import (
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestPrepareRuntimeNotificationFieldsSupportsPayloadDynamicRecipients(t *testing.T) {
	ctl := &NotificationJobCtl{
		workflowCtx: &commonmodels.WorkflowTaskCtx{
			WorkflowKeyVals: []*commonmodels.KeyVal{
				{Key: "payload.user.email", Value: "dev@example.com"},
			},
		},
		jobTaskSpec: &commonmodels.JobTaskNotificationSpec{
			WebHookType: setting.NotifyWebHookTypeMail,
			MailNotificationConfig: &commonmodels.MailNotificationConfig{
				DynamicRecipients: commonmodels.DynamicRecipients{
					"{{.payload.user.email}}",
				},
			},
		},
	}

	if err := ctl.prepareRuntimeNotificationFields(); err != nil {
		t.Fatalf("prepareRuntimeNotificationFields returned error: %v", err)
	}

	if len(ctl.jobTaskSpec.MailNotificationConfig.TargetUsers) != 1 {
		t.Fatalf("expected 1 target user, got %d", len(ctl.jobTaskSpec.MailNotificationConfig.TargetUsers))
	}

	got := ctl.jobTaskSpec.MailNotificationConfig.TargetUsers[0]
	if got == nil || got.Type != "email" || got.UserName != "dev@example.com" {
		t.Fatalf("expected resolved payload email target user, got %#v", got)
	}
}

func TestPrepareRuntimeNotificationFieldsSupportsPayloadStaticRecipients(t *testing.T) {
	ctl := &NotificationJobCtl{
		workflowCtx: &commonmodels.WorkflowTaskCtx{
			WorkflowKeyVals: []*commonmodels.KeyVal{
				{Key: "payload.reviewer.email", Value: "reviewer@example.com"},
			},
		},
		jobTaskSpec: &commonmodels.JobTaskNotificationSpec{
			WebHookType: setting.NotifyWebHookTypeMSTeam,
			MSTeamsNotificationConfig: &commonmodels.MSTeamsNotificationConfig{
				AtEmails: []string{"{{.payload.reviewer.email}}"},
			},
		},
	}

	if err := ctl.prepareRuntimeNotificationFields(); err != nil {
		t.Fatalf("prepareRuntimeNotificationFields returned error: %v", err)
	}

	if len(ctl.jobTaskSpec.MSTeamsNotificationConfig.AtEmails) != 1 {
		t.Fatalf("expected 1 rendered email, got %d", len(ctl.jobTaskSpec.MSTeamsNotificationConfig.AtEmails))
	}

	if got := ctl.jobTaskSpec.MSTeamsNotificationConfig.AtEmails[0]; got != "reviewer@example.com" {
		t.Fatalf("expected rendered payload email, got %q", got)
	}
}

func TestPrepareRuntimeNotificationFieldsDoesNotRenderPayloadInTitleOrContent(t *testing.T) {
	ctl := &NotificationJobCtl{
		workflowCtx: &commonmodels.WorkflowTaskCtx{
			WorkflowKeyVals: []*commonmodels.KeyVal{
				{Key: "payload.user.email", Value: "dev@example.com"},
				{Key: "workflow.trigger.branch", Value: "feature/demo"},
			},
		},
		jobTaskSpec: &commonmodels.JobTaskNotificationSpec{
			Title:   "branch={{.workflow.trigger.branch}} payload={{.payload.user.email}}",
			Content: "branch={{.workflow.trigger.branch}} payload={{.payload.user.email}}",
		},
	}

	if err := ctl.prepareRuntimeNotificationFields(); err != nil {
		t.Fatalf("prepareRuntimeNotificationFields returned error: %v", err)
	}

	want := "branch=feature/demo payload={{.payload.user.email}}"
	if ctl.jobTaskSpec.Title != want {
		t.Fatalf("expected title %q, got %q", want, ctl.jobTaskSpec.Title)
	}
	if ctl.jobTaskSpec.Content != want {
		t.Fatalf("expected content %q, got %q", want, ctl.jobTaskSpec.Content)
	}
}
