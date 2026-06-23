package instantmessage

import (
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestResolveWorkflowNotifyDynamicRecipientsSupportsPayloadEmail(t *testing.T) {
	task := &commonmodels.WorkflowTask{
		ProjectName:        "yaml",
		ProjectDisplayName: "yaml",
		TaskID:             350,
		StartTime:          time.Now().Unix(),
		WorkflowArgs: &commonmodels.WorkflowV4{
			HookPayload: &commonmodels.HookPayload{
				Branch:       "feature-1",
				TargetBranch: "feature-1",
				CommitID:     "14d4e3a44d3a02a2c3e48dfb4aec4d5fe91df31a",
				CommitSHA:    "14d4e3a44d3a02a2c3e48dfb4aec4d5fe91df31a",
				EventType:    "push",
				RawPayload: `{
					"head_commit": {
						"author": {
							"email": "huanghongbo@koderover.com"
						}
					}
				}`,
			},
		},
	}

	notify := &commonmodels.NotifyCtl{
		Enabled:     true,
		WebHookType: setting.NotifyWebHookTypeMail,
		NotifyTypes: []string{string(config.StatusCreated)},
		MailNotificationConfig: &commonmodels.MailNotificationConfig{
			DynamicRecipients: commonmodels.DynamicRecipients{"{{.payload.head_commit.author.email}}"},
		},
	}

	if err := resolveWorkflowNotifyDynamicRecipients(task, notify); err != nil {
		t.Fatalf("resolveWorkflowNotifyDynamicRecipients returned error: %v", err)
	}

	if got := len(notify.MailNotificationConfig.TargetUsers); got != 1 {
		t.Fatalf("expected 1 resolved mail target user, got %d", got)
	}

	target := notify.MailNotificationConfig.TargetUsers[0]
	if target == nil || target.Type != "email" || target.UserName != "huanghongbo@koderover.com" {
		t.Fatalf("unexpected resolved target: %#v", target)
	}
}
