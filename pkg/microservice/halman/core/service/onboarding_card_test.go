package service

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
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
