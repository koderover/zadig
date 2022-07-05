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

package types

type WorkloadInfo struct {
	PodName      string `json:"pod_name"`
	PodNamespace string `json:"pod_namespace"`
}

const IDEContainerNameDev = "dev"
const IDEContainerNameSidecar = "sidecar"

type StartDevmodeInfo struct {
	DevImage string `json:"dev_image"`
}

const IDESidecarImage = "ccr.ccs.tencentyun.com/koderover-rc/zgctl-sidecar:20220526172433-amd64"

const DevmodeWorkDir = "/home/zadig/"
