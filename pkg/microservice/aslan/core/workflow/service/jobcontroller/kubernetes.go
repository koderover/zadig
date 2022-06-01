/*
Copyright 2021 The KodeRover Authors.

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

package jobcontroller

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

func GetK8sClients(hubServerAddr, clusterID string) (crClient.Client, kubernetes.Interface, *rest.Config, error) {
	controllerRuntimeClient, err := kubeclient.GetKubeClient(hubServerAddr, clusterID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get controller runtime client: %s", err)
	}

	clientset, err := kubeclient.GetKubeClientSet(hubServerAddr, clusterID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get clientset: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(hubServerAddr, clusterID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get rest config: %s", err)
	}

	return controllerRuntimeClient, clientset, restConfig, nil
}

type JobLabel struct {
	WorkflowName string
	TaskID       int64
	JobType      string
}

const (
	jobLabelTaskKey  = "s-task"
	jobLabelSTypeKey = "s-type"
)

func ensureDeleteConfigMap(namespace string, jobLabel *JobLabel, kubeClient crClient.Client) error {
	ls := getJobLabels(jobLabel)
	return updater.DeleteConfigMapsAndWait(namespace, labels.Set(ls).AsSelector(), kubeClient)
}

func ensureDeleteJob(namespace string, jobLabel *JobLabel, kubeClient crClient.Client) error {
	ls := getJobLabels(jobLabel)
	return updater.DeleteJobsAndWait(namespace, labels.Set(ls).AsSelector(), kubeClient)
}

// getJobLabels get labels k-v map from JobLabel struct
func getJobLabels(jobLabel *JobLabel) map[string]string {
	retMap := map[string]string{
		jobLabelTaskKey:  fmt.Sprintf("%s-%d", strings.ToLower(jobLabel.WorkflowName), jobLabel.TaskID),
		jobLabelSTypeKey: strings.Replace(jobLabel.JobType, "_", "-", -1),
	}
	// no need to add labels with empty value to a job
	for k, v := range retMap {
		if len(v) == 0 {
			delete(retMap, k)
		}
	}
	return retMap
}

func createJobConfigMap(namespace, jobName string, jobLabel *JobLabel, jobCtx string, kubeClient crClient.Client) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    getJobLabels(jobLabel),
		},
		Data: map[string]string{
			"job-config.xml": jobCtx,
		},
	}

	return updater.CreateConfigMap(cm, kubeClient)
}

// getReaperImage generates the image used to run reaper
// depends on the setting on page 'Build'
func getReaperImage(reaperImage, buildOS string) string {
	// for built-in image, reaperImage and buildOs can generate a complete image
	// reaperImage: ccr.ccs.tencentyun.com/koderover-public/build-base:${BuildOS}-amd64
	// buildOS: focal xenial bionic
	jobImage := strings.ReplaceAll(reaperImage, "${BuildOS}", buildOS)
	return jobImage
}

func buildJob(jobType, jobImage, jobName, clusterID, currentNamespace string, resReq setting.Request, resReqSpec setting.RequestSpec, jobTask *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, registries []*task.RegistryNamespace) (*batchv1.Job, error) {

	labels := getJobLabels(&JobLabel{
		WorkflowName: workflowCtx.WorkflowName,
		TaskID:       workflowCtx.TaskID,
		JobType:      string(jobType),
	})

	// 引用集成到系统中的私有镜像仓库的访问权限
	ImagePullSecrets := []corev1.LocalObjectReference{
		{
			Name: setting.DefaultImagePullSecret,
		},
	}
	// for _, reg := range registries {
	// 	secretName, err := genRegistrySecretName(reg)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to generate registry secret name: %s", err)
	// 	}

	// 	secret := corev1.LocalObjectReference{
	// 		Name: secretName,
	// 	}
	// 	ImagePullSecrets = append(ImagePullSecrets, secret)
	// }

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Completions:  int32Ptr(1),
			Parallelism:  int32Ptr(1),
			BackoffLimit: int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: ImagePullSecrets,
					Containers: []corev1.Container{
						{
							ImagePullPolicy: corev1.PullAlways,
							Name:            labels["s-type"],
							Image:           jobImage,
							Env: []corev1.EnvVar{
								{
									Name:  "JOB_CONFIG_FILE",
									Value: path.Join(workflowCtx.ConfigMapMountDir, "job-config.xml"),
								},
								// 连接对应wd上的dockerdeamon
								{
									Name:  "DOCKER_HOST",
									Value: workflowCtx.DockerHost,
								},
							},
							VolumeMounts: getVolumeMounts(workflowCtx.ConfigMapMountDir),
							Resources:    getResourceRequirements(resReq, resReqSpec),

							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						},
					},
					Volumes: getVolumes(jobName),
				},
			},
		},
	}

	// if ctx.CacheEnable && ctx.Cache.MediumType == commontypes.NFSMedium {
	// 	volumeName := "build-cache"
	// 	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
	// 		Name: volumeName,
	// 		VolumeSource: corev1.VolumeSource{
	// 			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
	// 				ClaimName: ctx.Cache.NFSProperties.PVC,
	// 			},
	// 		},
	// 	})

	// 	mountPath := ctx.CacheUserDir
	// 	if ctx.CacheDirType == commontypes.WorkspaceCacheDir {
	// 		mountPath = "/workspace"
	// 	}

	// 	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
	// 		Name:      volumeName,
	// 		MountPath: mountPath,
	// 		SubPath:   ctx.Cache.NFSProperties.Subpath,
	// 	})
	// }

	// if affinity := addNodeAffinity(clusterID, pipelineTask.ConfigPayload.K8SClusters); affinity != nil {
	// 	job.Spec.Template.Spec.Affinity = affinity
	// }

	return job, nil
}

func getVolumeMounts(configMapMountDir string) []corev1.VolumeMount {
	resp := make([]corev1.VolumeMount, 0)

	resp = append(resp, corev1.VolumeMount{
		Name:      "job-config",
		MountPath: configMapMountDir,
	})

	return resp
}

func getVolumes(jobName string) []corev1.Volume {
	resp := make([]corev1.Volume, 0)
	resp = append(resp, corev1.Volume{
		Name: "job-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: jobName,
				},
			},
		},
	})
	return resp
}

func getResourceRequirements(resReq setting.Request, resReqSpec setting.RequestSpec) corev1.ResourceRequirements {

	switch resReq {
	case setting.HighRequest:
		return generateResourceRequirements(setting.HighRequest, setting.HighRequestSpec)

	case setting.MediumRequest:
		return generateResourceRequirements(setting.MediumRequest, setting.MediumRequestSpec)

	case setting.LowRequest:
		return generateResourceRequirements(setting.LowRequest, setting.LowRequestSpec)

	case setting.MinRequest:
		return generateResourceRequirements(setting.MinRequest, setting.MinRequestSpec)

	case setting.DefineRequest:
		return generateResourceRequirements(resReq, resReqSpec)

	default:
		return generateResourceRequirements(setting.DefaultRequest, setting.DefaultRequestSpec)
	}
}

//generateResourceRequirements
//cpu Request:Limit=1:4
//memory default Request:Limit=1:4 ; if memoryLimit>= 8Gi,Request:Limit=1:8
func generateResourceRequirements(req setting.Request, reqSpec setting.RequestSpec) corev1.ResourceRequirements {

	if req != setting.DefineRequest {
		return corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(reqSpec.CpuLimit) + setting.CpuUintM),
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(reqSpec.MemoryLimit) + setting.MemoryUintMi),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(reqSpec.CpuReq) + setting.CpuUintM),
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(reqSpec.MemoryReq) + setting.MemoryUintMi),
			},
		}
	}

	cpuReqInt := reqSpec.CpuLimit / 4
	if cpuReqInt < 1 {
		cpuReqInt = 1
	}
	memoryReqInt := reqSpec.MemoryLimit / 4
	if memoryReqInt >= 2*1024 {
		memoryReqInt = memoryReqInt / 2
	}
	if memoryReqInt < 1 {
		memoryReqInt = 1
	}

	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(reqSpec.CpuLimit) + setting.CpuUintM),
			corev1.ResourceMemory: resource.MustParse(strconv.Itoa(reqSpec.MemoryLimit) + setting.MemoryUintMi),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(cpuReqInt) + setting.CpuUintM),
			corev1.ResourceMemory: resource.MustParse(strconv.Itoa(memoryReqInt) + setting.MemoryUintMi),
		},
	}
}

func int32Ptr(i int32) *int32 { return &i }

func waitJobEndWithFile(ctx context.Context, taskTimeout int, namespace, jobName string, kubeClient crClient.Client, xl *zap.SugaredLogger) (status config.Status) {
	xl.Infof("wait job to start: %s/%s", namespace, jobName)
	timeout := time.After(time.Duration(taskTimeout) * time.Second)
	podTimeout := time.After(120 * time.Second)

	xl.Infof("Timeout of preparing Pod: %s. Timeout of running task: %s.", 120*time.Second, time.Duration(taskTimeout)*time.Second)

	var started bool
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled

		case <-podTimeout:
			return config.StatusTimeout
		default:
			job, _, err := getter.GetJob(namespace, jobName, kubeClient)
			if err != nil {
				xl.Errorf("get job failed, namespace:%s, jobName:%s, err:%v", namespace, jobName, err)
			}
			if job != nil {
				started = job.Status.Active > 0
			}
		}
		if started {
			break
		}

		time.Sleep(time.Second)
	}

	// 等待job 运行结束
	xl.Infof("wait job to end: %s %s", namespace, jobName)
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled

		case <-timeout:
			return config.StatusTimeout

		default:
			job, found, err := getter.GetJob(namespace, jobName, kubeClient)
			if err != nil || !found {
				xl.Errorf("failed to get pod with label job-name=%s %v", jobName, err)
				return config.StatusFailed
			}
			// pod is still running
			if job.Status.Active != 0 {

				pods, err := getter.ListPods(namespace, labels.Set{"job-name": jobName}.AsSelector(), kubeClient)
				if err != nil {
					xl.Errorf("failed to find pod with label job-name=%s %v", jobName, err)
					return config.StatusFailed
				}

				for _, pod := range pods {
					ipod := wrapper.Pod(pod)
					if ipod.Pending() {
						continue
					}
					if ipod.Failed() {
						return config.StatusFailed
					}
					if ipod.Succeeded() {
						return config.StatusPassed
					}
				}

			} else if job.Status.Succeeded != 0 {
				return config.StatusPassed
			} else {
				return config.StatusFailed
			}
		}

		time.Sleep(time.Second * 1)
	}
}
