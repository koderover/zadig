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

type Pod struct {
	Kind                 string            `json:"kind"`
	Name                 string            `json:"name"`
	Status               string            `json:"status"`
	Age                  string            `json:"age"`
	CreateTime           int64             `json:"createtime"`
	IP                   string            `json:"ip"`
	Labels               map[string]string `json:"labels"`
	Containers           []Container       `json:"containers"`
	NodeName             string            `json:"node_name"`
	HostIP               string            `json:"host_ip"`
	EnableDebugContainer bool              `json:"enable_debug_container"`
	PodReady             bool              `json:"pod_ready"`
	ContainersReady      bool              `json:"containers_ready"`
	ContainersMessage    string            `json:"containers_message"`
	Succeed              bool              `json:"-"`
	Ready                bool              `json:"-"`
}

type Container struct {
	Name         string          `json:"name"`
	Image        string          `json:"image"`
	RestartCount int32           `json:"restart_count"`
	Status       string          `json:"status"`
	Ready        bool            `json:"ready"`
	Ports        []ContainerPort `json:"ports"`
	// Message regarding the last termination of the container
	Message string `json:"message"`
	// reason from the last termination of the container
	Reason string `json:"reason"`
	// Time at which previous execution of the container started
	StartedAt int64 `json:"started_at,omitempty"`
	// Time at which the container last terminated
	FinishedAt int64 `json:"finished_at,omitempty"`
}

type ContainerPort struct {
	// If specified, this must be an IANA_SVC_NAME and unique within the pod. Each
	// named port in a pod must have a unique name. Name for the port that can be
	// referred to by services.
	// +optional
	Name string `json:"name,omitempty"`
	// Number of port to expose on the host.
	// If specified, this must be a valid port number, 0 < x < 65536.
	// If HostNetwork is specified, this must match ContainerPort.
	// Most containers do not need this.
	// +optional
	HostPort int32 `json:"hostPort,omitempty"`
	// Number of port to expose on the pod's IP address.
	// This must be a valid port number, 0 < x < 65536.
	ContainerPort int32 `json:"containerPort"`
	// Protocol for port. Must be UDP, TCP, or SCTP.
	// Defaults to "TCP".
	// +optional
	// +default="TCP"
	Protocol Protocol `json:"protocol,omitempty"`
	// What host IP to bind the external port to.
	// +optional
	HostIP string `json:"hostIP,omitempty"`
}

type Protocol string

const (
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP Protocol = "TCP"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP Protocol = "UDP"
	// ProtocolSCTP is the SCTP protocol.
	ProtocolSCTP Protocol = "SCTP"
)
