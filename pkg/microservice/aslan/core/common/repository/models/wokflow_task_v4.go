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

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

type WorkflowTask struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	TaskID              int64              `bson:"task_id"                   json:"task_id"`
	WorkflowName        string             `bson:"workflow_name"             json:"workflow_name"`
	WorkflowDisplayName string             `bson:"workflow_display_name"     json:"workflow_display_name"`
	Params              []*Param           `bson:"params"                    json:"params"`
	WorkflowArgs        *WorkflowV4        `bson:"workflow_args"             json:"workflow_args"`
	OriginWorkflowArgs  *WorkflowV4        `bson:"origin_workflow_args"      json:"origin_workflow_args"`
	KeyVals             []*KeyVal          `bson:"key_vals"                  json:"key_vals"`
	GlobalContext       map[string]string  `bson:"global_context"            json:"global_context"`
	Status              config.Status      `bson:"status"                    json:"status,omitempty"`
	TaskCreator         string             `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker         string             `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime          int64              `bson:"create_time"               json:"create_time,omitempty"`
	StartTime           int64              `bson:"start_time"                json:"start_time,omitempty"`
	EndTime             int64              `bson:"end_time"                  json:"end_time,omitempty"`
	Stages              []*StageTask       `bson:"stages"                    json:"stages"`
	ProjectName         string             `bson:"project_name,omitempty"    json:"project_name,omitempty"`
	IsDeleted           bool               `bson:"is_deleted"                json:"is_deleted"`
	IsArchived          bool               `bson:"is_archived"               json:"is_archived"`
	Error               string             `bson:"error,omitempty"           json:"error,omitempty"`
	IsRestart           bool               `bson:"is_restart"                json:"is_restart"`
	MultiRun            bool               `bson:"multi_run"                 json:"multi_run"`
}

func (WorkflowTask) TableName() string {
	return "workflow_task"
}

type StageTask struct {
	Name      string        `bson:"name"          json:"name"`
	Status    config.Status `bson:"status"        json:"status"`
	StartTime int64         `bson:"start_time"    json:"start_time,omitempty"`
	EndTime   int64         `bson:"end_time"      json:"end_time,omitempty"`
	Parallel  bool          `bson:"parallel"      json:"parallel"`
	Approval  *Approval     `bson:"approval"      json:"approval"`
	Jobs      []*JobTask    `bson:"jobs"          json:"jobs"`
	Error     string        `bson:"error"         json:"error"`
}

type JobTask struct {
	Name       string        `bson:"name"                json:"name"`
	K8sJobName string        `bson:"k8s_job_name"        json:"k8s_job_name"`
	JobType    string        `bson:"type"                json:"type"`
	Status     config.Status `bson:"status"              json:"status"`
	StartTime  int64         `bson:"start_time"          json:"start_time,omitempty"`
	EndTime    int64         `bson:"end_time"            json:"end_time,omitempty"`
	Error      string        `bson:"error"               json:"error"`
	Timeout    int64         `bson:"timeout"             json:"timeout"`
	Retry      int64         `bson:"retry"               json:"retry"`
	Spec       interface{}   `bson:"spec"                json:"spec"`
	Outputs    []*Output     `bson:"outputs"             json:"outputs"`
}

type JobTaskCustomDeploySpec struct {
	Namespace          string     `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	ClusterID          string     `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Timeout            int64      `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	WorkloadType       string     `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
	WorkloadName       string     `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	ContainerName      string     `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image              string     `bson:"image"                  json:"image"                 yaml:"image"`
	SkipCheckRunStatus bool       `bson:"skip_check_run_status"  json:"skip_check_run_status" yaml:"skip_check_run_status"`
	ReplaceResources   []Resource `bson:"replace_resources"      json:"replace_resources"     yaml:"replace_resources"`
}

type JobTaskDeploySpec struct {
	Env                string     `bson:"env"                              json:"env"                                 yaml:"env"`
	ServiceName        string     `bson:"service_name"                     json:"service_name"                        yaml:"service_name"`
	ServiceType        string     `bson:"service_type"                     json:"service_type"                        yaml:"service_type"`
	ServiceModule      string     `bson:"service_module"                   json:"service_module"                      yaml:"service_module"`
	SkipCheckRunStatus bool       `bson:"skip_check_run_status"            json:"skip_check_run_status"               yaml:"skip_check_run_status"`
	Image              string     `bson:"image"                            json:"image"                               yaml:"image"`
	ClusterID          string     `bson:"cluster_id"                       json:"cluster_id"                          yaml:"cluster_id"`
	Timeout            int        `bson:"timeout"                          json:"timeout"                             yaml:"timeout"`
	ReplaceResources   []Resource `bson:"replace_resources"                json:"replace_resources"                   yaml:"replace_resources"`
}

