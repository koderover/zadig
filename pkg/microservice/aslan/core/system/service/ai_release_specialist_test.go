package service

import (
	"context"
	"errors"
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestAIReleaseSpecialistConfigService(t *testing.T) {
	originalRead, originalWrite, originalFind := readAIReleaseSpecialistConfig, writeAIReleaseSpecialistConfig, findAIReleaseSpecialistModel
	t.Cleanup(func() {
		readAIReleaseSpecialistConfig, writeAIReleaseSpecialistConfig, findAIReleaseSpecialistModel = originalRead, originalWrite, originalFind
	})
	ctx := context.Background()
	dbErr := errors.New("database unavailable")
	for _, input := range []struct {
		name, id              string
		findErr, writeErr     error
		wantErr               bool
		wantWrites, wantFinds int
	}{
		{name: "clear selection", wantWrites: 1},
		{name: "selected model", id: "selected", wantWrites: 1, wantFinds: 1},
		{name: "missing or invalid model", id: "missing", findErr: dbErr, wantErr: true, wantFinds: 1},
		{name: "save failed", writeErr: dbErr, wantErr: true, wantWrites: 1},
	} {
		t.Run(input.name, func(t *testing.T) {
			writes, finds := 0, 0
			findAIReleaseSpecialistModel = func(_ context.Context, id string) (*commonmodels.LLMIntegration, error) {
				finds++
				if id != input.id {
					t.Fatal("wrong model lookup")
				}
				return &commonmodels.LLMIntegration{}, input.findErr
			}
			writeAIReleaseSpecialistConfig = func(_ context.Context, config *commonmodels.AIReleaseSpecialistConfig) error {
				writes++
				if config.LLMIntegrationID != input.id {
					t.Fatal("wrong config saved")
				}
				return input.writeErr
			}
			err := UpdateAIReleaseSpecialistConfig(ctx, &commonmodels.AIReleaseSpecialistConfig{LLMIntegrationID: input.id})
			if (err != nil) != input.wantErr || writes != input.wantWrites || finds != input.wantFinds {
				t.Fatalf("err=%v writes=%d finds=%d", err, writes, finds)
			}
		})
	}
	if err := UpdateAIReleaseSpecialistConfig(ctx, nil); err == nil {
		t.Fatal("nil config accepted")
	}
	readAIReleaseSpecialistConfig = func(context.Context) (*commonmodels.AIReleaseSpecialistConfig, error) {
		return &commonmodels.AIReleaseSpecialistConfig{LLMIntegrationID: "selected"}, nil
	}
	if config, err := GetAIReleaseSpecialistConfig(ctx); err != nil || config.LLMIntegrationID != "selected" {
		t.Fatalf("read config: %+v %v", config, err)
	}
	readAIReleaseSpecialistConfig = func(context.Context) (*commonmodels.AIReleaseSpecialistConfig, error) { return nil, dbErr }
	if _, err := GetAIReleaseSpecialistConfig(ctx); err == nil {
		t.Fatal("read failure ignored")
	}
}
