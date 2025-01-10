package updater

import (
	"fmt"
	"github.com/openkruise/kruise-api/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ScaleCloneSet(ns, name string, replicas int, cl client.Client) error {
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas": %d}}`, replicas))
	return PatchCloneSet(ns, name, patchBytes, cl)
}

func PatchCloneSet(ns, name string, patchBytes []byte, cl client.Client) error {
	return patchObject(&v1alpha1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, patchBytes, cl)
}
