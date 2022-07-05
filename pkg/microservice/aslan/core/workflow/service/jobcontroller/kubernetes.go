/*
Copyright 2022 The KodeRover Authors.

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	s3tool "github.com/koderover/zadig/pkg/tool/s3"
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
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/containerlog"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/podexec"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	commontypes "github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/job"
	"github.com/koderover/zadig/pkg/util"
)

const (
	BusyBoxImage       = "ccr.ccs.tencentyun.com/koderover-public/busybox:latest"
	ZadigContextDir    = "/zadig/"
	ZadigLogFile       = ZadigContextDir + "zadig.log"
	ZadigLifeCycleFile = ZadigContextDir + "lifecycle"
	JobExecutorFile    = "http://resource-server/jobexecutor"
	ResourceServer     = "resource-server"
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
	JobName      string
	JobType      string
}

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
		setting.JobLabelTaskKey:  fmt.Sprintf("%s-%d", strings.ToLower(jobLabel.WorkflowName), jobLabel.TaskID),
		setting.JobLabelNameKey:  strings.Replace(jobLabel.JobName, "_", "-", -1),
		setting.JobLabelSTypeKey: strings.Replace(jobLabel.JobType, "_", "-", -1),
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

func getBaseImage(buildOS, imageFrom string) string {
	// for built-in image, reaperImage and buildOs can generate a complete image
	// reaperImage: ccr.ccs.tencentyun.com/koderover-public/build-base:${BuildOS}-amd64
	// buildOS: focal xenial bionic
	jobImage := strings.ReplaceAll(config.ReaperImage(), "${BuildOS}", buildOS)
	// for custom image, buildOS represents the exact custom image
	if imageFrom == setting.ImageFromCustom {
		jobImage = buildOS
	}
	return jobImage
}

func buildJob(jobType, jobImage, jobName, clusterID, currentNamespace string, resReq setting.Request, resReqSpec setting.RequestSpec, jobTask *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, registries []*task.RegistryNamespace) (*batchv1.Job, error) {
	// 	tailLogCommandTemplate := `tail -f %s &
	// while [ -f %s ];
	// do
	// 	sleep 1s;
	// done;
	// `
	// 	tailLogCommand := fmt.Sprintf(tailLogCommandTemplate, ZadigLogFile, ZadigLifeCycleFile)

	var (
		jobExecutorBootingScript string
		jobExecutorBinaryFile    = JobExecutorFile
	)
	// not local cluster
	if clusterID != "" && clusterID != setting.LocalClusterID {
		jobExecutorBinaryFile = strings.Replace(jobExecutorBinaryFile, ResourceServer, ResourceServer+".koderover-agent", -1)
	} else {
		jobExecutorBinaryFile = strings.Replace(jobExecutorBinaryFile, ResourceServer, ResourceServer+"."+currentNamespace, -1)
	}

	jobExecutorBootingScript = fmt.Sprintf("curl -m 10 --retry-delay 3 --retry 3 -sSL %s -o reaper && chmod +x reaper && mv reaper /usr/local/bin && /usr/local/bin/reaper", jobExecutorBinaryFile)

	labels := getJobLabels(&JobLabel{
		WorkflowName: workflowCtx.WorkflowName,
		TaskID:       workflowCtx.TaskID,
		JobType:      string(jobType),
		JobName:      jobTask.Name,
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
					// InitContainers: []corev1.Container{
					// 	{
					// 		ImagePullPolicy: corev1.PullIfNotPresent,
					// 		Name:            "init-log-file",
					// 		Image:           BusyBoxImage,
					// 		VolumeMounts:    getVolumeMounts(workflowCtx.ConfigMapMountDir),
					// 		Command:         []string{"/bin/sh", "-c", fmt.Sprintf("touch %s %s", ZadigLogFile, ZadigLifeCycleFile)},
					// 	},
					// },
					Containers: []corev1.Container{
						{
							ImagePullPolicy: corev1.PullAlways,
							Name:            jobTask.Name,
							Image:           jobImage,
							Command:         []string{"/bin/sh", "-c"},
							Args:            []string{jobExecutorBootingScript},
							// Command:         []string{"/bin/sh", "-c", "jobexecutor"},
							// Lifecycle: &corev1.Lifecycle{
							// 	PreStop: &corev1.Handler{
							// 		Exec: &corev1.ExecAction{
							// 			Command: []string{"/bin/sh", "-c", fmt.Sprintf("rm %s", ZadigLifeCycleFile)},
							// 		},
							// 	},
							// },
							Env: []corev1.EnvVar{
								{
									Name:  "JOB_CONFIG_FILE",
									Value: path.Join(workflowCtx.ConfigMapMountDir, "job-config.xml"),
								},
								// 连接对应wd上的dockerdeamon
								{
									Name:  "DOCKER_HOST",
									Value: jobTask.Properties.DockerHost,
								},
							},
							VolumeMounts: getVolumeMounts(workflowCtx.ConfigMapMountDir),
							Resources:    getResourceRequirements(resReq, resReqSpec),

							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							TerminationMessagePath:   job.JobTerminationFile,
						},
						// {
						// 	ImagePullPolicy: corev1.PullIfNotPresent,
						// 	Name:            "log",
						// 	Image:           BusyBoxImage,
						// 	VolumeMounts:    getVolumeMounts(workflowCtx.ConfigMapMountDir),
						// 	Command:         []string{"/bin/sh", "-c"},
						// 	Args:            []string{tailLogCommand},
						// 	Lifecycle:       &corev1.Lifecycle{},
						// },
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
	resp = append(resp, corev1.VolumeMount{
		Name:      "zadig-context",
		MountPath: ZadigContextDir,
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
	resp = append(resp, corev1.Volume{
		Name: "zadig-context",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
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

func waitJobEndWithFile(ctx context.Context, taskTimeout int, namespace, jobName string, checkFile bool, kubeClient crClient.Client, clientset kubernetes.Interface, restConfig *rest.Config, xl *zap.SugaredLogger) (status config.Status) {
	xl.Infof("wait job to start: %s/%s", namespace, jobName)
	timeout := time.After(time.Duration(taskTimeout) * time.Minute)
	podTimeout := time.After(120 * time.Second)

	xl.Infof("Timeout of preparing Pod: %s. Timeout of running task: %s.", 120*time.Second, time.Duration(taskTimeout)*time.Minute)

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
				if !checkFile {
					// break only break the select{}, not the outside for{}
					break
				}

				pods, err := getter.ListPods(namespace, labels.Set{"job-name": jobName}.AsSelector(), kubeClient)
				if err != nil {
					xl.Errorf("failed to find pod with label job-name=%s %v", jobName, err)
					return config.StatusFailed
				}
				var done, exists bool
				var jobStatus commontypes.JobStatus

				for _, pod := range pods {
					ipod := wrapper.Pod(pod)
					if ipod.Pending() {
						continue
					}
					if ipod.Failed() {
						return config.StatusFailed
					}
					if !ipod.Finished() {
						jobStatus, exists, err = checkDogFoodExistsInContainer(clientset, restConfig, namespace, ipod.Name, ipod.ContainerNames()[0])
						if err != nil {
							// Note:
							// Currently, this error indicates "the target Pod cannot be accessed" or "the target Pod can be accessed, but the dog food file does not exist".
							// In these two scenarios, `Info` is used to print logs because they are not business semantic exceptions.
							xl.Infof("Result of checking dog food file %s: %s", pods[0].Name, err)
							break
						}
						if !exists {
							break
						}
					}
					done = true
				}

				if done {
					xl.Infof("Dog food is found, stop to wait %s. Job status: %s.", job.Name, jobStatus)

					switch jobStatus {
					case commontypes.JobFail:
						return config.StatusFailed
					default:
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

func getJobOutput(namespace, containerName string, jobLabel *JobLabel, kubeClient crClient.Client) ([]*job.JobOutput, error) {
	resp := []*job.JobOutput{}
	ls := getJobLabels(jobLabel)
	pods, err := getter.ListPods(namespace, labels.Set(ls).AsSelector(), kubeClient)
	if err != nil {
		return resp, err
	}
	for _, pod := range pods {
		ipod := wrapper.Pod(pod)
		// only collect succeeed job outputs.
		if !ipod.Succeeded() {
			return resp, nil
		}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name != ls[containerName] {
				continue
			}
			if containerStatus.State.Terminated != nil && len(containerStatus.State.Terminated.Message) != 0 {
				if err := json.Unmarshal([]byte(containerStatus.State.Terminated.Message), &resp); err != nil {
					return resp, err
				}
				return resp, nil
			}
		}
	}
	return resp, nil
}

func saveContainerLog(job *commonmodels.JobTask, workflowName string, taskID int64, jobLabel *JobLabel, kubeClient crClient.Client) error {
	selector := labels.Set(getJobLabels(jobLabel)).AsSelector()
	pods, err := getter.ListPods(job.Properties.Namespace, selector, kubeClient)
	if err != nil {
		return err
	}

	if len(pods) < 1 {
		return fmt.Errorf("no pod found with selector: %s", selector)
	}

	if len(pods[0].Status.ContainerStatuses) < 1 {
		return fmt.Errorf("no cotainer statuses : %s", selector)
	}

	buf := new(bytes.Buffer)
	// 默认取第一个build job的第一个pod的第一个container的日志
	sort.SliceStable(pods, func(i, j int) bool {
		return pods[i].CreationTimestamp.Before(&pods[j].CreationTimestamp)
	})

	clientSet, err := kubeclient.GetClientset(config.HubServerAddress(), job.Properties.ClusterID)
	if err != nil {
		log.Errorf("saveContainerLog, get client set error: %s", err)
		return err
	}

	if err := containerlog.GetContainerLogs(job.Properties.Namespace, pods[0].Name, pods[0].Spec.Containers[0].Name, false, int64(0), buf, clientSet); err != nil {
		return fmt.Errorf("failed to get container logs: %s", err)
	}

	store, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return fmt.Errorf("failed to get default s3 storage: %s", err)
	}

	if tempFileName, err := util.GenerateTmpFile(); err == nil {
		defer func() {
			_ = os.Remove(tempFileName)
		}()
		if err = saveFile(buf, tempFileName); err == nil {

			if store.Subfolder != "" {
				store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, strings.ToLower(workflowName), taskID, "log")
			} else {
				store.Subfolder = fmt.Sprintf("%s/%d/%s", strings.ToLower(workflowName), taskID, "log")
			}
			forcedPathStyle := true
			if store.Provider == setting.ProviderSourceAli {
				forcedPathStyle = false
			}
			s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
			if err != nil {
				return fmt.Errorf("saveContainerLog s3 create client error: %v", err)
			}
			fileName := strings.Replace(strings.ToLower(job.Name), "_", "-", -1)
			objectKey := GetObjectPath(store.Subfolder, fileName+".log")
			if err = s3client.Upload(
				store.Bucket,
				tempFileName,
				objectKey,
			); err != nil {
				return fmt.Errorf("saveContainerLog s3 Upload error: %v", err)
			}
		} else {
			return fmt.Errorf("saveContainerLog saveFile error: %v", err)
		}
	} else {
		return fmt.Errorf("saveContainerLog GenerateTmpFile error: %v", err)
	}
	return nil
}

func GetObjectPath(subFolder, name string) string {
	// target should not be started with /
	if subFolder != "" {
		return strings.TrimLeft(filepath.Join(subFolder, name), "/")
	}

	return strings.TrimLeft(name, "/")
}

func checkDogFoodExistsInContainer(clientset kubernetes.Interface, restConfig *rest.Config, namespace, pod, container string) (commontypes.JobStatus, bool, error) {
	stdout, _, success, err := podexec.KubeExec(clientset, restConfig, podexec.ExecOptions{
		Command:       []string{"/bin/sh", "-c", fmt.Sprintf("test -f %[1]s && cat %[1]s", setting.DogFood)},
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	})

	return commontypes.JobStatus(stdout), success, err
}
