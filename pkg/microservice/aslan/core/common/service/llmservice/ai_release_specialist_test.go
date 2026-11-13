package llmservice

import (
	"context"
	"errors"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

func TestAIReleaseSpecialistModelSelection(t *testing.T) {
	originalRead, originalFind, originalDefault := readAIReleaseSpecialistConfig, findAIReleaseSpecialistModel, defaultAIReleaseSpecialistClient
	t.Cleanup(func() {
		readAIReleaseSpecialistConfig, findAIReleaseSpecialistModel, defaultAIReleaseSpecialistClient = originalRead, originalFind, originalDefault
	})
	unavailable := errors.New("unavailable")
	for _, input := range []struct {
		name, id                     string
		readErr, findErr, defaultErr error
		protocol                     llm.Protocol
		wantErr                      bool
		defaults, finds              int
	}{
		{name: "no selection uses default", defaults: 1},
		{name: "selected model", id: "selected", protocol: llm.ProtocolOpenAI, finds: 1},
		{name: "deleted selected model", id: "deleted", findErr: unavailable, wantErr: true, finds: 1},
		{name: "unusable selected model", id: "invalid", protocol: "invalid", wantErr: true, finds: 1},
		{name: "config read failure", readErr: unavailable, wantErr: true},
		{name: "default unavailable", defaultErr: unavailable, defaults: 1, wantErr: true},
	} {
		t.Run(input.name, func(t *testing.T) {
			defaults, finds := 0, 0
			readAIReleaseSpecialistConfig = func(context.Context) (*models.AIReleaseSpecialistConfig, error) {
				return &models.AIReleaseSpecialistConfig{LLMIntegrationID: input.id}, input.readErr
			}
			findAIReleaseSpecialistModel = func(_ context.Context, id string) (*models.LLMIntegration, error) {
				finds++
				if id != input.id {
					t.Fatal("wrong selected ID")
				}
				return &models.LLMIntegration{Name: "selected", Model: "selected-model", Protocol: input.protocol, ProviderName: llm.ProviderOther, Token: "secret", BaseURL: "https://model.test/v1"}, input.findErr
			}
			defaultAIReleaseSpecialistClient = func(context.Context) (llm.ILLM, error) { defaults++; return nil, input.defaultErr }
			client, err := GetAIReleaseSpecialistLLMClient(context.Background())
			if (err != nil) != input.wantErr || defaults != input.defaults || finds != input.finds {
				t.Fatalf("err=%v defaults=%d finds=%d", err, defaults, finds)
			}
			if input.id == "selected" && (client == nil || client.GetModel() != "selected-model") {
				t.Fatal("selected model was not configured")
			}
		})
	}
}
