package service

import (
	"reflect"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

func Test_newLLMClient(t *testing.T) {
	type args struct {
		llmIntegration *models.LLMIntegration
	}
	tests := []struct {
		name    string
		args    args
		want    llm.ILLM
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newLLMClient(tt.args.llmIntegration)
			if (err != nil) != tt.wantErr {
				t.Errorf("newLLMClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newLLMClient() = %v, want %v", got, tt.want)
			}
		})
	}
}
