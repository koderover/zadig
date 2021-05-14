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

package models

import (
	"bytes"
	"fmt"
	"text/template"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
)

type Notification struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty"           json:"id,omitempty"`
	CodehostId int                 `bson:"codehost_id"                  json:"codehost_id"`
	Tasks      []*NotificationTask `bson:"tasks,omitempty"              json:"tasks,omitempty"`
	PrId       int                 `bson:"pr_id"                        json:"pr_id"`
	CommentId  string              `bson:"comment_id"                   json:"comment_id"`
	ProjectId  string              `bson:"project_id"                   json:"project_id"`
	Created    int64               `bson:"create"                       json:"create"`
	BaseUri    string              `bson:"base_uri"                     json:"base_uri"`
	IsPipeline bool                `bson:"is_pipeline"                  json:"is_pipeline"`
	IsTest     bool                `bson:"is_test"                      json:"is_test"`
	ErrInfo    string              `bson:"err_info"                     json:"err_info"`
	PrTask     *PrTaskInfo         `bson:"pr_task_info,omitempty"       json:"pr_task_info,omitempty"`
	Label      string              `bson:"label"                        json:"label"  `
	Revision   string              `bson:"revision"                     json:"revision"`
}

type PrTaskInfo struct {
	EnvStatus        string `bson:"env_status,omitempty"                json:"env_status,omitempty"`
	EnvName          string `bson:"env_name,omitempty"                  json:"env_name,omitempty"`
	EnvRecyclePolicy string `bson:"env_recycle_policy,omitempty"        json:"env_recycle_policy,omitempty"`
	ProductName      string `bson:"product_name,omitempty"              json:"product_name,omitempty"`
}

type NotificationTask struct {
	ProductName  string            `bson:"product_name"    json:"product_name"`
	WorkflowName string            `bson:"workflow_name"   json:"workflow_name"`
	PipelineName string            `bson:"pipeline_name"   json:"pipeline_name"`
	TestName     string            `bson:"test_name"       json:"test_name"`
	ID           int64             `bson:"id"              json:"id"`
	Status       config.TaskStatus `bson:"status"          json:"status"`
	TestReports  []*TestSuite      `bson:"test_reports,omitempty" json:"test_reports,omitempty"`

	FirstCommented bool `json:"first_commented,omitempty" bson:"first_commented,omitempty"`
}

func (t NotificationTask) StatusVerbose() string {
	switch t.Status {
	case config.TaskStatusReady:
		return "准备中"
	case config.TaskStatusRunning:
		return "运行中"
	case config.TaskStatusCompleted:
		return "完成"
	case config.TaskStatusFailed:
		return "失败"
	case config.TaskStatusTimeout:
		return "超时"
	case config.TaskStatusCancelled:
		return "已取消"
	case config.TaskStatusPass:
		return "成功"
	default:
		return "未知"
	}
}

func (Notification) TableName() string {
	return "scm_notify"
}

func (n *Notification) ToString() string {
	return fmt.Sprintf("notification: %s #%d", n.ProjectId, n.PrId)
}

