/*
 * Copyright 2023 The KodeRover Authors.
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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
)

type Observability struct {
	ID   primitive.ObjectID       `json:"id" bson:"_id,omitempty" yaml:"id"`
	Type config.ObservabilityType `json:"type" bson:"type" yaml:"type"`
	Name string                   `json:"name" bson:"name" yaml:"name"`
	Host string                   `json:"host" bson:"host" yaml:"host"`
	// ConsoleHost is used for guanceyun console, Host is guanceyun OpenApi Addr
	ConsoleHost string `json:"console_host" bson:"console_host" yaml:"console_host"`
	// ApiKey is used for guanceyun
	ApiKey string `json:"api_key" bson:"api_key" yaml:"api_key"`

	GrafanaToken string `json:"grafana_token" bson:"grafana_token" yaml:"grafana_token"`
	UpdateTime   int64  `json:"update_time" bson:"update_time" yaml:"update_time"`
}

func (Observability) TableName() string {
	return "observability"
}
