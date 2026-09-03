package kube

import (
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildReplicaOverridesByRenderedYaml_ReplicaChanged(t *testing.T) {
	candidateYaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  replicas: 3
`

	replicaMap, err := ExtractWorkloadReplicas(candidateYaml)
	require.NoError(t, err)
	got, err := buildReplicaOverridesFromReplicaMap(replicaMap)
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

	replicaMap, err := ExtractWorkloadReplicas(candidateYaml)
	require.NoError(t, err)
	got, err := buildReplicaOverridesFromReplicaMap(replicaMap)
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

func TestFilterPodsByControllerUID(t *testing.T) {
	t.Parallel()

	controller := true
	targetUID := types.UID("target-workload")
	ownedPod := podWithController("owned", targetUID, &controller)
	siblingPod := podWithController("sibling", types.UID("sibling-workload"), &controller)
	orphanPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "orphan"}}
	nonControllerPod := podWithController("non-controller", targetUID, nil)

	pods := FilterPodsByControllerUID(
		[]*corev1.Pod{ownedPod, siblingPod, orphanPod, nonControllerPod},
		targetUID,
	)

	require.Len(t, pods, 1)
	require.Equal(t, ownedPod.Name, pods[0].Name)
}

func TestFilterDeploymentPodsByOwner(t *testing.T) {
	t.Parallel()

	controller := true
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: "osl-mm-oms-lane5",
		UID:  types.UID("lane5-deployment"),
	}}
	siblingDeploymentUID := types.UID("main-deployment")

	ownedReplicaSet := replicaSetWithController("osl-mm-oms-lane5-abc", types.UID("lane5-replicaset"), deployment.UID, &controller)
	siblingReplicaSet := replicaSetWithController("osl-mm-oms-xyz", types.UID("main-replicaset"), siblingDeploymentUID, &controller)
	ownedPod := podWithController("osl-mm-oms-lane5-abc-1", ownedReplicaSet.UID, &controller)
	siblingPod := podWithController("osl-mm-oms-xyz-1", siblingReplicaSet.UID, &controller)

	pods := FilterDeploymentPodsByOwner(
		deployment,
		[]*appsv1.ReplicaSet{ownedReplicaSet, siblingReplicaSet},
		[]*corev1.Pod{ownedPod, siblingPod},
	)

	require.Len(t, pods, 1)
	require.Equal(t, ownedPod.Name, pods[0].Name)
}

func TestFilterDeploymentPodsByOwnerExcludesOverlappingSiblingPods(t *testing.T) {
	t.Parallel()

	controller := true
	zero := int32(0)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "osl-mm-oms-lane5", UID: types.UID("lane5-deployment")},
		Spec:       appsv1.DeploymentSpec{Replicas: &zero},
	}
	siblingReplicaSet := replicaSetWithController(
		"osl-mm-oms-main",
		types.UID("main-replicaset"),
		types.UID("main-deployment"),
		&controller,
	)
	siblingPod := podWithController("osl-mm-oms-main-1", siblingReplicaSet.UID, &controller)

	pods := FilterDeploymentPodsByOwner(
		deployment,
		[]*appsv1.ReplicaSet{siblingReplicaSet},
		[]*corev1.Pod{siblingPod},
	)

	require.Empty(t, pods)
}

func TestFilterDeploymentPodsByOwnerIncludesAllOwnedReplicaSets(t *testing.T) {
	t.Parallel()

	controller := true
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: "api",
		UID:  types.UID("api-deployment"),
	}}
	oldReplicaSet := replicaSetWithController("api-old", types.UID("api-old"), deployment.UID, &controller)
	newReplicaSet := replicaSetWithController("api-new", types.UID("api-new"), deployment.UID, &controller)

	pods := FilterDeploymentPodsByOwner(
		deployment,
		[]*appsv1.ReplicaSet{oldReplicaSet, newReplicaSet},
		[]*corev1.Pod{
			podWithController("api-old-1", oldReplicaSet.UID, &controller),
			podWithController("api-new-1", newReplicaSet.UID, &controller),
		},
	)

	require.Len(t, pods, 2)
}

func TestFilterJobsByControllerUID(t *testing.T) {
	t.Parallel()

	controller := true
	cronJobUID := types.UID("target-cronjob")
	ownedJob := jobWithController("target-1", cronJobUID, &controller)
	siblingJob := jobWithController("sibling-1", types.UID("sibling-cronjob"), &controller)
	orphanJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "orphan"}}

	jobs := FilterJobsByControllerUID(
		[]*batchv1.Job{ownedJob, siblingJob, orphanJob},
		cronJobUID,
	)

	require.Len(t, jobs, 1)
	require.Equal(t, ownedJob.Name, jobs[0].Name)
}

func replicaSetWithController(name string, uid, deploymentUID types.UID, controller *bool) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		UID:  uid,
		OwnerReferences: []metav1.OwnerReference{{
			Kind:       "Deployment",
			UID:        deploymentUID,
			Controller: controller,
		}},
	}}
}

func podWithController(name string, controllerUID types.UID, controller *bool) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		OwnerReferences: []metav1.OwnerReference{{
			UID:        controllerUID,
			Controller: controller,
		}},
	}}
}

func jobWithController(name string, controllerUID types.UID, controller *bool) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		OwnerReferences: []metav1.OwnerReference{{
			UID:        controllerUID,
			Controller: controller,
		}},
	}}
}
