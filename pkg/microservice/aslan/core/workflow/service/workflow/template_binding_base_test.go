package workflow

import (
	"context"
	"testing"

	"github.com/spf13/viper"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.mongodb.org/mongo-driver/mongo/options"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

func TestRenderTemplateBindingOnCurrentBase(t *testing.T) {
	previousDB := viper.Get("ASLAN_DB")
	viper.Set("ASLAN_DB", "zadig")
	defer viper.Set("ASLAN_DB", previousDB)
	clientOptions := options.Client()
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock).ClientOptions(clientOptions))
	defer mt.Close()
	mt.Run("complete delta", func(mt *mtest.T) {
		mongotool.InitWithOption(context.Background(), clientOptions)
		snapshotID := primitive.NewObjectID()
		snapshot := bson.D{
			{Key: "_id", Value: snapshotID},
			{Key: "version", Value: 3},
			{Key: "snapshot", Value: bson.D{
				{Key: "template_name", Value: "publish"},
				{Key: "stages", Value: bson.A{
					bson.D{{Key: "id", Value: "build"}, {Key: "name", Value: "build"}},
					bson.D{{Key: "id", Value: "deploy"}, {Key: "name", Value: "deploy"}},
				}},
			}},
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
				mt.AddMockResponses(mtest.CreateCursorResponse(0, "zadig.workflow_template_version", mtest.FirstBatch, snapshot))
				workflow := &commonmodels.WorkflowV4{
					Description: "keep description",
					NotifyCtls:  []*commonmodels.NotifyCtl{{Enabled: true}},
					Stages:      []*commonmodels.WorkflowStage{{Name: "old instance delta"}},
					TemplateBinding: &commonmodels.WorkflowTemplateBinding{
						Enabled: true, TemplateID: "template", BaseVersion: 3, BaseVersionID: snapshotID.Hex(), DeltaPatches: tc.patches,
					},
				}
				rendered, err := renderTemplateBoundWorkflowOnBase(workflow)
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
				if workflow.TemplateBinding.BaseVersion != 3 || workflow.TemplateBinding.BaseVersionID != snapshotID.Hex() {
					t.Fatalf("base changed: %+v", workflow.TemplateBinding)
				}
				if workflow.Description != "keep description" || len(workflow.NotifyCtls) != 1 {
					t.Fatal("instance settings lost")
				}
			})
		}
	})
}
