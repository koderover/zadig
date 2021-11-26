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
	// DefaultRequest 4 CPU 8 G
	DefaultRequest Request = "default"
	// DefineRequest x CPU x G
	DefineRequest Request = "define"
)

// RequestSpec
type RequestSpec struct {
	CpuLimit    string `bson:"cpu_limit"           json:"cpu_limit"`
	MemoryLimit string `bson:"memory_limit"        json:"memory_limit"`
	CpuReq      string `bson:"cpu_req"             json:"cpu_req,omitempty"`
	MemoryReq   string `bson:"memory_req"          json:"memory_req,omitempty"`
}

var (
	// HighRequest 16 CPU 32 G
	HighRequestSpec = RequestSpec{
		CpuLimit:    "16",
		MemoryLimit: "32Gi",
		CpuReq:      "4",
		MemoryReq:   "4Gi",
	}

	// MediumRequest 8 CPU 16 G
	MediumRequestSpec = RequestSpec{
		CpuLimit:    "8",
		MemoryLimit: "16Gi",
		CpuReq:      "2",
		MemoryReq:   "2Gi",
	}
	// LowRequest 4 CPU 8 G
	LowRequestSpec = RequestSpec{
		CpuLimit:    "4",
		MemoryLimit: "8Gi",
		CpuReq:      "1",
		MemoryReq:   "1Gi",
	}
	// MinRequest 2 CPU 2 G
	MinRequestSpec = RequestSpec{
		CpuLimit:    "2",
		MemoryLimit: "2Gi",
		CpuReq:      "0.5",
		MemoryReq:   "512Mi",
	}
	// DefineRequest X CPU X G
	DefineRequestSpec = RequestSpec{}
	// DefaultRequest 4 CPU 8 G
	DefaultRequestSpec = RequestSpec{
		CpuLimit:    "4",
		MemoryLimit: "8Gi",
		CpuReq:      "1",
		MemoryReq:   "1Gi",
	}
)

const (
	CpuUintM     string = "m"
	MemoryUintMi string = "Mi"
	MemoryUintGi string = "Gi"
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
	SonarQube            // 9
	WarpDrive            // 10
	Minio                // 11
	OPA                  // 12
	Policy               // 13
	Config               // 14
	User                 // 15
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
	OPA: {
		Name: "opa",
		Port: 8181,
	},
	Policy: {
		Name: "policy",
		Port: 80,
	},
	Config: {
		Name: "config",
		Port: 80,
	},
	User: {
		Name: "user",
		Port: 80,
	},
}
