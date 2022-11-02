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
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/multicluster/service"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/containerlog"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/podexec"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	commontypes "github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/job"
	"github.com/koderover/zadig/pkg/util"
)

const (
	BusyBoxImage       = "koderover.tencentcloudcr.com/koderover-public/busybox:latest"
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
	JobName string
	JobType string
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
	// reaperImage: koderover.tencentcloudcr.com/koderover-public/build-base:${BuildOS}-amd64
	// buildOS: focal xenial bionic
	jobImage := strings.ReplaceAll(config.ReaperImage(), "${BuildOS}", buildOS)
	// for custom image, buildOS represents the exact custom image
	if imageFrom == setting.ImageFromCustom {
		jobImage = buildOS
	}
	return jobImage
}

func buildTolerations(clusterConfig *commonmodels.AdvancedConfig) []corev1.Toleration {
	ret := make([]corev1.Toleration, 0)
	if clusterConfig == nil || len(clusterConfig.Tolerations) == 0 {
		return ret
	}

	err := yaml.Unmarshal([]byte(clusterConfig.Tolerations), &ret)
	if err != nil {
		log.Errorf("failed to parse toleration config, err: %s", err)
		return nil
	}
	return ret
}

func addNodeAffinity(clusterConfig *commonmodels.AdvancedConfig) *corev1.Affinity {
	if clusterConfig == nil || len(clusterConfig.NodeLabels) == 0 {
		return nil
	}

	switch clusterConfig.Strategy {
	case setting.RequiredSchedule:
		nodeSelectorTerms := make([]corev1.NodeSelectorTerm, 0)
		for _, nodeLabel := range clusterConfig.NodeLabels {
			var matchExpressions []corev1.NodeSelectorRequirement
			matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
				Key:      nodeLabel.Key,
				Operator: nodeLabel.Operator,
				Values:   nodeLabel.Value,
			})
			nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
				MatchExpressions: matchExpressions,
			})
		}

		affinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: nodeSelectorTerms,
				},
			},
		}
		return affinity
	case setting.PreferredSchedule:
		preferredScheduleTerms := make([]corev1.PreferredSchedulingTerm, 0)
		for _, nodeLabel := range clusterConfig.NodeLabels {
			var matchExpressions []corev1.NodeSelectorRequirement
			matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
				Key:      nodeLabel.Key,
				Operator: nodeLabel.Operator,
				Values:   nodeLabel.Value,
			})
			nodeSelectorTerm := corev1.NodeSelectorTerm{
				MatchExpressions: matchExpressions,
			}
			preferredScheduleTerms = append(preferredScheduleTerms, corev1.PreferredSchedulingTerm{
				Weight:     10,
				Preference: nodeSelectorTerm,
			})
		}
		affinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: preferredScheduleTerms,
			},
		}
		return affinity
	default:
		return nil
	}
}

