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
	Name   string `json:"name" yaml:"name" bson:"name"`
	Avatar string `json:"avatar" yaml:"avatar" bson:"avatar"`
}

type DepartmentInfo struct {
	ID   string `json:"id" yaml:"id" bson:"id"`
	Name string `json:"name" yaml:"name" bson:"name"`
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
