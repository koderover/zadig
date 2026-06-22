package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"go.uber.org/zap"
)

const (
	testJobServiceName = "hello-kubernetes"
	testJobNamespace   = "test-ns"
	testProductName    = "test-project"
	testEnvName        = "dev"
)

func TestGetServiceWorkloadsIncludesJob(t *testing.T) {
	job, pod := newJobWorkloadObjects()
	inf := newJobWorkloadInformer(t, job, pod)
	env := &commonmodels.Product{
		ProductName: testProductName,
		EnvName:     testEnvName,
		Namespace:   testJobNamespace,
		Services: [][]*commonmodels.ProductService{{
			{
				ServiceName: testJobServiceName,
				ProductName: testProductName,
				Type:        setting.K8SDeployType,
			},
		}},
	}

	workloads, err := GetServiceWorkloads(newJobWorkloadTemplate(), env, inf, zap.NewNop().Sugar())
	require.NoError(t, err)
	require.Len(t, workloads, 1)
	require.Equal(t, setting.Job, workloads[0].Type)
	require.True(t, workloads[0].Ready)
	require.Equal(t, []string{"busybox"}, workloads[0].Images)
}

func newJobWorkloadTemplate() *commonmodels.Service {
	return &commonmodels.Service{
		ServiceName: testJobServiceName,
		ProductName: testProductName,
		Yaml: `apiVersion: batch/v1
kind: Job
metadata:
  name: hello-kubernetes
spec:
  template:
    metadata:
      labels:
        job-name: hello-kubernetes
    spec:
      restartPolicy: Never
      containers:
      - name: hello
        image: busybox
`,
	}
}

func newJobWorkloadObjects() (*batchv1.Job, *corev1.Pod) {
	now := metav1.NewTime(time.Unix(1, 0))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testJobServiceName,
			Namespace:         testJobNamespace,
			CreationTimestamp: now,
		},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"job-name": testJobServiceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"job-name": testJobServiceName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:  "hello",
						Image: "busybox",
					}},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testJobServiceName + "-pod",
			Namespace:         testJobNamespace,
			CreationTimestamp: now,
			Labels: map[string]string{
				"job-name": testJobServiceName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "hello",
				Image: "busybox",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}

	return job, pod
}

func newJobWorkloadInformer(t *testing.T, objs ...runtime.Object) informers.SharedInformerFactory {
	t.Helper()

	clientset := k8sfake.NewSimpleClientset(objs...)
	inf := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace(testJobNamespace))
	inf.Batch().V1().Jobs().Informer()
	inf.Core().V1().Pods().Informer()

	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })

	inf.Start(stopCh)
	for informerType, synced := range inf.WaitForCacheSync(stopCh) {
		require.Truef(t, synced, "failed to sync informer %v", informerType)
	}

	return inf
}
