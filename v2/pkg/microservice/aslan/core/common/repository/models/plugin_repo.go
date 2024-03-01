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
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PluginRepo struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"             yaml:"source,omitempty"`
	IsOffical       bool               `bson:"is_offical"                json:"is_offical"               yaml:"is_offical"`
	RepoURL         string             `bson:"repo_url"                  json:"repo_url"                 yaml:"-"`
	RepoNamespace   string             `bson:"repo_namespace"            json:"repo_namespace"           yaml:"repo_namespace"`
	Source          string             `bson:"source,omitempty"          json:"source,omitempty"         yaml:"source,omitempty"`
	RepoOwner       string             `bson:"repo_owner"                json:"repo_owner"               yaml:"repo_owner"`
	RepoName        string             `bson:"repo_name"                 json:"repo_name"                yaml:"repo_name"`
	Branch          string             `bson:"branch"                    json:"branch"                   yaml:"branch"`
	CommitID        string             `bson:"commit_id,omitempty"       json:"commit_id,omitempty"      yaml:"commit_id,omitempty"`
	CodehostID      int                `bson:"codehost_id"               json:"codehost_id"              yaml:"codehost_id"`
	UpdateTime      int64              `bson:"update_time"               json:"update_time"              yaml:"update_time"`
	PluginTemplates []*PluginTemplate  `bson:"plugin_templates"          json:"plugin_templates"         yaml:"plugin_templates"`
	Status          string             `bson:"status"                    json:"status"                   yaml:"status"`
	Error           string             `bson:"error"                     json:"error"                    yaml:"error"`
}

type PluginTemplate struct {
	Name        string    `bson:"name"             json:"name"             yaml:"name"`
	IsOffical   bool      `bson:"is_offical"       json:"is_offical"       yaml:"is_offical"`
	Category    string    `bson:"category"         json:"category"         yaml:"category"`
	Description string    `bson:"description"      json:"description"      yaml:"description"`
	RepoURL     string    `bson:"repo_url"         json:"repo_url"         yaml:"-"`
	Version     string    `bson:"version"          json:"version"          yaml:"version"`
	Image       string    `bson:"image"            json:"image"            yaml:"image"`
	Args        []string  `bson:"args"             json:"args"             yaml:"args"`
	Cmds        []string  `bson:"cmds"             json:"cmds"             yaml:"cmds"`
	Envs        []*Env    `bson:"envs"             json:"envs"             yaml:"envs"`
	Inputs      []*Param  `bson:"inputs"           json:"inputs"           yaml:"inputs"`
	Outputs     []*Output `bson:"outputs"          json:"outputs"          yaml:"outputs"`
}

type Env struct {
	Name  string `bson:"name"             json:"name"             yaml:"name"`
	Value string `bson:"value"            json:"value"            yaml:"value"`
}

func (PluginRepo) TableName() string {
	return "plugin_repo"
}
