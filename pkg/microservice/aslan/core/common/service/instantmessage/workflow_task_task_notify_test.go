/*
Copyright 2026 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package instantmessage

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestNotifyStatusMatchRejectWithFailedSubscription(t *testing.T) {
	failedOnly := sets.NewString(string(config.StatusFailed))
	require.True(t, notifyStatusMatch(failedOnly, config.StatusReject))
	require.True(t, notifyStatusMatch(failedOnly, config.StatusFailed))
	require.False(t, notifyStatusMatch(failedOnly, config.StatusPassed))
}

func TestWaitingApproveNotificationPresentation(t *testing.T) {
	content, err := getJobTaskTplExec(
		`{{taskStatus .Job.Status}}`,
		&jobTaskNotification{Job: &models.JobTask{Status: config.StatusWaitingApprove}},
		string(config.SystemLanguageZhCN),
	)
	require.NoError(t, err)
	require.Equal(t, "待确认", content)
	require.Equal(t, feishuHeaderTemplateOrange, getColorTemplateWithStatus(config.StatusWaitingApprove))
}

func TestManualExecStageUsersPreferActualExecutor(t *testing.T) {
	candidates := []*models.User{{UserID: "candidate-1"}, {UserID: "candidate-2"}}
	stage := &models.StageTask{ManualExec: &models.ManualExec{
		ManualExecUsers:   candidates,
		ManualExectorID:   "actual-user",
		ManualExectorName: "Actual User",
	}}

	users := manualExecStageUsers(stage)
	require.Len(t, users, 1)
	require.Equal(t, "actual-user", users[0].UserID)
	require.Equal(t, "Actual User", users[0].UserName)
}

func TestManualExecStageUsersFallBackToCandidates(t *testing.T) {
	candidates := []*models.User{{UserID: "candidate-1"}, {UserID: "candidate-2"}}
	stage := &models.StageTask{ManualExec: &models.ManualExec{ManualExecUsers: candidates}}

	require.Equal(t, candidates, manualExecStageUsers(stage))
}
