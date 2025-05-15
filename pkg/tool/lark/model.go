/*
 * Copyright 2022 The KodeRover Authors.
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

package lark

type UserInfo struct {
	ID     string `json:"id" yaml:"id" bson:"id"`
	IDType string `json:"id_type" yaml:"id_type" bson:"id_type"`
	Name   string `json:"name" yaml:"name" bson:"name"`
	Avatar string `json:"avatar,omitempty" yaml:"avatar,omitempty" bson:"avatar,omitempty"`
	// IsExecutor marks if the user is the executor of the workflow
	IsExecutor bool `json:"is_executor" yaml:"is_executor" bson:"is_executor"`
}

type DepartmentInfo struct {
	ID           string `json:"id" yaml:"id" bson:"id"`
	DepartmentID string `json:"department_id" yaml:"department_id" bson:"department_id"`
	Name         string `json:"name" yaml:"name" bson:"name"`
}

type ContactRange struct {
	UserIDs       []string `json:"user_id_list"`
	DepartmentIDs []string `json:"department_id_list"`
	GroupIDs      []string `json:"group_id_list,omitempty"`
}

type pageInfo struct {
	token   string
	hasMore bool
}

type formData struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}
