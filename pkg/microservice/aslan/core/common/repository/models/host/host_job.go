package host

import "go.mongodb.org/mongo-driver/bson/primitive"

type HostJob struct {
	ID           primitive.ObjectID `bson:"_id"                    json:"id"`
	ProjectName  string             `bson:"project_name"           json:"project_name"`
	WorkflowName string             `bson:"workflow_name"          json:"workflow_name"`
	TaskID       int64              `bson:"task_id"                json:"task_id"`
	JobName      string             `bson:"job_name"               json:"job_name"`
	Name         string             `bson:"name"                   json:"name"`
	Workspace    string             `bson:"workspace"              json:"workspace"`
	Envs         EnvVar             `bson:"envs"                   json:"envs"`
	SecretEnvs   EnvVar             `bson:"secret_envs"            json:"secret_envs"`
	Paths        string             `bson:"paths"                  json:"paths"`
	Steps        []*Step            `bson:"steps"                  json:"steps"`
	Outputs      []string           `bson:"outputs"                json:"outputs"`
	Status       string             `bson:"status"                 json:"status"`
	HostID       string             `bson:"host_id"                json:"host_id"`
	StartTime    int64              `bson:"start_time"             json:"start_time"`
	EndTime      int64              `bson:"end_time"               json:"end_time"`
	Error        string             `bson:"error"                  json:"error"`
	HostTags     []string           `bson:"host_tags"              json:"host_tags"`
	HostName     []string           `bson:"host_name"              json:"host_name"`
}

type Step struct {
	Name      string      `json:"name"`
	StepType  string      `json:"type"`
	Onfailure bool        `json:"on_failure"`
	Spec      interface{} `json:"spec"`
}

type EnvVar []string

type JobOutput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ReportJobParameters struct {
	JobID     string
	Status    string
	Error     error
	Log       string
	StartTime int64
	EndTime   int64
}

func (HostJob) TableName() string {
	return "host_job"
}