func (n *Notification) CreateCommentBody() (comment string, err error) {
	hasTest := false
	for _, task := range n.Tasks {
		if len(task.TestReports) != 0 {
			hasTest = true
			break
		}
	}

	tmplSource := ""
	if n.IsPipeline {
		if len(n.Tasks) == 0 {
			tmplSource = "触发的工作流：等待任务启动中"
		} else if !hasTest {
			tmplSource =
				"|触发的工作流|状态| \n |---|---| \n {{range .Tasks}}|[{{.PipelineName}}#{{.ID}}]({{$.BaseUri}}/v1/projects/detail/{{.ProductName}}/pipelines/single/{{.PipelineName}}/{{.ID}}) | {{if eq .StatusVerbose $.Success}} {+ {{.StatusVerbose}} +}{{else}}{- {{.StatusVerbose}} -}{{end}} | \n {{end}}"
		} else {
			tmplSource =
				"|触发的工作流|状态|测试结果（成功数/总用例数量）| \n |---|---|---| \n {{range .Tasks}}|[{{.PipelineName}}#{{.ID}}]({{$.BaseUri}}/v1/projects/detail/{{.ProductName}}/pipelines/single/{{.PipelineName}}/{{.ID}}) | {{if eq .StatusVerbose $.Success}} {+ {{.StatusVerbose}} +}{{else}}{- {{.StatusVerbose}} -}{{end}} | {{range .TestReports}}{{.Name}}: {{.Successes}}/{{.Tests}} <br> {{end}} | \n {{end}}"
		}
	} else if n.IsTest {
		if len(n.Tasks) == 0 {
			tmplSource = "触发的测试：等待任务启动中"
		} else if !hasTest {
			tmplSource =
				"|触发的测试|状态| \n |---|---| \n {{range .Tasks}}|[{{.TestName}}#{{.ID}}]({{$.BaseUri}}/v1/projects/detail/{{.ProductName}}/test/detail/function/{{.TestName}}/{{.ID}}) | {{if eq .StatusVerbose $.Success}} {+ {{.StatusVerbose}} +}{{else}}{- {{.StatusVerbose}} -}{{end}} | \n {{end}}"
		} else {
			tmplSource =
				"|触发的测试|状态|测试结果（成功数/总用例数量）| \n |---|---|---| \n {{range .Tasks}}|[{{.TestName}}#{{.ID}}]({{$.BaseUri}}/v1/projects/detail/{{.ProductName}}/test/detail/function/{{.TestName}}/{{.ID}}) | {{if eq .StatusVerbose $.Success}} {+ {{.StatusVerbose}} +}{{else}}{- {{.StatusVerbose}} -}{{end}} | {{range .TestReports}}{{.Name}}: {{.Successes}}/{{.Tests}} <br> {{end}} | \n {{end}}"
		}
	} else {
		if len(n.Tasks) == 0 {
			tmplSource = "触发的工作流：等待任务启动中"
		} else if !hasTest {
			tmplSource =
				"|触发的工作流|状态| \n |---|---| \n {{range .Tasks}}|[{{.WorkflowName}}#{{.ID}}]({{$.BaseUri}}/v1/projects/detail/{{.ProductName}}/pipelines/multi/{{.WorkflowName}}/{{.ID}}) | {{if eq .StatusVerbose $.Success}} {+ {{.StatusVerbose}} +}{{else}}{- {{.StatusVerbose}} -}{{end}} | \n {{end}}"
		} else {
			tmplSource =
				"|触发的工作流|状态|测试结果（成功数/总用例数量）| \n |---|---|---| \n {{range .Tasks}}|[{{.WorkflowName}}#{{.ID}}]({{$.BaseUri}}/v1/projects/detail/{{.ProductName}}/pipelines/multi/{{.WorkflowName}}/{{.ID}}) | {{if eq .StatusVerbose $.Success}} {+ {{.StatusVerbose}} +}{{else}}{- {{.StatusVerbose}} -}{{end}} | {{range .TestReports}}{{.Name}}: {{.Successes}}/{{.Tests}} <br> {{end}} | \n {{end}}"
		}
	}

	if n.PrTask != nil {
		if n.PrTask.EnvName != "" {
			content := fmt.Sprintf("生成基准环境：[%s]({{$.BaseUri}}/v1/projects/detail/%s/envs/detail?envName=%s) 状态：%s \n\n", n.PrTask.EnvName, n.PrTask.ProductName, n.PrTask.EnvName, n.PrTask.EnvStatus)
			tmplSource = fmt.Sprintf("%s%s", content, tmplSource)
		}

		if n.PrTask.EnvRecyclePolicy != "" {
			policyName := getEnvRecyclePolicy(n.PrTask.EnvRecyclePolicy)
			content := fmt.Sprintf("根据策略清理环境：[%s]({{$.BaseUri}}/v1/projects/detail/%s/envs/detail?envName=%s) 回收策略：%s \n\n", n.PrTask.EnvName, n.PrTask.ProductName, n.PrTask.EnvName, policyName)
			tmplSource = fmt.Sprintf("%s%s", tmplSource, content)
		}
	}

	tmpl := template.Must(template.New("comment").Parse(tmplSource))
	buffer := bytes.NewBufferString("")

	if err = tmpl.Execute(buffer, struct {
		Tasks   []*NotificationTask
		BaseUri string
		NoTask  bool
		Success string
	}{
		n.Tasks,
		n.BaseUri,
		len(n.Tasks) == 0,
		"成功",
	}); err != nil {
		return
	}

	return buffer.String(), nil
}

func getEnvRecyclePolicy(policy string) string {
	switch policy {
	case config.EnvRecyclePolicyAlways:
		return "每次销毁"
	case config.EnvRecyclePolicyTaskStatus:
		return "工作流成功之后销毁"
	case config.EnvRecyclePolicyNever:
		return "每次保留"
	default:
		return "每次保留"
	}
}
