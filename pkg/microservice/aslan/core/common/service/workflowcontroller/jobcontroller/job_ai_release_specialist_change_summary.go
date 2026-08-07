/*
Copyright 2026 The KodeRover Authors.

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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/magiconair/properties"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

const aiReleaseSpecialistConfigFieldLimit = 100

func appendAIReleaseSpecialistTaskDetails(item *commonmodels.AIReleaseSummaryItem, job *commonmodels.JobTask) {
	if item == nil || job == nil {
		return
	}

	switch job.JobType {
	case string(config.JobNacos):
		spec := &commonmodels.JobTaskNacosSpec{}
		if commonmodels.IToi(job.Spec, spec) == nil {
			item.ConfigChanges = buildAIConfigChangeSummaries(spec)
		}
	case string(config.JobApollo):
		spec := &commonmodels.JobTaskApolloSpec{}
		if commonmodels.IToi(job.Spec, spec) == nil {
			item.ConfigChanges = buildAIApolloChangeSummaries(spec)
		}
	case string(config.JobSQL):
		spec := &commonmodels.JobTaskSQLSpec{}
		if commonmodels.IToi(job.Spec, spec) == nil {
			item.SQLExecution = buildAISQLExecutionSummary(spec)
		}
	}
}

func buildAIApolloChangeSummaries(spec *commonmodels.JobTaskApolloSpec) []*commonmodels.AIConfigChangeSummary {
	if spec == nil {
		return nil
	}

	result := make([]*commonmodels.AIConfigChangeSummary, 0, len(spec.NamespaceList))
	for _, namespace := range spec.NamespaceList {
		if namespace == nil {
			continue
		}
		before := make(map[string]interface{}, len(namespace.OriginalConfig))
		for _, item := range namespace.OriginalConfig {
			if item != nil {
				before[item.Key] = item.Val
			}
		}
		after := make(map[string]interface{}, len(namespace.KeyValList))
		for _, item := range namespace.KeyValList {
			if item != nil {
				after[item.Key] = item.Val
			}
		}
		added, updated, removed := diffAIConfigFieldMaps(before, after)
		changedFieldCount := len(added) + len(updated) + len(removed)
		summary := &commonmodels.AIConfigChangeSummary{
			NamespaceName:          namespace.Env,
			DataID:                 strings.Trim(strings.Join([]string{namespace.AppID, namespace.Namespace}, "/"), "/"),
			Group:                  namespace.ClusterID,
			Format:                 namespace.Type,
			ContentChanged:         changedFieldCount > 0,
			ChangedFieldsAvailable: true,
			ChangedFieldCount:      changedFieldCount,
			FieldsTruncated:        changedFieldCount > aiReleaseSpecialistConfigFieldLimit,
		}
		if changedFieldCount > 0 {
			summary.ChangedFieldsHash = hashAIConfigFields(added, updated, removed)
			summary.AddedFields, summary.UpdatedFields, summary.RemovedFields = limitAIConfigFields(added, updated, removed)
		}
		result = append(result, summary)
	}
	return result
}

func buildAIConfigChangeSummaries(spec *commonmodels.JobTaskNacosSpec) []*commonmodels.AIConfigChangeSummary {
	if spec == nil {
		return nil
	}

	result := make([]*commonmodels.AIConfigChangeSummary, 0, len(spec.NacosDatas))
	for _, data := range spec.NacosDatas {
		if data == nil {
			continue
		}
		summary := &commonmodels.AIConfigChangeSummary{
			NamespaceName:          data.NamespaceName,
			DataID:                 data.DataID,
			Group:                  data.Group,
			Format:                 data.Format,
			ContentChanged:         data.OriginalContent != data.Content,
			ChangedFieldsAvailable: data.OriginalContent == data.Content,
		}
		if strings.TrimSpace(summary.NamespaceName) == "" {
			summary.NamespaceName = spec.NamespaceName
		}
		if summary.ContentChanged {
			added, updated, removed, ok := diffAIConfigFields(data.Format, data.OriginalContent, data.Content)
			if ok {
				summary.ChangedFieldsAvailable = true
				summary.ChangedFieldCount = len(added) + len(updated) + len(removed)
				summary.ChangedFieldsHash = hashAIConfigFields(added, updated, removed)
				summary.FieldsTruncated = summary.ChangedFieldCount > aiReleaseSpecialistConfigFieldLimit
				summary.AddedFields, summary.UpdatedFields, summary.RemovedFields = limitAIConfigFields(added, updated, removed)
			}
		}
		result = append(result, summary)
	}
	return result
}

func buildAISQLExecutionSummary(spec *commonmodels.JobTaskSQLSpec) *commonmodels.AISQLExecutionSummary {
	if spec == nil {
		return nil
	}

	result := &commonmodels.AISQLExecutionSummary{StatementCount: len(spec.Results)}
	for _, statement := range spec.Results {
		if statement == nil {
			continue
		}
		switch statement.Status {
		case setting.SQLExecStatusSuccess:
			result.SuccessfulStatementCount++
			result.RowsAffected += statement.RowsAffected
		case setting.SQLExecStatusFailed:
			result.FailedStatementCount++
		default:
			result.PendingStatementCount++
		}
	}
	return result
}

func diffAIConfigFields(format, before, after string) (added, updated, removed []string, ok bool) {
	beforeFields, ok := parseAIConfigFields(format, before)
	if !ok {
		return nil, nil, nil, false
	}
	afterFields, ok := parseAIConfigFields(format, after)
	if !ok {
		return nil, nil, nil, false
	}

	added, updated, removed = diffAIConfigFieldMaps(beforeFields, afterFields)
	return added, updated, removed, true
}

func diffAIConfigFieldMaps(beforeFields, afterFields map[string]interface{}) (added, updated, removed []string) {
	for path, afterValue := range afterFields {
		beforeValue, exists := beforeFields[path]
		switch {
		case !exists:
			added = append(added, path)
		case !reflect.DeepEqual(beforeValue, afterValue):
			updated = append(updated, path)
		}
	}
	for path := range beforeFields {
		if _, exists := afterFields[path]; !exists {
			removed = append(removed, path)
		}
	}
	sort.Strings(added)
	sort.Strings(updated)
	sort.Strings(removed)
	return added, updated, removed
}

func parseAIConfigFields(format, content string) (map[string]interface{}, bool) {
	fields := make(map[string]interface{})
	if strings.TrimSpace(content) == "" {
		return fields, true
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "yaml", "yml":
		var value interface{}
		if err := yaml.Unmarshal([]byte(content), &value); err != nil {
			return nil, false
		}
		flattenAIConfigFields("", normalizeAIConfigValue(value), fields)
	case "json":
		var value interface{}
		if err := json.Unmarshal([]byte(content), &value); err != nil {
			return nil, false
		}
		flattenAIConfigFields("", value, fields)
	case "properties":
		parsed, err := properties.LoadString(content)
		if err != nil {
			return nil, false
		}
		for key, value := range parsed.Map() {
			fields[key] = value
		}
	default:
		return nil, false
	}
	return fields, true
}

func normalizeAIConfigValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			result[key] = normalizeAIConfigValue(item)
		}
		return result
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			result[fmt.Sprint(key)] = normalizeAIConfigValue(item)
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			result = append(result, normalizeAIConfigValue(item))
		}
		return result
	default:
		return value
	}
}

func flattenAIConfigFields(path string, value interface{}, fields map[string]interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		if len(typed) == 0 && path != "" {
			fields[path] = typed
			return
		}
		for key, item := range typed {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			flattenAIConfigFields(childPath, item, fields)
		}
	default:
		if path == "" {
			path = "$"
		}
		fields[path] = typed
	}
}

func hashAIConfigFields(added, updated, removed []string) string {
	fields := append(append(append([]string{}, added...), updated...), removed...)
	sort.Strings(fields)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(fields, "\n"))))
}

func limitAIConfigFields(added, updated, removed []string) ([]string, []string, []string) {
	remaining := aiReleaseSpecialistConfigFieldLimit
	limit := func(values []string) []string {
		if len(values) > remaining {
			values = values[:remaining]
		}
		remaining -= len(values)
		return append([]string(nil), values...)
	}
	return limit(added), limit(updated), limit(removed)
}
