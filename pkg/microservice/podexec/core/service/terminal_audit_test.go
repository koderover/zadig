package service

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidatePodReturnsValidatedPod(t *testing.T) {
	want := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "container"}},
		},
	}
	client := fake.NewSimpleClientset(want)

	got, err := ValidatePod(client, "ns", "pod", "container")
	if err != nil {
		t.Fatalf("validate pod: %v", err)
	}
	if got.Name != want.Name || got.Namespace != want.Namespace {
		t.Fatalf("validated pod = %s/%s, want %s/%s", got.Namespace, got.Name, want.Namespace, want.Name)
	}
}

func TestCollectContainerSecretValuesReturnsRequiredSecretError(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "container",
				Env: []corev1.EnvVar{{
					Name: "TOKEN",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "missing"},
							Key:                  "token",
						},
					},
				}},
			}},
		},
	}

	_, err := collectContainerSecretValues(context.Background(), fake.NewSimpleClientset(), pod, "ns", "container")
	if err == nil {
		t.Fatal("expected required secret lookup error")
	}
}
