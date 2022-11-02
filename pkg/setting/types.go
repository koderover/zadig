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

type RequestSpec struct {
	CpuLimit    int `bson:"cpu_limit"                     json:"cpu_limit"               yaml:"cpu_limit"`
	MemoryLimit int `bson:"memory_limit"                  json:"memory_limit"            yaml:"memory_limit"`
	CpuReq      int `bson:"cpu_req,omitempty"             json:"cpu_req,omitempty"       yaml:"cpu_req,omitempty"`
	MemoryReq   int `bson:"memory_req,omitempty"          json:"memory_req,omitempty"    yaml:"memory_req,omitempty"`
	// gpu request, eg: "nvidia.com/gpu: 1"
	GpuLimit string `bson:"gpu_limit"                     json:"gpu_limit"               yaml:"gpu_limit"`
}

var (
	// HighRequestSpec 16 CPU 32 G
	HighRequestSpec = RequestSpec{
		CpuLimit:    16000,
		MemoryLimit: 32768,
		CpuReq:      4000,
		MemoryReq:   4096,
	}
	// MediumRequestSpec 8 CPU 16 G
	MediumRequestSpec = RequestSpec{
		CpuLimit:    8000,
		MemoryLimit: 16384,
		CpuReq:      2000,
		MemoryReq:   2048,
	}
	// LowRequestSpec 4 CPU 8 G
	LowRequestSpec = RequestSpec{
		CpuLimit:    4000,
		MemoryLimit: 8192,
		CpuReq:      1000,
		MemoryReq:   1024,
	}
	// MinRequestSpec 2 CPU 2 G
	MinRequestSpec = RequestSpec{
		CpuLimit:    2000,
		MemoryLimit: 2048,
		CpuReq:      500,
		MemoryReq:   512,
	}
	// DefineRequest X CPU X G
	DefineRequestSpec  = RequestSpec{}
	DefaultRequestSpec = RequestSpec{
		CpuLimit:    4000,
		MemoryLimit: 8192,
		CpuReq:      1000,
		MemoryReq:   1024,
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
	SonarQube            // 7
	WarpDrive            // 8
	Minio                // 9
	OPA                  // 10
	Policy               // 11
	Vendor
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
	Vendor: {
		Name: "plutus-vendor",
		Port: 29000,
	},
}