func buildPlainJob(jobName string, resReq setting.Request, resReqSpec setting.RequestSpec, jobTask *commonmodels.JobTask, jobTaskSpec *commonmodels.JobTaskPluginSpec, workflowCtx *commonmodels.WorkflowTaskCtx) (*batchv1.Job, error) {
	collectJobOutput := `OLD_IFS=$IFS
export IFS=","
files='%s'
outputs='%s'
file_arr=($files)
output_arr=($outputs)
IFS="$OLD_IFS"
result="{"
for i in ${!file_arr[@]};
do
	file_value=$(cat ${file_arr[$i]})
	output_value=${output_arr[$i]}
	result="$result\"$output_value\":\"$file_value\","
done
result=$(sed 's/,$/}/' <<< $result)
echo $result > %s
`
	files := []string{}
	outputs := []string{}
	for _, output := range jobTask.Outputs {
		outputFile := path.Join(job.JobOutputDir, output.Name)
		files = append(files, outputFile)
		outputs = append(outputs, output.Name)
	}
	collectJobOutputCommand := fmt.Sprintf(collectJobOutput, strings.Join(files, ","), strings.Join(outputs, ","), job.JobTerminationFile)

	labels := getJobLabels(&JobLabel{
		JobType: string(jobTask.JobType),
		JobName: jobTask.K8sJobName,
	})

	ImagePullSecrets, err := getImagePullSecrets(jobTaskSpec.Properties.Registries)
	if err != nil {
		return nil, err
	}

	envs := []corev1.EnvVar{}
	for _, env := range jobTaskSpec.Plugin.Envs {
		envs = append(envs, corev1.EnvVar{Name: env.Name, Value: env.Value})
	}

	clusterID := jobTaskSpec.Properties.ClusterID
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	// fetch cluster to get nodeAffinity and tolerations
	targetCluster, err := service.GetCluster(clusterID, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to find target cluster %s, err: %s", clusterID, err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Completions:  int32Ptr(1),
			Parallelism:  int32Ptr(1),
			BackoffLimit: int32Ptr(0),
			// in case finished zombie job not cleaned up by zadig
			TTLSecondsAfterFinished: int32Ptr(3600),
			// in case zombie job never stop
			ActiveDeadlineSeconds: int64Ptr(jobTaskSpec.Properties.Timeout*60 + 3600),
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
							Name:            jobTask.Name,
							Image:           jobTaskSpec.Plugin.Image,
							Args:            jobTaskSpec.Plugin.Args,
							Command:         jobTaskSpec.Plugin.Cmds,
							Lifecycle: &corev1.Lifecycle{
								PostStart: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", fmt.Sprintf("mkdir -p %s", job.JobOutputDir)},
									},
								},
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", collectJobOutputCommand},
									},
								},
							},
							Env: envs,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "zadig-context",
									MountPath: ZadigContextDir,
								},
							},
							Resources: getResourceRequirements(resReq, resReqSpec),

							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							TerminationMessagePath:   job.JobTerminationFile,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "zadig-context",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					Tolerations: buildTolerations(targetCluster.AdvancedConfig),
					Affinity:    addNodeAffinity(targetCluster.AdvancedConfig),
				},
			},
		},
	}
	return job, nil
}

func buildJob(jobType, jobImage, jobName, clusterID, currentNamespace string, resReq setting.Request, resReqSpec setting.RequestSpec, jobTask *commonmodels.JobTask, jobTaskSpec *commonmodels.JobTaskFreestyleSpec, workflowCtx *commonmodels.WorkflowTaskCtx, registries []*task.RegistryNamespace) (*batchv1.Job, error) {
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

	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	// fetch cluster to get nodeAffinity and tolerations
	targetCluster, err := service.GetCluster(clusterID, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to find target cluster %s, err: %s", clusterID, err)
	}

	jobExecutorBootingScript = fmt.Sprintf("curl -m 10 --retry-delay 3 --retry 3 -sSL %s -o reaper && chmod +x reaper && mv reaper /usr/local/bin && /usr/local/bin/reaper", jobExecutorBinaryFile)

	labels := getJobLabels(&JobLabel{
		JobType: string(jobType),
		JobName: jobTask.K8sJobName,
	})

	ImagePullSecrets, err := getImagePullSecrets(jobTaskSpec.Properties.Registries)
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Completions:  int32Ptr(1),
			Parallelism:  int32Ptr(1),
			BackoffLimit: int32Ptr(0),
			// in case finished zombie job not cleaned up by zadig
			TTLSecondsAfterFinished: int32Ptr(3600),
			// in case zombie job never stop
			ActiveDeadlineSeconds: int64Ptr(jobTaskSpec.Properties.Timeout*60 + 3600),
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
									Value: jobTaskSpec.Properties.DockerHost,
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
					Volumes:     getVolumes(jobName),
					Tolerations: buildTolerations(targetCluster.AdvancedConfig),
					Affinity:    addNodeAffinity(targetCluster.AdvancedConfig),
				},
			},
		},
	}

	if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == commontypes.NFSMedium {
		volumeName := "build-cache"
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: jobTaskSpec.Properties.Cache.NFSProperties.PVC,
				},
			},
		})

		mountPath := jobTaskSpec.Properties.CacheUserDir
		if jobTaskSpec.Properties.CacheDirType == commontypes.WorkspaceCacheDir {
			mountPath = workflowCtx.Workspace
		}

		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
			SubPath:   jobTaskSpec.Properties.Cache.NFSProperties.Subpath,
		})
	}

	// if affinity := addNodeAffinity(clusterID, pipelineTask.ConfigPayload.K8SClusters); affinity != nil {
	// 	job.Spec.Template.Spec.Affinity = affinity
	// }

	return job, nil
}

