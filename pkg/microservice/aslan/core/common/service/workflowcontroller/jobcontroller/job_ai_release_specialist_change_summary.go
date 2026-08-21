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

const (
	aiReleaseSpecialistConfigFieldLimit = 100
	aiReleaseSpecialistConfigMaxDepth   = 50
	aiReleaseSpecialistConfigTooDeep    = "[truncated: too deep]"
)

func appendAIReleaseSpecialistTaskDetails(item *commonmodels.AIReleaseSummaryItem, job *commonmodels.JobTask) error {
	if item == nil || job == nil {
		return nil
	}

	switch job.JobType {
	case string(config.JobNacos):
		spec := &commonmodels.JobTaskNacosSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			return fmt.Errorf("decode Nacos task %s: %w", job.OriginName, err)
		}
		item.ConfigChanges = buildAIConfigChangeSummaries(spec)
	case string(config.JobApollo):
		spec := &commonmodels.JobTaskApolloSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			return fmt.Errorf("decode Apollo task %s: %w", job.OriginName, err)
		}
		item.ConfigChanges = buildAIApolloChangeSummaries(spec)
	case string(config.JobSQL):
		spec := &commonmodels.JobTaskSQLSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			return fmt.Errorf("decode SQL task %s: %w", job.OriginName, err)
		}
		item.SQLExecution = buildAISQLExecutionSummary(spec)
	}
	return nil
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
		flattenAIConfigFields("", value, fields, 0)
	case "json":
		var value interface{}
		if err := json.Unmarshal([]byte(content), &value); err != nil {
			return nil, false
		}
		flattenAIConfigFields("", value, fields, 0)
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

func flattenAIConfigFields(path string, value interface{}, fields map[string]interface{}, depth int) {
	if depth >= aiReleaseSpecialistConfigMaxDepth {
		switch value.(type) {
		case map[string]interface{}, map[interface{}]interface{}:
			if path == "" {
				path = "$"
			}
			fields[path] = aiReleaseSpecialistConfigTooDeep
			return
		}
	}

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
			flattenAIConfigFields(childPath, item, fields, depth+1)
		}
	case map[interface{}]interface{}:
		if len(typed) == 0 && path != "" {
			fields[path] = typed
			return
		}
		for key, item := range typed {
			childPath := fmt.Sprint(key)
			if path != "" {
				childPath = path + "." + childPath
			}
			flattenAIConfigFields(childPath, item, fields, depth+1)
		}
	default:
		if path == "" {
			path = "$"
		}
		fields[path] = typed
	}
}

func hashAIConfigFields(added, updated, removed []string) string {
	fields := make([]string, 0, len(added)+len(updated)+len(removed))
	for _, field := range added {
		fields = append(fields, "added:"+field)
	}
	for _, field := range updated {
		fields = append(fields, "updated:"+field)
	}
	for _, field := range removed {
		fields = append(fields, "removed:"+field)
	}
	sort.Strings(fields)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(fields, "\n"))))
}

func limitAIConfigFields(added, updated, removed []string) ([]string, []string, []string) {
	// Rotate from removals to additions so every change type remains visible.
	values := [][]string{removed, updated, added}
	limited := make([][]string, len(values))
	for remaining := aiReleaseSpecialistConfigFieldLimit; remaining > 0; {
		progress := false
		for index := range values {
			if len(limited[index]) >= len(values[index]) {
				continue
			}
			limited[index] = append(limited[index], values[index][len(limited[index])])
			remaining--
			progress = true
			if remaining == 0 {
				break
			}
		}
		if !progress {
			break
		}
	}
	// values and limited are ordered as removed, updated, added.
	return limited[2], limited[1], limited[0]
}
