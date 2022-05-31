package jobcontroller

import (
	"strings"
	"sync"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

func BuildJobExcutorContext(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, globalContext *sync.Map) *JobContext {
	var envVars, secretEnvVars []string
	for _, env := range job.Properties.Args {
		if env.IsCredential {
			secretEnvVars = append(secretEnvVars, strings.Join([]string{env.Key, env.Value}, "="))
			continue
		}
		envVars = append(envVars, strings.Join([]string{env.Key, env.Value}, "="))
	}
	return &JobContext{
		Name:         job.Name,
		Envs:         envVars,
		SecretEnvs:   secretEnvVars,
		WorkflowName: workflowCtx.WorkflowName,
		TaskID:       workflowCtx.TaskID,
		Steps:        job.Steps,
	}
}
