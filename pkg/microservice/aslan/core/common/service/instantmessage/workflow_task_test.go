package instantmessage

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestHasTaskNotifyCtls(t *testing.T) {
	prepareNotify := &models.NotifyCtl{
		Enabled:     true,
		WebHookType: setting.NotifyWebHookTypeMail,
		NotifyTypes: []string{string(config.StatusPrepare)},
		MailNotificationConfig: &models.MailNotificationConfig{
			TargetUsers: []*models.User{{Type: setting.UserTypeUser, UserID: "u1"}},
		},
	}

	tests := []struct {
		name       string
		notifyCtls []*models.NotifyCtl
		status     config.Status
		want       bool
	}{
		{
			name:       "match configured status",
			notifyCtls: []*models.NotifyCtl{prepareNotify},
			status:     config.StatusPrepare,
			want:       true,
		},
		{
			name: "ignore disabled notify",
			notifyCtls: []*models.NotifyCtl{
				{
					Enabled:     false,
					WebHookType: setting.NotifyWebHookTypeMail,
					NotifyTypes: []string{string(config.StatusPrepare)},
					MailNotificationConfig: &models.MailNotificationConfig{
						TargetUsers: []*models.User{{Type: setting.UserTypeUser, UserID: "u1"}},
					},
				},
			},
			status: config.StatusPrepare,
			want:   false,
		},
		{
			name: "ignore unmatched status",
			notifyCtls: []*models.NotifyCtl{
				{
					Enabled:     true,
					WebHookType: setting.NotifyWebHookTypeMail,
					NotifyTypes: []string{string(config.StatusPassed)},
					MailNotificationConfig: &models.MailNotificationConfig{
						TargetUsers: []*models.User{{Type: setting.UserTypeUser, UserID: "u1"}},
					},
				},
			},
			status: config.StatusPrepare,
			want:   false,
		},
		{
			name: "skip invalid legacy config and keep searching",
			notifyCtls: []*models.NotifyCtl{
				{
					Enabled:     true,
					WebHookType: setting.NotifyWebHookTypeMail,
					NotifyTypes: []string{string(config.StatusPrepare)},
				},
				prepareNotify,
			},
			status: config.StatusPrepare,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasTaskNotifyCtls(tt.notifyCtls, tt.status)
			if got != tt.want {
				t.Fatalf("HasTaskNotifyCtls() = %v, want %v", got, tt.want)
			}
		})
	}
}
