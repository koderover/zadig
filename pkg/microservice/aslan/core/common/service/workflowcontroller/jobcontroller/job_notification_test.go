package jobcontroller

import (
	"testing"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestSendJobNotificationsSendsWhenStatusMatches(t *testing.T) {
	origSendTaskNotifications := sendTaskNotifications
	defer func() {
		sendTaskNotifications = origSendTaskNotifications
	}()

	var (
		calls     int
		lastInput *instantmessage.TaskNotifyInput
	)
	sendTaskNotifications = func(input *instantmessage.TaskNotifyInput) error {
		calls++
		lastInput = input
		return nil
	}

	job := &commonmodels.JobTask{
		Name:        "build-0",
		DisplayName: "构建",
		Status:      config.StatusPrepare,
		NotifyCtls: []*commonmodels.NotifyCtl{
			{
				Enabled:     true,
				WebHookType: setting.NotifyWebHookTypeMail,
				NotifyTypes: []string{string(config.StatusPrepare)},
				MailNotificationConfig: &commonmodels.MailNotificationConfig{
					TargetUsers: []*commonmodels.User{{Type: setting.UserTypeUser, UserID: "u1"}},
				},
			},
		},
	}

	sendJobNotifications(&commonmodels.WorkflowTaskCtx{WorkflowName: "wf", TaskID: 1}, job, config.StatusPrepare, zap.NewNop().Sugar())

	if calls != 1 {
		t.Fatalf("send task notifications called %d times, want 1", calls)
	}
	if lastInput == nil {
		t.Fatalf("task notification input is nil")
	}
	if lastInput.WorkflowName != "wf" || lastInput.TaskID != 1 {
		t.Fatalf("unexpected workflow identity: %s/%d", lastInput.WorkflowName, lastInput.TaskID)
	}
	if lastInput.Job != job {
		t.Fatalf("notification job = %#v, want original job", lastInput.Job)
	}
	if lastInput.Status != config.StatusPrepare {
		t.Fatalf("notification status = %s, want %s", lastInput.Status, config.StatusPrepare)
	}
	if lastInput.StatusTextKeyOverride != "taskStatusExecutionStarted" {
		t.Fatalf("status text override = %q, want taskStatusExecutionStarted", lastInput.StatusTextKeyOverride)
	}
}

func TestSendJobNotificationsDoesNotSendWhenStatusDoesNotMatch(t *testing.T) {
	origSendTaskNotifications := sendTaskNotifications
	defer func() {
		sendTaskNotifications = origSendTaskNotifications
	}()

	var calls int
	sendTaskNotifications = func(input *instantmessage.TaskNotifyInput) error {
		calls++
		return nil
	}

	job := &commonmodels.JobTask{
		Name:        "build-0",
		DisplayName: "构建",
		Status:      config.StatusFailed,
		NotifyCtls: []*commonmodels.NotifyCtl{
			{
				Enabled:     true,
				WebHookType: setting.NotifyWebHookTypeMail,
				NotifyTypes: []string{string(config.StatusPassed)},
				MailNotificationConfig: &commonmodels.MailNotificationConfig{
					TargetUsers: []*commonmodels.User{{Type: setting.UserTypeUser, UserID: "u1"}},
				},
			},
		},
	}

	sendJobNotifications(&commonmodels.WorkflowTaskCtx{WorkflowName: "wf", TaskID: 1}, job, config.StatusFailed, zap.NewNop().Sugar())

	if calls != 0 {
		t.Fatalf("send task notifications called %d times, want 0", calls)
	}
}

func TestSendJobNotificationsSendsFinalStatusWithoutStartTextOverride(t *testing.T) {
	origSendTaskNotifications := sendTaskNotifications
	defer func() {
		sendTaskNotifications = origSendTaskNotifications
	}()

	var lastInput *instantmessage.TaskNotifyInput
	sendTaskNotifications = func(input *instantmessage.TaskNotifyInput) error {
		lastInput = input
		return nil
	}

	job := &commonmodels.JobTask{
		Name:        "build-0",
		DisplayName: "构建",
		Status:      config.StatusFailed,
		NotifyCtls: []*commonmodels.NotifyCtl{
			{
				Enabled:     true,
				WebHookType: setting.NotifyWebHookTypeMail,
				NotifyTypes: []string{string(config.StatusFailed)},
				MailNotificationConfig: &commonmodels.MailNotificationConfig{
					TargetUsers: []*commonmodels.User{{Type: setting.UserTypeUser, UserID: "u1"}},
				},
			},
		},
	}

	sendJobNotifications(&commonmodels.WorkflowTaskCtx{WorkflowName: "wf", TaskID: 1}, job, config.StatusFailed, zap.NewNop().Sugar())

	if lastInput == nil {
		t.Fatalf("task notification input is nil")
	}
	if lastInput.Status != config.StatusFailed {
		t.Fatalf("notification status = %s, want %s", lastInput.Status, config.StatusFailed)
	}
	if lastInput.StatusTextKeyOverride != "" {
		t.Fatalf("status text override = %q, want empty", lastInput.StatusTextKeyOverride)
	}
}
