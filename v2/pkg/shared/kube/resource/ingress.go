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

package resource

type Ingress struct {
	Name     string            `json:"name"`
	Labels   map[string]string `json:"labels"`
	HostInfo []HostInfo        `json:"host_info"`
	IPs      []string          `json:"ips"`
	Age      string            `json:"age"`
}

type HostInfo struct {
	Host     string    `json:"host"`
	Backends []Backend `json:"backend"`
}

type Backend struct {
	ServiceName string `json:"service_name"`
	ServicePort string `json:"service_port"`
}