func getImagePullSecrets(registries []*commonmodels.RegistryNamespace) ([]corev1.LocalObjectReference, error) {
	ImagePullSecrets := []corev1.LocalObjectReference{
		{
			Name: setting.DefaultImagePullSecret,
		},
	}
	for _, reg := range registries {
		secretName, err := kube.GenRegistrySecretName(reg)
		if err != nil {
			return ImagePullSecrets, fmt.Errorf("failed to generate registry secret name: %s", err)
		}

		secret := corev1.LocalObjectReference{
			Name: secretName,
		}
		ImagePullSecrets = append(ImagePullSecrets, secret)
	}
	return ImagePullSecrets, nil
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

// generateResourceRequirements
// cpu Request:Limit=1:4
// memory default Request:Limit=1:4 ; if memoryLimit>= 8Gi,Request:Limit=1:8
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

	limits := corev1.ResourceList{}
	requests := corev1.ResourceList{}

	if reqSpec.CpuLimit > 0 {
		cpuReqInt := reqSpec.CpuLimit / 4
		if cpuReqInt < 1 {
			cpuReqInt = 1
		}
		limits[corev1.ResourceCPU] = resource.MustParse(strconv.Itoa(reqSpec.CpuLimit) + setting.CpuUintM)
		requests[corev1.ResourceCPU] = resource.MustParse(strconv.Itoa(cpuReqInt) + setting.CpuUintM)
	}

	if reqSpec.MemoryLimit > 0 {
		memoryReqInt := reqSpec.MemoryLimit / 4
		if memoryReqInt >= 2*1024 {
			memoryReqInt = memoryReqInt / 2
		}
		if memoryReqInt < 1 {
			memoryReqInt = 1
		}
		limits[corev1.ResourceMemory] = resource.MustParse(strconv.Itoa(reqSpec.MemoryLimit) + setting.MemoryUintMi)
		requests[corev1.ResourceMemory] = resource.MustParse(strconv.Itoa(memoryReqInt) + setting.MemoryUintMi)
	}

	// add gpu limit
	if len(reqSpec.GpuLimit) > 0 {
		reqSpec.GpuLimit = strings.ReplaceAll(reqSpec.GpuLimit, " ", "")
		requestPair := strings.Split(reqSpec.GpuLimit, ":")
		if len(requestPair) == 2 {
			limits[corev1.ResourceName(requestPair[0])] = resource.MustParse(requestPair[1])
		}
	}

	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
}

func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }

func waitPlainJobEnd(ctx context.Context, taskTimeout int, namespace, jobName string, kubeClient crClient.Client, xl *zap.SugaredLogger) config.Status {
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
	// wait for the job to end.
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

			if job.Status.Succeeded != 0 {
				return config.StatusPassed
			}
			if job.Status.Failed != 0 {
				return config.StatusFailed
			}
		}

		time.Sleep(time.Second * 1)
	}

}

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

func saveContainerLog(namespace, clusterID, workflowName, jobName string, taskID int64, jobLabel *JobLabel, kubeClient crClient.Client) error {
	selector := labels.Set(getJobLabels(jobLabel)).AsSelector()
	pods, err := getter.ListPods(namespace, selector, kubeClient)
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

	clientSet, err := kubeclient.GetClientset(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("saveContainerLog, get client set error: %s", err)
		return err
	}

	if err := containerlog.GetContainerLogs(namespace, pods[0].Name, pods[0].Spec.Containers[0].Name, false, int64(0), buf, clientSet); err != nil {
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
			fileName := strings.Replace(strings.ToLower(jobName), "_", "-", -1)
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

func waitDeploymentReady(ctx context.Context, deploymentName, namespace string, timout int64, kubeClient crClient.Client, logger *zap.SugaredLogger) (config.Status, error) {
	timeout := time.After(time.Duration(timout) * time.Second)
	tick := time.NewTicker(time.Second * 2)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled, errors.New("job was cancelled")

		case <-timeout:
			msg := fmt.Sprintf("timeout waiting for the deployment: %s to run", deploymentName)
			return config.StatusTimeout, errors.New(msg)

		case <-tick.C:
			d, found, err := getter.GetDeployment(namespace, deploymentName, kubeClient)
			if err != nil || !found {
				logger.Errorf(
					"failed to check deployment ready status %s/%s - %v",
					namespace,
					deploymentName,
					err,
				)
			} else {
				if wrapper.Deployment(d).Ready() {
					return config.StatusRunning, nil
				}
			}
		}
	}
}
