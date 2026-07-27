package util

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

var payloadVariableRegexp = regexp.MustCompile(`{{\.(payload(\.[\p{L}\d_-]+)+)}}`)

// BuildPayloadVariables flattens webhook payload leaves into workflow variables.
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

// FilterWorkflowPayloadVariables keeps only payload variables referenced by
// runtime-rendered workflow fields.
func FilterWorkflowPayloadVariables(workflow *commonmodels.WorkflowV4) error {
	if workflow == nil || workflow.HookPayload == nil || len(workflow.HookPayload.PayloadVars) == 0 {
		return nil
	}

	runtimeFields := struct {
		Params     []*commonmodels.Param         `json:"params"`
		Stages     []*commonmodels.WorkflowStage `json:"stages"`
		NotifyCtls []*commonmodels.NotifyCtl     `json:"notify_ctls"`
	}{
		Params:     workflow.Params,
		Stages:     workflow.Stages,
		NotifyCtls: workflow.NotifyCtls,
	}
	data, err := json.Marshal(runtimeFields)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow runtime fields: %w", err)
	}

	referencedKeys := make(map[string]struct{})
	for _, match := range payloadVariableRegexp.FindAllSubmatch(data, -1) {
		if len(match) > 1 {
			referencedKeys[string(match[1])] = struct{}{}
		}
	}

	variablesByKey := make(map[string]*commonmodels.KeyVal, len(workflow.HookPayload.PayloadVars))
	for _, variable := range workflow.HookPayload.PayloadVars {
		if variable == nil {
			continue
		}
		if _, referenced := referencedKeys[variable.Key]; !referenced {
			continue
		}
		if _, exists := variablesByKey[variable.Key]; !exists {
			variablesByKey[variable.Key] = variable
		}
	}

	keys := make([]string, 0, len(variablesByKey))
	for key := range variablesByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	workflow.HookPayload.PayloadVars = make([]*commonmodels.KeyVal, 0, len(keys))
	for _, key := range keys {
		workflow.HookPayload.PayloadVars = append(workflow.HookPayload.PayloadVars, variablesByKey[key])
	}
	return nil
}

// ParseDynamicRecipientKind validates notification recipient variables and
// normalizes phone/mobile fields to the mobile contact kind.
func ParseDynamicRecipientKind(key string) (string, bool) {
	key = strings.ToLower(key)
	if strings.HasPrefix(key, "job.") {
		parts := strings.Split(key, ".")
		outputMarkerIndex := strings.LastIndex(key, ".output.")
		validInput := len(parts) == 3 && parts[1] != "" && parts[2] != ""
		validOutput := outputMarkerIndex > len("job.") &&
			!strings.Contains(key[outputMarkerIndex+len(".output."):], ".")
		if !validInput && !validOutput {
			return "", false
		}
	} else if !strings.HasPrefix(key, "payload.") && !strings.HasPrefix(key, "workflow.") {
		return "", false
	}
	if strings.HasSuffix(key, ".output.") {
		return "", false
	}

	parts := strings.Split(key, ".")
	field := parts[len(parts)-1]
	switch {
	case field == "email" || strings.HasSuffix(field, "_email"):
		return "email", true
	case field == "mobile" || strings.HasSuffix(field, "_mobile"),
		field == "phone" || strings.HasSuffix(field, "_phone"):
		return "mobile", true
	case field == "user_id" || strings.HasSuffix(field, "_user_id"),
		field == "userid" || strings.HasSuffix(field, "_userid"),
		field == "uid" || strings.HasSuffix(field, "_uid"):
		return "user_id", true
	default:
		return "", false
	}
}

// WorkflowGlobalContextToKeyMap converts Mongo-safe workflow context keys back
// to the variable names used by the notification renderer.
func WorkflowGlobalContextToKeyMap(context map[string]string) map[string]string {
	resp := make(map[string]string, len(context))
	for key, value := range context {
		key = strings.ReplaceAll(key, "@?", ".")
		key = strings.TrimSuffix(strings.TrimPrefix(key, "{{."), "}}")
		if key != "" {
			resp[key] = value
		}
	}
	return resp
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
	return resp
}

// BuildWorkflowPayloadVariableKVs exposes webhook payload fields to dynamic
// notification recipient resolution. Recipient templates are validated at the
// resolver boundary.
func BuildWorkflowPayloadVariableKVs(workflow *commonmodels.WorkflowV4) []*commonmodels.KeyVal {
	if workflow == nil || workflow.HookPayload == nil {
		return nil
	}
	return workflow.HookPayload.PayloadVars
}

func BuildWorkflowTriggerVariableKVs(hookPayload *commonmodels.HookPayload) []*commonmodels.KeyVal {
	if hookPayload == nil {
		return nil
	}

	resp := make([]*commonmodels.KeyVal, 0, 8)
	appendIfNotEmpty := func(key, value string) {
		if value != "" {
			resp = append(resp, &commonmodels.KeyVal{Key: key, Value: value, IsCredential: false})
		}
	}

	values := map[string]string{
		"payload.trigger.branch":         hookPayload.Branch,
		"payload.trigger.target_branch":  hookPayload.TargetBranch,
		"payload.trigger.pr":             hookPayload.MergeRequestID,
		"payload.trigger.commit_id":      hookPayload.CommitID,
		"payload.trigger.commit_sha":     inferWorkflowTriggerCommitSHA(hookPayload),
		"payload.trigger.commit_message": hookPayload.CommitMessage,
		"payload.trigger.committer":      hookPayload.Committer,
		"payload.trigger.event":          hookPayload.EventType,
	}
	for _, key := range WorkflowTriggerVariableKeys() {
		appendIfNotEmpty(key, values[key])
	}
	return resp
}

func WorkflowTriggerVariableKeys() []string {
	return []string{
		"payload.trigger.branch",
		"payload.trigger.target_branch",
		"payload.trigger.pr",
		"payload.trigger.commit_id",
		"payload.trigger.commit_sha",
		"payload.trigger.commit_message",
		"payload.trigger.committer",
		"payload.trigger.event",
	}
}

var commitSHARegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func inferWorkflowTriggerCommitSHA(hookPayload *commonmodels.HookPayload) string {
	if hookPayload.CommitSHA != "" {
		return hookPayload.CommitSHA
	}
	if commitSHARegex.MatchString(hookPayload.CommitID) {
		return hookPayload.CommitID
	}
	return ""
}

func BuildWorkflowRuntimeVariableKVs(workflow *commonmodels.WorkflowV4, projectName, projectDisplayName string, taskID int64, creator, account, uid string, now time.Time) []*commonmodels.KeyVal {
	resp := BuildWorkflowSystemVariableKVs(workflow, projectName, projectDisplayName, taskID, creator, account, uid, now)
	if workflow == nil || workflow.HookPayload == nil {
		return resp
	}
	resp = append(resp, workflow.HookPayload.PayloadVars...)
	return append(resp, BuildWorkflowTriggerVariableKVs(workflow.HookPayload)...)
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
