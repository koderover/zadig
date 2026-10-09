/*
 * Copyright 2026 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

const (
	ReleasePlanCustomFieldTypeString       = "string"
	ReleasePlanCustomFieldTypeText         = "text"
	ReleasePlanCustomFieldTypeSingleSelect = "single_select"
	ReleasePlanCustomFieldTypeMultiSelect  = "multi_select"
	ReleasePlanCustomFieldTypeCheckbox     = "checkbox"
)

var releasePlanCustomFieldTypes = map[string]struct{}{
	ReleasePlanCustomFieldTypeString:       {},
	ReleasePlanCustomFieldTypeText:         {},
	ReleasePlanCustomFieldTypeSingleSelect: {},
	ReleasePlanCustomFieldTypeMultiSelect:  {},
	ReleasePlanCustomFieldTypeCheckbox:     {},
}

func ListReleasePlanCustomFields() ([]*models.ReleasePlanCustomFieldDefinition, error) {
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		return nil, errors.Wrap(err, "get release plan hook setting")
	}
	fields := hookSetting.CustomFields
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Order < fields[j].Order })
	return fields, nil
}

// buildReleasePlanCustomFields turns the submitted field list into the definitions to store.
// Fields without an id get a generated id and key; changing an existing field's type generates a new key;
// fields missing from the list are deleted; the list order becomes the display order.
func buildReleasePlanCustomFields(existing, submitted []*models.ReleasePlanCustomFieldDefinition) ([]*models.ReleasePlanCustomFieldDefinition, error) {
	existingByID := make(map[string]*models.ReleasePlanCustomFieldDefinition, len(existing))
	for _, field := range existing {
		existingByID[field.ID] = field
	}
	seenIDs := make(map[string]struct{}, len(submitted))
	seenNames := make(map[string]struct{}, len(submitted))
	fields := make([]*models.ReleasePlanCustomFieldDefinition, 0, len(submitted))
	for i, item := range submitted {
		if item == nil {
			continue
		}
		field := *item
		field.Name = strings.TrimSpace(field.Name)
		field.Options = slices.Clone(field.Options)
		for i, option := range field.Options {
			field.Options[i] = strings.TrimSpace(option)
		}
		if err := validateReleasePlanCustomFieldDefinition(&field); err != nil {
			return nil, err
		}
		if _, ok := seenNames[field.Name]; ok {
			return nil, fmt.Errorf("custom field name %s already exists", field.Name)
		}
		seenNames[field.Name] = struct{}{}

		if field.ID == "" {
			field.ID = primitive.NewObjectID().Hex()
			field.Key = "custom_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		} else {
			current, ok := existingByID[field.ID]
			if !ok {
				return nil, fmt.Errorf("custom field %s not found", field.ID)
			}
			if _, ok := seenIDs[field.ID]; ok {
				return nil, fmt.Errorf("custom field %s is duplicated", field.ID)
			}
			field.Key = current.Key
			if field.Type != current.Type {
				field.Key = "custom_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			}
		}
		seenIDs[field.ID] = struct{}{}
		field.Order = i + 1
		fields = append(fields, &field)
	}
	return fields, nil
}

func validateReleasePlanCustomFieldDefinition(field *models.ReleasePlanCustomFieldDefinition) error {
	if field.Name == "" {
		return e.ErrInvalidParam.AddDesc("custom field name cannot be empty")
	}
	if _, ok := releasePlanCustomFieldTypes[field.Type]; !ok {
		return e.ErrInvalidParam.AddDesc(fmt.Sprintf("unsupported custom field type %s", field.Type))
	}
	selectionType := isReleasePlanCustomFieldSelectionType(field.Type)
	if selectionType && len(field.Options) == 0 {
		return e.ErrInvalidParam.AddDesc(fmt.Sprintf("options of custom field %s cannot be empty", field.Name))
	}
	if !selectionType && len(field.Options) > 0 {
		return e.ErrInvalidParam.AddDesc("options are only supported for selection fields")
	}
	seen := make(map[string]struct{}, len(field.Options))
	for _, option := range field.Options {
		if option == "" {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("options of custom field %s cannot contain empty values", field.Name))
		}
		if _, ok := seen[option]; ok {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("duplicate option %s in custom field %s", option, field.Name))
		}
		seen[option] = struct{}{}
	}
	return nil
}

func isReleasePlanCustomFieldSelectionType(fieldType string) bool {
	return fieldType == ReleasePlanCustomFieldTypeSingleSelect ||
		fieldType == ReleasePlanCustomFieldTypeMultiSelect ||
		fieldType == ReleasePlanCustomFieldTypeCheckbox
}

// snapshotReleasePlanCustomFields stores the current field definitions on a new plan
// and validates submitted values, filtering outdated values when copying a plan.
func snapshotReleasePlanCustomFields(plan *models.ReleasePlan, definitions []*models.ReleasePlanCustomFieldDefinition, isCopy bool) error {
	if isCopy {
		plan.CustomFields = filterReleasePlanCustomFieldValues(definitions, plan.CustomFields)
	} else if err := validateReleasePlanCustomFieldValues(definitions, plan.CustomFields, false); err != nil {
		return err
	}
	plan.CustomFieldDefinitions = definitions
	return nil
}

// filterReleasePlanCustomFieldValues keeps only the values that are still valid for definitions.
// It is used when values come from another plan whose field definitions may be outdated.
func filterReleasePlanCustomFieldValues(definitions []*models.ReleasePlanCustomFieldDefinition, values map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		value, ok := values[definition.Key]
		if !ok || validateReleasePlanCustomFieldValue(definition, value) != nil {
			continue
		}
		filtered[definition.Key] = value
	}
	return filtered
}

func validateReleasePlanCustomFieldValues(definitions []*models.ReleasePlanCustomFieldDefinition, values map[string]interface{}, requireRequired bool) error {
	byKey := make(map[string]*models.ReleasePlanCustomFieldDefinition, len(definitions))
	for _, definition := range definitions {
		if definition != nil {
			byKey[definition.Key] = definition
		}
	}
	for key, value := range values {
		definition, ok := byKey[key]
		if !ok {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s is not defined", key))
		}
		if err := validateReleasePlanCustomFieldValue(definition, value); err != nil {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s: %s", definition.Name, err))
		}
	}
	if requireRequired {
		for _, definition := range definitions {
			if definition != nil && definition.Required && isEmptyReleasePlanCustomFieldValue(values[definition.Key]) {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("required custom field %s is empty", definition.Name))
			}
		}
	}
	return nil
}

func validateReleasePlanCustomFieldValue(definition *models.ReleasePlanCustomFieldDefinition, value interface{}) error {
	if isEmptyReleasePlanCustomFieldValue(value) {
		return nil
	}
	switch definition.Type {
	case ReleasePlanCustomFieldTypeString, ReleasePlanCustomFieldTypeText:
		if _, ok := value.(string); !ok {
			return errors.New("value must be a string")
		}
	case ReleasePlanCustomFieldTypeSingleSelect:
		selected, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}
		if !slices.Contains(definition.Options, selected) {
			return fmt.Errorf("value %s is not an option", selected)
		}
	case ReleasePlanCustomFieldTypeMultiSelect, ReleasePlanCustomFieldTypeCheckbox:
		selected, ok := releasePlanCustomFieldStringSlice(value)
		if !ok {
			return errors.New("value must be an array of strings")
		}
		for _, item := range selected {
			if !slices.Contains(definition.Options, item) {
				return fmt.Errorf("value %s is not an option", item)
			}
		}
	default:
		return fmt.Errorf("unsupported custom field type %s", definition.Type)
	}
	return nil
}

// releasePlanCustomFieldStringSlice accepts []string from Go callers, []interface{} from JSON
// and primitive.A from values decoded out of MongoDB.
func releasePlanCustomFieldStringSlice(value interface{}) ([]string, bool) {
	var items []interface{}
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []interface{}:
		items = typed
	case primitive.A:
		items = typed
	default:
		return nil, false
	}
	result := make([]string, len(items))
	for i, item := range items {
		str, ok := item.(string)
		if !ok {
			return nil, false
		}
		result[i] = str
	}
	return result, true
}

func isEmptyReleasePlanCustomFieldValue(value interface{}) bool {
	if value == nil {
		return true
	}
	if str, ok := value.(string); ok {
		return strings.TrimSpace(str) == ""
	}
	if values, ok := releasePlanCustomFieldStringSlice(value); ok {
		return len(values) == 0
	}
	return false
}
