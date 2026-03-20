package kube

import (
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/stretchr/testify/require"
)

func TestReconcileReplicaOverridesByRenderedYaml_ReplicaChanged(t *testing.T) {
	candidateYaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  replicas: 3
`

	got, err := buildReplicaOverridesByRenderedYaml(candidateYaml)
	require.NoError(t, err)

	overrideMap := toOverrideMap(got)
	require.Equal(t, int32(3), overrideMap[ReplicaOverrideKey(setting.Deployment, "demo")])
}

func TestBuildReplicaOverridesByRenderedYaml_MultiWorkloads(t *testing.T) {
	candidateYaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  replicas: 2
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: db
spec:
  replicas: 1
`

	got, err := buildReplicaOverridesByRenderedYaml(candidateYaml)
	require.NoError(t, err)

	overrideMap := toOverrideMap(got)
	require.Equal(t, int32(2), overrideMap[ReplicaOverrideKey(setting.Deployment, "demo")])
	require.Equal(t, int32(1), overrideMap[ReplicaOverrideKey(setting.StatefulSet, "db")])
}

func toOverrideMap(overrides []*commonmodels.WorkLoad) map[string]int32 {
	ret := make(map[string]int32)
	for _, item := range overrides {
		if item == nil {
			continue
		}
		ret[ReplicaOverrideKey(item.WorkloadType, item.WorkloadName)] = item.Replicas
	}
	return ret
}
