package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestProjectCardClearsDeploymentsWithoutDiscardingOtherDraftValues(t *testing.T) {
	draft := &models.OnboardingInput{
		ProjectKey: "demo", ProjectName: "Demo", ProjectType: "k8s",
		Services:    []models.ServiceInput{{Name: "api", Source: "yaml_git"}},
		Environment: &models.EnvironmentInput{Name: "dev", Services: []string{"api"}},
	}
	source := &models.Run{Plan: common.JSON(models.OperationPlan{Draft: draft})}
	patch, err := onboardingCardPatch(source, map[string]any{"environment.choice": "empty"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := mergeOnboardingDraft(draft, patch)
	if err != nil || saved.Environment == nil || saved.Environment.Name != "dev" || len(saved.Environment.Services) != 0 || saved.ProjectKey != "demo" || len(saved.Services) != 1 {
		t.Fatalf("incorrect merged draft: %#v, %v", saved, err)
	}
	if _, err := onboardingCardPatch(source, map[string]any{"environment.choice": "deploy"}); err == nil {
		t.Fatal("a deployment choice without selected services was accepted")
	}
}

func TestInvalidProjectKeyIsRejectedBeforeCreatingContinuation(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WithArgs("user", "submit", "user").
		WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}))
	req := models.RunRequest{RequestID: "submit", Message: "项目标识", UIContinuation: &models.UIContinuation{
		Card: models.UICard{
			ID: "card", SchemaVersion: 1, Title: "项目标识", Type: "question", Status: "pending", Revision: 2, DraftFields: true,
			Data:    common.JSON(models.UIData{Questions: []models.UIQuestion{{ID: "project_key", Title: "项目标识", Type: "text", Required: true}}}),
			Actions: []models.UIAction{{ID: "submit", Label: "提交", Type: "submit"}},
		},
		SourceRun: &models.Run{Plan: common.JSON(models.OperationPlan{Draft: &models.OnboardingInput{}})},
		Revision:  2, ActionID: "submit", Values: map[string]any{"project_key": "Demo/QA"},
	}}
	service := &AgentService{Store: store}
	cfg := &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true, HistoryRetentionDays: 90}}
	if _, err := service.Start(context.Background(), "user", "conversation-1", "", req, cfg); !errors.Is(err, common.ErrInvalidRequest) || !strings.Contains(err.Error(), "项目标识") {
		t.Fatalf("wanted a field validation error before opening a run transaction, got %v", err)
	}
}
