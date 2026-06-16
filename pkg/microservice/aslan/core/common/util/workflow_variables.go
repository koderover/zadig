package util

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func BuildPayloadVariables(rawPayload string) []*commonmodels.KeyVal {
	if rawPayload == "" {
		return nil
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return nil
	}

	resp := make([]*commonmodels.KeyVal, 0)
	flattenPayloadValue("payload", payload, &resp)
	return resp
}

func flattenPayloadValue(prefix string, value interface{}, resp *[]*commonmodels.KeyVal) {
	switch val := value.(type) {
	case map[string]interface{}:
		for key, item := range val {
			flattenPayloadValue(prefix+"."+key, item, resp)
		}
	case []interface{}:
		for index, item := range val {
			flattenPayloadValue(fmt.Sprintf("%s.%d", prefix, index), item, resp)
		}
	case string:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: val, IsCredential: false})
	case float64:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: strconv.FormatFloat(val, 'f', -1, 64), IsCredential: false})
	case bool:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: strconv.FormatBool(val), IsCredential: false})
	case nil:
		return
	default:
		*resp = append(*resp, &commonmodels.KeyVal{Key: prefix, Value: fmt.Sprint(val), IsCredential: false})
	}
}

func BuildWorkflowSystemVariableKVs(workflow *commonmodels.WorkflowV4, projectName, projectDisplayName string, taskID int64, creator, account, uid string, now time.Time) []*commonmodels.KeyVal {
	if workflow == nil {
		return nil
	}

	resp := []*commonmodels.KeyVal{
		{Key: "project", Value: projectName, IsCredential: false},
		{Key: "project.id", Value: projectName, IsCredential: false},
		{Key: "project.name", Value: projectDisplayName, IsCredential: false},
		{Key: "workflow.id", Value: workflow.Name, IsCredential: false},
		{Key: "workflow.name", Value: workflow.DisplayName, IsCredential: false},
		{Key: "workflow.task.id", Value: fmt.Sprintf("%d", taskID), IsCredential: false},
		{Key: "workflow.task.creator", Value: creator, IsCredential: false},
		{Key: "workflow.task.creator.id", Value: account, IsCredential: false},
		{Key: "workflow.task.creator.userId", Value: uid, IsCredential: false},
		{Key: "workflow.task.timestamp", Value: fmt.Sprintf("%d", now.Unix()), IsCredential: false},
		{Key: "workflow.task.datetime", Value: now.Format(time.DateTime), IsCredential: false},
		{
			Key:          "workflow.task.url",
			Value:        fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", configbase.SystemAddress(), projectName, workflow.Name, taskID, url.QueryEscape(workflow.DisplayName)),
			IsCredential: false,
		},
	}

	for _, param := range workflow.Params {
		if param == nil {
			continue
		}
		value := param.Value
		if param.ParamsType == string(commonmodels.MultiSelectType) {
			value = strings.Join(param.ChoiceValue, ",")
		} else if param.ParamsType == string(commonmodels.FileType) {
			continue
		}
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"workflow", "params", param.Name}, "."),
			Value:        value,
			IsCredential: false,
		})
	}
	if workflow.HookPayload != nil {
		resp = append(resp, BuildWorkflowTriggerVariableKVs(workflow.HookPayload)...)
		resp = append(resp, BuildPayloadVariables(workflow.HookPayload.RawPayload)...)
	}

	return resp
}

func BuildWorkflowTriggerVariableKVs(hookPayload *commonmodels.HookPayload) []*commonmodels.KeyVal {
	if hookPayload == nil {
		return nil
	}

	resp := make([]*commonmodels.KeyVal, 0, 7)
	appendIfNotEmpty := func(key, value string) {
		if value == "" {
			return
		}
		resp = append(resp, &commonmodels.KeyVal{Key: key, Value: value, IsCredential: false})
	}

	appendIfNotEmpty("workflow.trigger.branch", hookPayload.Branch)
	appendIfNotEmpty("workflow.trigger.target_branch", hookPayload.TargetBranch)
	appendIfNotEmpty("workflow.trigger.pr", hookPayload.MergeRequestID)
	appendIfNotEmpty("workflow.trigger.commit_id", hookPayload.CommitID)
	appendIfNotEmpty("workflow.trigger.commit_message", hookPayload.CommitMessage)
	appendIfNotEmpty("workflow.trigger.committer", hookPayload.Committer)
	appendIfNotEmpty("workflow.trigger.event", hookPayload.EventType)

	return resp
}

func BuildWorkflowRuntimeVariableKVs(workflow *commonmodels.WorkflowV4, projectName, projectDisplayName string, taskID int64, creator, account, uid string, now time.Time) []*commonmodels.KeyVal {
	return BuildWorkflowSystemVariableKVs(workflow, projectName, projectDisplayName, taskID, creator, account, uid, now)
}

func KeyValsToMap(kvs []*commonmodels.KeyVal) map[string]string {
	resp := make(map[string]string)
	for _, kv := range kvs {
		if kv == nil || kv.Key == "" || kv.GetValue() == "" {
			continue
		}
		resp[kv.Key] = kv.GetValue()
	}
	return resp
}
