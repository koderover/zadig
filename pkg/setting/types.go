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

package setting

import "regexp"

type Request string

const (
	// HighRequest 16 CPU 32 G
	HighRequest Request = "high"
	// MediumRequest 8 CPU 16 G
	MediumRequest Request = "medium"
	// LowRequest 4 CPU 8 G
	LowRequest Request = "low"
	// MinRequest 2 CPU 2 G
	MinRequest Request = "min"
)

var ValidName = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`)
var ValidNameHint = "a valid name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character"

const (
	Aslan     = iota + 1 // 1
	Aslanx               // 2
	Clair                // 3
	Collie               // 4
	Cron                 // 5
	HubServer            // 6
	PodExec              // 7
	Poetry               // 8
	SonarQube            // 9
	WarpDrive            // 10
	Minio                // 11
)

type ServiceInfo struct {
	Name string
	Port int32
}

var Services = map[int]*ServiceInfo{
	Aslan: {
		Name: "aslan",
		Port: 25000,
	},
	Aslanx: {
		Name: "aslanx",
		Port: 25002,
	},
	Clair: {
		Name: "clair",
		Port: 34002,
	},
	Collie: {
		Name: "collie-server",
		Port: 28080,
	},
	Cron: {
		Name: "cron",
		Port: 80,
	},
	HubServer: {
		Name: "hub-server",
		Port: 26000,
	},
	PodExec: {
		Name: "podexec",
		Port: 27000,
	},
	Poetry: {
		Name: "poetry",
		Port: 34001,
	},
	SonarQube: {
		Name: "sonarqube",
		Port: 80,
	},
	WarpDrive: {
		Name: "warpdrive",
		Port: 80,
	},
	Minio: {
		Name: "zadig-minio",
		Port: 9000,
	},
}
