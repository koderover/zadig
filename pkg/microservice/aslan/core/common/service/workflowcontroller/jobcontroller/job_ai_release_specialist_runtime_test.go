package jobcontroller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestBuildAIReleaseSpecialistEvaluationPromptScopesRuntimeReplicaCountsByEnvironment(t *testing.T) {
	prompt, err := BuildAIReleaseSpecialistEvaluationPrompt(nil, "", &commonmodels.AIReleaseSpecialistInput{
		RuntimeServices: &commonmodels.AIRuntimeServicesSummary{
			Items: []*commonmodels.AIRuntimeServiceItem{
				{EnvName: "qa", ServiceName: "service1", PodCount: 2, ReadyPods: 2},
				{EnvName: "prod", ServiceName: "service1", PodCount: 1, ReadyPods: 1},
			},
		},
	})

	require.NoError(t, err)
	require.Contains(t, prompt, "不同环境的副本数允许不同")
	require.Contains(t, prompt, "workloads[].replicas")
	require.Contains(t, prompt, "不能仅因为 ready_pods != workloads[].replicas 给出 warning")
}

func TestBuildAIRuntimeServiceItemDoesNotExposeConfiguredReplicaCount(t *testing.T) {
	item := buildAIRuntimeServiceItem(
		&commonmodels.Product{EnvName: "prod", Production: true},
		&commonmodels.ProductService{
			ServiceName: "service1",
			WorkLoads: []*commonmodels.WorkLoad{
				{WorkloadType: "Deployment", WorkloadName: "service1", Replicas: 2},
			},
		},
	)

	payload, err := json.Marshal(item)
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"replicas"`)
}
