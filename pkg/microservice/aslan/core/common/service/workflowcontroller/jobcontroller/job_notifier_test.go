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

package jobcontroller

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
)

func newNotifyTestJob(name, origin string, status config.Status) *commonmodels.JobTask {
	return &commonmodels.JobTask{
		Name:       name,
		OriginName: origin,
		Status:     status,
		NotifyCtls: []*commonmodels.NotifyCtl{
			{
				Enabled:                    true,
				WebHookType:                "feishu",
				LarkHookNotificationConfig: &commonmodels.LarkHookNotificationConfig{HookAddress: "https://example.com/hook"},
				NotifyTypes: []string{
					string(config.StatusPrepare),
					string(config.StatusPassed),
					string(config.StatusFailed),
				},
			},
		},
	}
}

func withSendTaskNotificationsStub(t *testing.T, stub func(input *instantmessage.TaskNotifyInput) error) {
	t.Helper()
	originalSend := sendTaskNotifications
	sendTaskNotifications = stub
	t.Cleanup(func() { sendTaskNotifications = originalSend })
}

func newNotifyTestWorkflowCtx() *commonmodels.WorkflowTaskCtx {
	return &commonmodels.WorkflowTaskCtx{
		WorkflowName: "wf",
		TaskID:       1,
		GlobalContextGetAll: func() map[string]string {
			return map[string]string{}
		},
	}
}

func TestJobNotifierDoesNotHoldLockWhileSending(t *testing.T) {
	job := newNotifyTestJob("job-a", "build-a", "")
	notifier := newJobNotifier([]*commonmodels.JobTask{job}, newNotifyTestWorkflowCtx(), zap.NewNop().Sugar())

	withSendTaskNotificationsStub(t, func(input *instantmessage.TaskNotifyInput) error {
		require.True(t, notifier.mu.TryLock(), "notifier lock is held while sending a notification")
		notifier.mu.Unlock()
		return nil
	})

	notifier.jobStarted(job)
	job.Status = config.StatusPassed
	notifier.jobFinished(job)
}

func TestJobNotifierSendsJobSnapshots(t *testing.T) {
	var got *instantmessage.TaskNotifyInput
	withSendTaskNotificationsStub(t, func(input *instantmessage.TaskNotifyInput) error {
		got = input
		return nil
	})

	jobA := newNotifyTestJob("job-a", "build", config.StatusPrepare)
	jobA.StartTime = 10
	jobB := newNotifyTestJob("job-b", "build", "")
	jobB.StartTime = 0
	notifier := newJobNotifier([]*commonmodels.JobTask{jobA, jobB}, newNotifyTestWorkflowCtx(), zap.NewNop().Sugar())

	notifier.jobStarted(jobA)

	require.NotNil(t, got)
	require.Len(t, got.Jobs, 2)
	require.NotSame(t, jobA, got.Jobs[0])
	require.NotSame(t, jobB, got.Jobs[1])
	require.NotSame(t, jobA, got.Job)

	jobA.Status = config.StatusFailed
	jobA.StartTime = 99
	jobB.Status = config.StatusPassed
	jobB.StartTime = 100

	require.Equal(t, config.StatusPrepare, got.Jobs[0].Status)
	require.Equal(t, int64(10), got.Jobs[0].StartTime)
	require.Empty(t, got.Jobs[1].Status)
	require.Equal(t, int64(0), got.Jobs[1].StartTime)
}

func TestJobNotifierGroupsSplitJobTasks(t *testing.T) {
	var sent []*instantmessage.TaskNotifyInput
	withSendTaskNotificationsStub(t, func(input *instantmessage.TaskNotifyInput) error {
		sent = append(sent, input)
		return nil
	})

	jobA1 := newNotifyTestJob("job-a-1", "build", "")
	jobA2 := newNotifyTestJob("job-a-2", "build", "")
	jobB := newNotifyTestJob("job-b", "deploy", "")
	notifier := newJobNotifier([]*commonmodels.JobTask{jobA1, jobA2, jobB}, newNotifyTestWorkflowCtx(), zap.NewNop().Sugar())

	notifier.jobStarted(jobA1)
	notifier.jobStarted(jobA2)
	notifier.jobStarted(jobB)
	require.Len(t, sent, 2)
	require.Len(t, sent[0].Jobs, 2)
	require.Equal(t, config.StatusPrepare, sent[0].Status)

	sent = nil
	jobA1.Status = config.StatusPassed
	notifier.jobFinished(jobA1)
	require.Empty(t, sent)

	jobA2.Status = config.StatusFailed
	notifier.jobFinished(jobA2)
	require.Len(t, sent, 1)
	require.Equal(t, config.StatusFailed, sent[0].Status)
	require.Len(t, sent[0].Jobs, 2)
}

func TestJobNotifierFlushesStartedIncompleteGroup(t *testing.T) {
	var sent []*instantmessage.TaskNotifyInput
	withSendTaskNotificationsStub(t, func(input *instantmessage.TaskNotifyInput) error {
		sent = append(sent, input)
		return nil
	})

	jobA1 := newNotifyTestJob("job-a-1", "build", "")
	jobA2 := newNotifyTestJob("job-a-2", "build", "")
	jobB := newNotifyTestJob("job-b", "deploy", "")
	notifier := newJobNotifier([]*commonmodels.JobTask{jobA1, jobA2, jobB}, newNotifyTestWorkflowCtx(), zap.NewNop().Sugar())

	notifier.jobStarted(jobA1)
	sent = nil
	jobA1.Status = config.StatusFailed
	notifier.jobFinished(jobA1)
	notifier.flush()

	require.Len(t, sent, 1)
	require.Equal(t, config.StatusFailed, sent[0].Status)
	require.Len(t, sent[0].Jobs, 2)
}

func TestAggregateJobNotifyStatus(t *testing.T) {
	cases := []struct {
		name     string
		statuses []config.Status
		want     config.Status
	}{
		{"all passed", []config.Status{config.StatusPassed, config.StatusPassed}, config.StatusPassed},
		{"one failed", []config.Status{config.StatusPassed, config.StatusFailed}, config.StatusFailed},
		{"reject beats passed", []config.Status{config.StatusPassed, config.StatusReject}, config.StatusReject},
		{"timeout beats failed", []config.Status{config.StatusFailed, config.StatusTimeout}, config.StatusTimeout},
		{"unstarted ignored", []config.Status{config.StatusPassed, ""}, config.StatusPassed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jobs := make([]*commonmodels.JobTask, 0, len(tc.statuses))
			for _, status := range tc.statuses {
				jobs = append(jobs, &commonmodels.JobTask{Status: status})
			}
			require.Equal(t, tc.want, aggregateJobNotifyStatus(jobs))
		})
	}
}
