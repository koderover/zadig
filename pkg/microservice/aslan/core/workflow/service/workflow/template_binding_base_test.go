package workflow

import (
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestTemplateBindingDeltaOnBaseSnapshot(t *testing.T) {
	snapshot := &commonmodels.WorkflowV4Template{
		TemplateName: "publish",
		Stages: []*commonmodels.WorkflowStage{
			{ID: "build", Name: "build"},
			{ID: "deploy", Name: "deploy"},
		},
	}
	for _, tc := range []struct {
		name          string
		patches       []*commonmodels.JSONPatchOperation
		wantStageName string
		wantErr       bool
	}{
		{name: "ordered operations", patches: []*commonmodels.JSONPatchOperation{
			{Operation: "remove", Path: "/stages/0"},
			{Operation: "replace", Path: "/stages/0/name", Value: "publish app"},
		}, wantStageName: "publish app"},
		{name: "empty delta restores base", patches: []*commonmodels.JSONPatchOperation{}, wantStageName: "build"},
		{name: "invalid path rejects complete delta", patches: []*commonmodels.JSONPatchOperation{
			{Operation: "replace", Path: "/stages/0/name", Value: "must not save"},
			{Operation: "replace", Path: "/stages/99/name", Value: "invalid"},
		}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workflow := &commonmodels.WorkflowV4{
				Description: "keep description",
				NotifyCtls:  []*commonmodels.NotifyCtl{{Enabled: true}},
				Stages:      []*commonmodels.WorkflowStage{{Name: "old instance delta"}},
				TemplateBinding: &commonmodels.WorkflowTemplateBinding{
					Enabled: true, TemplateID: "template", BaseVersion: 3, BaseVersionID: "snapshot-3", DeltaPatches: tc.patches,
				},
			}
			_, rendered, err := validateFrontendWorkflowDelta(workflowFromTemplateSnapshot(snapshot, workflow), tc.patches)
			if tc.wantErr {
				if err == nil || rendered != nil {
					t.Fatalf("invalid delta must fail without a partial result: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			applyTemplateBindingInheritedFields(workflow, rendered)
			if workflow.Stages[0].Name != tc.wantStageName {
				t.Fatalf("unexpected stages: %+v", workflow.Stages)
			}
			if workflow.TemplateBinding.BaseVersion != 3 || workflow.TemplateBinding.BaseVersionID != "snapshot-3" {
				t.Fatalf("base changed: %+v", workflow.TemplateBinding)
			}
			if workflow.Description != "keep description" || len(workflow.NotifyCtls) != 1 {
				t.Fatal("instance settings lost")
			}
		})
	}
}
