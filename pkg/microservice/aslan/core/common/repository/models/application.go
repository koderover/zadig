/*
Copyright 2025 The KodeRover Authors.

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
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Application struct {
	ID                    primitive.ObjectID        `bson:"_id,omitempty"                     json:"id"`
	Name                  string                    `bson:"name"                              json:"name"`
	Key                   string                    `bson:"key"                               json:"key"`
	Project               string                    `bson:"project"                           json:"project"`
	Repository            *ApplicationRepositoryRef `bson:"repository,omitempty"              json:"repository,omitempty"`
	Type                  string                    `bson:"type"                              json:"type"`
	Owner                 string                    `bson:"owner"                             json:"owner"`
	CreateTime            int64                     `bson:"create_time,omitempty"             json:"create_time"`
	UpdateTime            int64                     `bson:"update_time"                       json:"update_time"`
	Description           string                    `bson:"description,omitempty"             json:"description,omitempty"`
	TestingServiceName    string                    `bson:"testing_service_name,omitempty"    json:"testing_service_name,omitempty"`
	ProductionServiceName string                    `bson:"production_service_name,omitempty" json:"production_service_name,omitempty"`
	CustomFields          map[string]interface{}    `bson:"custom_fields,omitempty"           json:"custom_fields,omitempty"`
	// field used only for frontend, showing which plugin is activated on a specific application.
	Plugins []string `bson:"-"                                json:"plugins,omitempty"`
}

type ApplicationRepositoryRef struct {
	CodehostID    int    `bson:"codehost_id"                json:"codehost_id"`
	RepoOwner     string `bson:"repo_owner,omitempty"       json:"repo_owner,omitempty"`
	RepoNamespace string `bson:"repo_namespace,omitempty"   json:"repo_namespace,omitempty"`
	RepoName      string `bson:"repo_name,omitempty"        json:"repo_name,omitempty"`
	Branch        string `bson:"branch,omitempty"           json:"branch,omitempty"`
}

func (Application) TableName() string { return "application" }

type ApplicationFieldDefinition struct {
	ID          primitive.ObjectID                `bson:"_id,omitempty"     json:"id"`
	Key         string                            `bson:"key"               json:"key"`
	Name        string                            `bson:"name"              json:"name"`
	Type        config.ApplicationCustomFieldType `bson:"type"              json:"type"`
	Default     interface{}                       `bson:"default"           json:"default"`
	Options     []string                          `bson:"options,omitempty" json:"options,omitempty"`
	Unique      bool                              `bson:"unique"            json:"unique"`
	Required    bool                              `bson:"required"          json:"required"`
	ShowInList  bool                              `bson:"show_in_list"      json:"show_in_list"`
	Description string                            `bson:"description,omitempty" json:"description,omitempty"`
	// Source indicates where the field comes from.
	Source config.ApplicationFieldSourceType `bson:"source,omitempty"   json:"source,omitempty"`

	CreateTime int64 `bson:"create_time"       json:"create_time"`
	UpdateTime int64 `bson:"update_time"       json:"update_time"`
}

func (ApplicationFieldDefinition) TableName() string { return "application_field_definition" }

// Validate checks business rules for ApplicationFieldDefinition.
func (d *ApplicationFieldDefinition) Validate() error {
	if d == nil {
		return fmt.Errorf("empty body")
	}
	if strings.TrimSpace(d.Key) == "" || strings.TrimSpace(d.Name) == "" || strings.TrimSpace(string(d.Type)) == "" {
		return fmt.Errorf("key, name, type are required")
	}

	// validate supported types
	switch d.Type {
	case config.ApplicationCustomFieldTypeText,
		config.ApplicationCustomFieldTypeNumber,
		config.ApplicationCustomFieldTypeBool,
		config.ApplicationCustomFieldTypeDatetime,
		config.ApplicationCustomFieldTypeSingleSelect,
		config.ApplicationCustomFieldTypeMultiSelect,
		config.ApplicationCustomFieldTypeLink,
		config.ApplicationCustomFieldTypeUser,
		config.ApplicationCustomFieldTypeUserGroup,
		config.ApplicationCustomFieldTypeProject,
		config.ApplicationCustomFieldTypeRepository:
		// supported
	default:
		return fmt.Errorf("invalid type")
	}

	// options validation for select types
	if (d.Type == config.ApplicationCustomFieldTypeSingleSelect || d.Type == config.ApplicationCustomFieldTypeMultiSelect) && len(d.Options) == 0 {
		return fmt.Errorf("options required for select types")
	}
	if d.Type != config.ApplicationCustomFieldTypeSingleSelect && d.Type != config.ApplicationCustomFieldTypeMultiSelect && len(d.Options) > 0 {
		return fmt.Errorf("options only allowed for select types")
	}
	if d.Type == config.ApplicationCustomFieldTypeMultiSelect && d.Unique {
		return fmt.Errorf("multi_select cannot be unique")
	}
	return nil
}
