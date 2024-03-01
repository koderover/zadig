/*
Copyright 2021 The KodeRover Authors.

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

type ServicesInExternalEnv struct {
	ProductName string `bson:"product_name"         json:"product_name"`
	ServiceName string `bson:"service_name"         json:"service_name"`
	EnvName     string `bson:"env_name"             json:"env_name"`
	Namespace   string `bson:"namespace"            json:"namespace"`
	ClusterID   string `bson:"cluster_id"           json:"cluster_id"`
}

func (ServicesInExternalEnv) TableName() string {
	return "services_in_external_env"
}
