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

import "time"

type input struct {
	Cluster *ClusterInfo `json:"cluster"`
}

type ClusterInfo struct {
	ClusterID string    `json:"_"`
	Joined    time.Time `json:"_"`

	PodIP   string `json:"podIP"`
	Address string `json:"address"`
	Token   string `json:"token"`
	CACert  string `json:"caCert"`
}
