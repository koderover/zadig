/*
Copyright 2026 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package updater

import (
	"context"
	"fmt"

	"github.com/openkruise/kruise-api/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
)

// PatchCloneSetV2 applies a raw merge patch to a CloneSet.
// CloneSet is a CRD from OpenKruise so we use controller-runtime client
// (no typed kubernetes clientset API available for CRDs).
func PatchCloneSetV2(ctx context.Context, clusterID, namespace, name string, patchBytes []byte) error {
	c, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %w", err)
	}

	err = c.Patch(ctx, &v1alpha1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}, client.RawPatch(types.MergePatchType, patchBytes))
	if err != nil {
		return fmt.Errorf("failed to patch CloneSet %s/%s: %w", namespace, name, err)
	}
	return nil
}

func ScaleCloneSetV2(ctx context.Context, clusterID, namespace, name string, replicas int) error {
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	return PatchCloneSetV2(ctx, clusterID, namespace, name, patchBytes)
}
