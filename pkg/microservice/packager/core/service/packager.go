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

package service

type ImageData struct {
	ImageUrl  string `bson:"image_url" json:"image_url"`
	ImageName string `bson:"image_name" json:"image_name"`
	ImageTag  string `bson:"image_tag" json:"image_tag"`
}

// ImagesByService defines all images in a service
type ImagesByService struct {
	ServiceName string       `bson:"service_name" json:"service_name"`
	Images      []*ImageData `bson:"images" json:"images"`
}

//DockerRegistry  registry host/user/password
type DockerRegistry struct {
	Host      string `yaml:"host"`
	UserName  string `yaml:"username"`
	Password  string `yaml:"password"`
	Namespace string `yaml:"namespace"`
}

// Context ...
type Context struct {
	JobType          string             `yaml:"job_type"`
	ProgressFile     string             `yaml:"progress_file"`
	Images           []*ImagesByService `yaml:"images"`
	SourceRegistries []*DockerRegistry  `yaml:"source_registries"`
	TargetRegistries []*DockerRegistry  `yaml:"target_registries"`
}