type Resource struct {
	Name      string `bson:"name"                              json:"name"                                 yaml:"name"`
	Kind      string `bson:"kind"                              json:"kind"                                 yaml:"kind"`
	Container string `bson:"container"                         json:"container"                            yaml:"container"`
	Origin    string `bson:"origin"                            json:"origin"                               yaml:"origin"`
}

type JobTaskHelmDeploySpec struct {
	Env                string                   `bson:"env"                              json:"env"                                 yaml:"env"`
	ServiceName        string                   `bson:"service_name"                     json:"service_name"                        yaml:"service_name"`
	ServiceType        string                   `bson:"service_type"                     json:"service_type"                        yaml:"service_type"`
	SkipCheckRunStatus bool                     `bson:"skip_check_run_status"            json:"skip_check_run_status"               yaml:"skip_check_run_status"`
	ImageAndModules    []*ImageAndServiceModule `bson:"image_and_service_modules"        json:"image_and_service_modules"           yaml:"image_and_service_modules"`
	ClusterID          string                   `bson:"cluster_id"                       json:"cluster_id"                          yaml:"cluster_id"`
	ReleaseName        string                   `bson:"release_name"                     json:"release_name"                        yaml:"release_name"`
	Timeout            int                      `bson:"timeout"                          json:"timeout"                             yaml:"timeout"`
	ReplaceResources   []Resource               `bson:"replace_resources"                json:"replace_resources"                   yaml:"replace_resources"`
}

type ImageAndServiceModule struct {
	ServiceModule string `bson:"service_module"                     json:"service_module"                        yaml:"service_module"`
	Image         string `bson:"image"                              json:"image"                                 yaml:"image"`
}

type JobTaskFreestyleSpec struct {
	Properties JobProperties `bson:"properties"          json:"properties"        yaml:"properties"`
	Steps      []*StepTask   `bson:"steps"               json:"steps"             yaml:"steps"`
}

type JobTaskPluginSpec struct {
	Properties JobProperties   `bson:"properties"          json:"properties"        yaml:"properties"`
	Plugin     *PluginTemplate `bson:"plugin"              json:"plugin"            yaml:"plugin"`
}

type JobTaskBlueGreenDeploySpec struct {
	ClusterID        string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	// unit is minute.
	DeployTimeout      int64   `bson:"deploy_timeout"              json:"deploy_timeout"             yaml:"deploy_timeout"`
	K8sServiceName     string  `bson:"k8s_service_name"            json:"k8s_service_name"           yaml:"k8s_service_name"`
	BlueK8sServiceName string  `bson:"blue_k8s_service_name"       json:"blue_k8s_service_name"      yaml:"blue_k8s_service_name"`
	WorkloadType       string  `bson:"workload_type"               json:"workload_type"              yaml:"workload_type"`
	WorkloadName       string  `bson:"workload_name"               json:"workload_name"              yaml:"workload_name"`
	BlueWorkloadName   string  `bson:"blue_workload_name"          json:"blue_workload_name"         yaml:"blue_workload_name"`
	ContainerName      string  `bson:"container_name"              json:"container_name"             yaml:"container_name"`
	Version            string  `bson:"version"                     json:"version"                    yaml:"version"`
	Image              string  `bson:"image"                       json:"image"                      yaml:"image"`
	FirstDeploy        bool    `bson:"first_deploy"                json:"first_deploy"               yaml:"first_deploy"`
	Events             *Events `bson:"events"                      json:"events"                     yaml:"events"`
}

