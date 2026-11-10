package jobcontroller

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

func TestAIReleaseSpecialistCompilationAndExecutionUseConfiguredClient(t *testing.T) {
	originalClient, originalTask := getAIReleaseSpecialistLLMClient, findWorkflowTaskForAIReleaseSpecialist
	t.Cleanup(func() {
		getAIReleaseSpecialistLLMClient, findWorkflowTaskForAIReleaseSpecialist = originalClient, originalTask
	})
	modelErr := errors.New("configured model unavailable")
	clientCalls := 0
	getAIReleaseSpecialistLLMClient = func(context.Context) (llm.ILLM, error) { clientCalls++; return nil, modelErr }
	if _, err := CompileAIReleaseSpecialistRulePlan(context.Background(), "reject scan bugs", &aiReleaseSpecialistRuleCatalog{}); !errors.Is(err, modelErr) || clientCalls != 1 {
		t.Fatalf("rule compilation must use configured client: calls=%d err=%v", clientCalls, err)
	}
	job := &commonmodels.JobTask{Name: "release-specialist"}
	task := &commonmodels.WorkflowTask{Stages: []*commonmodels.StageTask{{Jobs: []*commonmodels.JobTask{job}}}}
	catalog, err := buildAIReleaseSpecialistRuleCatalog(task, job.Name)
	if err != nil {
		t.Fatal(err)
	}
	source := "reject scan bugs"
	plan := &commonmodels.AIReleaseSpecialistRulePlan{
		Version:        aiReleaseSpecialistRulePlanVersion,
		SourceRuleHash: hashAIReleaseSpecialistRuleSource(source),
		ContextHash:    hashAIReleaseSpecialistRuleCatalog(catalog),
		Rules:          []*commonmodels.AIReleaseSpecialistRulePlanRule{{Dimension: "scan", Metric: "bug_count", Operator: "equal", Value: "1", Result: "fail"}},
	}
	findWorkflowTaskForAIReleaseSpecialist = func(string, int64) (*commonmodels.WorkflowTask, error) { return task, nil }
	controller := &AIReleaseSpecialistJobCtl{job: job, workflowCtx: &commonmodels.WorkflowTaskCtx{}, jobTaskSpec: &commonmodels.JobTaskAIReleaseSpecialistSpec{PromptTemplate: source, RulePlan: plan}, ack: func() {}}
	controller.Run(context.Background())
	if clientCalls != 2 || job.Status != config.StatusFailed || !strings.Contains(job.Error, modelErr.Error()) {
		t.Fatalf("execution must use configured client and report failure: calls=%d status=%s error=%s", clientCalls, job.Status, job.Error)
	}
}
