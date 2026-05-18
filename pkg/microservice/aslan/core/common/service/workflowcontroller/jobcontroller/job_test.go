package jobcontroller

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestShouldSkipApprovalJobForReleasePlan(t *testing.T) {
	tests := []struct {
		name        string
		job         *commonmodels.JobTask
		workflowCtx *commonmodels.WorkflowTaskCtx
		want        bool
	}{
		{
			name: "skip approval job for approved release plan task",
			job: &commonmodels.JobTask{
				JobType: string(config.JobApproval),
			},
			workflowCtx: &commonmodels.WorkflowTaskCtx{
				ReleasePlan: &commonmodels.ReleasePlanRef{
					ApprovalPassed: true,
				},
			},
			want: true,
		},
		{
			name: "do not skip non approval job",
			job: &commonmodels.JobTask{
				JobType: string(config.JobZadigBuild),
			},
			workflowCtx: &commonmodels.WorkflowTaskCtx{
				ReleasePlan: &commonmodels.ReleasePlanRef{
					ApprovalPassed: true,
				},
			},
			want: false,
		},
		{
			name: "do not skip when release plan approval not passed",
			job: &commonmodels.JobTask{
				JobType: string(config.JobApproval),
			},
			workflowCtx: &commonmodels.WorkflowTaskCtx{
				ReleasePlan: &commonmodels.ReleasePlanRef{
					ApprovalPassed: false,
				},
			},
			want: false,
		},
		{
			name: "do not skip direct workflow task without release plan",
			job: &commonmodels.JobTask{
				JobType: string(config.JobApproval),
			},
			workflowCtx: &commonmodels.WorkflowTaskCtx{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipApprovalJobForReleasePlan(tt.job, tt.workflowCtx); got != tt.want {
				t.Fatalf("shouldSkipApprovalJobForReleasePlan() = %v, want %v", got, tt.want)
			}
		})
	}
}