type JobTaskBlueGreenReleaseSpec struct {
	ClusterID          string  `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace          string  `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	K8sServiceName     string  `bson:"k8s_service_name"       json:"k8s_service_name"      yaml:"k8s_service_name"`
	BlueK8sServiceName string  `bson:"blue_k8s_service_name"  json:"blue_k8s_service_name" yaml:"blue_k8s_service_name"`
	WorkloadType       string  `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
	WorkloadName       string  `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	BlueWorkloadName   string  `bson:"blue_workload_name"     json:"blue_workload_name"    yaml:"blue_workload_name"`
	Version            string  `bson:"version"                json:"version"               yaml:"version"`
	Image              string  `bson:"image"                  json:"image"                 yaml:"image"`
	ContainerName      string  `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Events             *Events `bson:"events"                 json:"events"                yaml:"events"`
}

type JobTaskCanaryDeploySpec struct {
	ClusterID        string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	// unit is minute.
	DeployTimeout      int64   `bson:"deploy_timeout"                 json:"deploy_timeout"                yaml:"deploy_timeout"`
	K8sServiceName     string  `bson:"k8s_service_name"               json:"k8s_service_name"              yaml:"k8s_service_name"`
	WorkloadType       string  `bson:"workload_type"                  json:"workload_type"                 yaml:"workload_type"`
	WorkloadName       string  `bson:"workload_name"                  json:"workload_name"                 yaml:"workload_name"`
	ContainerName      string  `bson:"container_name"                 json:"container_name"                yaml:"container_name"`
	CanaryPercentage   int     `bson:"canary_percentage"              json:"canary_percentage"             yaml:"canary_percentage"`
	CanaryReplica      int     `bson:"canary_replica"                 json:"canary_replica"                yaml:"canary_replica"`
	CanaryWorkloadName string  `bson:"canary_workload_name"           json:"canary_workload_name"          yaml:"canary_workload_name"`
	Version            string  `bson:"version"                        json:"version"                       yaml:"version"`
	Image              string  `bson:"image"                          json:"image"                         yaml:"image"`
	Events             *Events `bson:"events"                         json:"events"                        yaml:"events"`
}

type JobTaskCanaryReleaseSpec struct {
	ClusterID          string `bson:"cluster_id"             json:"cluster_id"             yaml:"cluster_id"`
	Namespace          string `bson:"namespace"              json:"namespace"              yaml:"namespace"`
	K8sServiceName     string `bson:"k8s_service_name"       json:"k8s_service_name"       yaml:"k8s_service_name"`
	WorkloadType       string `bson:"workload_type"          json:"workload_type"          yaml:"workload_type"`
	WorkloadName       string `bson:"workload_name"          json:"workload_name"          yaml:"workload_name"`
	ContainerName      string `bson:"container_name"         json:"container_name"         yaml:"container_name"`
	Version            string `bson:"version"                json:"version"                yaml:"version"`
	Image              string `bson:"image"                  json:"image"                  yaml:"image"`
	CanaryWorkloadName string `bson:"canary_workload_name"   json:"canary_workload_name"   yaml:"canary_workload_name"`
	// unit is minute.
	ReleaseTimeout int64   `bson:"release_timeout"        json:"release_timeout"       yaml:"release_timeout"`
	Events         *Events `bson:"events"                 json:"events"                yaml:"events"`
}

type JobTaskGrayReleaseSpec struct {
	ClusterID        string `bson:"cluster_id"             json:"cluster_id"             yaml:"cluster_id"`
	ClusterName      string `bson:"cluster_name"           json:"cluster_name"           yaml:"cluster_name"`
	Namespace        string `bson:"namespace"              json:"namespace"              yaml:"namespace"`
	WorkloadType     string `bson:"workload_type"          json:"workload_type"          yaml:"workload_type"`
	WorkloadName     string `bson:"workload_name"          json:"workload_name"          yaml:"workload_name"`
	ContainerName    string `bson:"container_name"         json:"container_name"         yaml:"container_name"`
	FirstJob         bool   `bson:"first_job"              json:"first_job"              yaml:"first_job"`
	Image            string `bson:"image"                  json:"image"                  yaml:"image"`
	GrayWorkloadName string `bson:"gray_workload_name"     json:"gray_workload_name"     yaml:"gray_workload_name"`
	// unit is minute.
	DeployTimeout int64   `bson:"deploy_timeout"        json:"deploy_timeout"       yaml:"deploy_timeout"`
	GrayScale     int     `bson:"gray_scale"            json:"gray_scale"           yaml:"gray_scale"`
	TotalReplica  int     `bson:"total_replica"         json:"total_replica"        yaml:"total_replica"`
	GrayReplica   int     `bson:"gray_replica"          json:"gray_replica"         yaml:"gray_replica"`
	Events        *Events `bson:"events"                json:"events"               yaml:"events"`
}

type JobTaskGrayRollbackSpec struct {
	ClusterID        string `bson:"cluster_id"             json:"cluster_id"             yaml:"cluster_id"`
	ClusterName      string `bson:"cluster_name"           json:"cluster_name"           yaml:"cluster_name"`
	Namespace        string `bson:"namespace"              json:"namespace"              yaml:"namespace"`
	WorkloadType     string `bson:"workload_type"          json:"workload_type"          yaml:"workload_type"`
	WorkloadName     string `bson:"workload_name"          json:"workload_name"          yaml:"workload_name"`
	ContainerName    string `bson:"container_name"         json:"container_name"         yaml:"container_name"`
	Image            string `bson:"image"                  json:"image"                  yaml:"image"`
	GrayWorkloadName string `bson:"gray_workload_name"     json:"gray_workload_name"     yaml:"gray_workload_name"`
	// unit is minute.
	RollbackTimeout int64   `bson:"rollback_timeout"      json:"rollback_timeout"     yaml:"rollback_timeout"`
	TotalReplica    int     `bson:"total_replica"         json:"total_replica"        yaml:"total_replica"`
	Events          *Events `bson:"events"                json:"events"               yaml:"events"`
}

type JobTasK8sPatchSpec struct {
	ClusterID  string           `bson:"cluster_id"             json:"cluster_id"             yaml:"cluster_id"`
	Namespace  string           `bson:"namespace"              json:"namespace"              yaml:"namespace"`
	PatchItems []*PatchTaskItem `bson:"patch_items"            json:"patch_items"           yaml:"patch_items"`
}

type PatchTaskItem struct {
	ResourceName    string   `bson:"resource_name"                json:"resource_name"               yaml:"resource_name"`
	ResourceKind    string   `bson:"resource_kind"                json:"resource_kind"               yaml:"resource_kind"`
	ResourceGroup   string   `bson:"resource_group"               json:"resource_group"              yaml:"resource_group"`
	ResourceVersion string   `bson:"resource_version"             json:"resource_version"            yaml:"resource_version"`
	PatchContent    string   `bson:"patch_content"                json:"patch_content"               yaml:"patch_content"`
	Params          []*Param `bson:"params"                       json:"params"                      yaml:"params"`
	// support strategic-merge/merge/json
	PatchStrategy string `bson:"patch_strategy"          json:"patch_strategy"         yaml:"patch_strategy"`
	Error         string `bson:"error"                   json:"error"                  yaml:"error"`
}

type Event struct {
	EventType string `bson:"event_type"             json:"event_type"            yaml:"event_type"`
	Time      string `bson:"time"                   json:"time"                  yaml:"time"`
	Message   string `bson:"message"                json:"message"               yaml:"message"`
}

type Events []*Event

func (e *Events) Info(message string) {
	*e = append(*e, &Event{
		EventType: "info",
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Message:   message,
	})
}

func (e *Events) Error(message string) {
	*e = append(*e, &Event{
		EventType: "error",
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Message:   message,
	})
}

type StepTask struct {
	Name      string          `bson:"name"           json:"name"         yaml:"name"`
	JobName   string          `bson:"job_name"       json:"job_name"     yaml:"job_name"`
	Error     string          `bson:"error"          json:"error"        yaml:"error"`
	StepType  config.StepType `bson:"type"           json:"type"         yaml:"type"`
	Onfailure bool            `bson:"on_failure"     json:"on_failure"   yaml:"on_failure"`
	// step input params,differ form steps
	Spec interface{} `bson:"spec"           json:"spec"   yaml:"spec"`
	// step output results,like testing results,differ form steps
	Result interface{} `bson:"result"         json:"result"  yaml:"result"`
}

type WorkflowTaskCtx struct {
	WorkflowName      string
	ProjectName       string
	TaskID            int64
	DockerHost        string
	Workspace         string
	DistDir           string
	DockerMountDir    string
	ConfigMapMountDir string
	WorkflowKeyVals   []*KeyVal
	GlobalContextGet  func(key string) (string, bool)
	GlobalContextSet  func(key, value string)
	GlobalContextEach func(f func(k, v string) bool)
}
