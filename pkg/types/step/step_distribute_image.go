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

package step

type StepImageDistributeSpec struct {
	SourceRegistry   *RegistryNamespace      `bson:"source_registry"                json:"source_registry"               yaml:"source_registry"`
	TargetRegistry   *RegistryNamespace      `bson:"target_registry"                json:"target_registry"               yaml:"target_registry"`
	DistributeTarget []*DistributeTaskTarget `bson:"distribute_target"              json:"distribute_target"             yaml:"distribute_target"`
}

type DistributeTaskTarget struct {
	SoureImage    string `bson:"source_image"       yaml:"source_image"     json:"source_image"`
	TargetImage   string `bson:"target_image"       yaml:"target_image"     json:"target_image"`
	TargetTag     string `bson:"target_tag"         yaml:"target_tag"       json:"target_tag"`
	ServiceName   string `bson:"service_name"       yaml:"service_name"     json:"service_name"`
	ServiceModule string `bson:"service_module"     yaml:"service_module"   json:"service_module"`
	UpdateTag     bool   `bson:"update_tag"         yaml:"update_tag"       json:"update_tag"`
}

type RegistryNamespace struct {
	RegAddr string `bson:"reg_addr"            json:"reg_addr"             yaml:"reg_addr"`
	// Namespace is NOT a required field, this could be empty when the registry is AWS ECR or so.
	// use with CAUTION !!!!
	TLSEnabled bool   `bson:"tls_enabled"              json:"tls_enabled"             yaml:"tls_enabled"`
	TLSCert    string `bson:"tls_cert"                 json:"tls_cert"                yaml:"tls_cert"`
	Namespace  string `bson:"namespace,omitempty"      json:"namespace,omitempty"     yaml:"namespace,omitempty"`
	AccessKey  string `bson:"access_key"               json:"access_key"              yaml:"access_key"`
	SecretKey  string `bson:"secret_key"               json:"secret_key"              yaml:"secret_key"`
}
