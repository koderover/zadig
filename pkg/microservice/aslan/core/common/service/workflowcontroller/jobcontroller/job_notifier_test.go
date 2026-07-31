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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func buildNotifyTestJob(name, origin string, status config.Status) *commonmodels.JobTask {
	return &commonmodels.JobTask{
		Name:       name,
		OriginName: origin,
		Status:     status,
		NotifyCtls: []*commonmodels.NotifyCtl{
			{
				Enabled:                    true,
				WebHookType:                "feishu",
				NotifyTypes:                []string{string(config.StatusPassed), string(config.StatusFailed), string(config.StatusPrepare)},
				LarkHookNotificationConfig: &commonmodels.LarkHookNotificationConfig{HookAddress: "https://example.com/hook"},
			},
		},
	}
}

func TestJobNotifierGroupsSplitJobTasks(t *testing.T) {
	log.Init(&log.Config{Level: "error"})

	var sent []*instantmessage.TaskNotifyInput
	originalSend := sendTaskNotifications
	sendTaskNotifications = func(input *instantmessage.TaskNotifyInput) error {
		sent = append(sent, input)
		return nil
	}
	defer func() { sendTaskNotifications = originalSend }()

	jobA1 := buildNotifyTestJob("job-1-0-0-build", "build", "")
	jobA2 := buildNotifyTestJob("job-1-0-1-build", "build", "")
	jobB := buildNotifyTestJob("job-1-1-0-deploy", "deploy", "")

	workflowCtx := &commonmodels.WorkflowTaskCtx{
		WorkflowName:        "wf",
		TaskID:              1,
		GlobalContextGetAll: func() map[string]string { return map[string]string{} },
	}
	notifier := newJobNotifier([]*commonmodels.JobTask{jobA1, jobA2, jobB}, workflowCtx, log.SugaredLogger())

	// Only the first started job of a group triggers the start notification.
	notifier.jobStarted(jobA1)
	notifier.jobStarted(jobA2)
	notifier.jobStarted(jobB)
	if len(sent) != 2 {
		t.Fatalf("expected 2 prepare notifications, got %d", len(sent))
	}
	if len(sent[0].Jobs) != 2 || sent[0].Status != config.StatusPrepare {
		t.Fatalf("expected grouped prepare notification with 2 jobs, got %d jobs, status %s", len(sent[0].Jobs), sent[0].Status)
	}

	// Final notification is sent once per group, after all group jobs finish,
	// with the aggregated status.
	sent = nil
	jobA1.Status = config.StatusPassed
	notifier.jobFinished(jobA1)
	if len(sent) != 0 {
		t.Fatalf("expected no notification before the whole group finishes, got %d", len(sent))
	}
	jobA2.Status = config.StatusFailed
	notifier.jobFinished(jobA2)
	if len(sent) != 1 {
		t.Fatalf("expected 1 final notification for the group, got %d", len(sent))
	}
	if sent[0].Status != config.StatusFailed || len(sent[0].Jobs) != 2 {
		t.Fatalf("expected aggregated failed status with 2 jobs, got status %s with %d jobs", sent[0].Status, len(sent[0].Jobs))
	}

	// flush notifies groups with finished jobs whose remaining jobs never ran.
	sent = nil
	jobB.Status = config.StatusPassed
	notifier.jobFinished(jobB)
	notifier.flush()
	if len(sent) != 1 {
		t.Fatalf("expected 1 notification for the single-job group, got %d", len(sent))
	}
	if sent[0].Status != config.StatusPassed {
		t.Fatalf("expected passed status, got %s", sent[0].Status)
	}
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
	for _, c := range cases {
		jobs := make([]*commonmodels.JobTask, 0, len(c.statuses))
		for _, s := range c.statuses {
			jobs = append(jobs, &commonmodels.JobTask{Status: s})
		}
		if got := aggregateJobNotifyStatus(jobs); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
