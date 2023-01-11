/*
Copyright 2023 The KodeRover Authors.

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

import "go.uber.org/zap"

type NacosNamespace struct {
	NamespaceID    string `json:"namespace_id"`
	NamespacedName string `json:"namespace_name"`
}

type NacosConfig struct {
	DataID  string `json:"data_id"`
	Group   string `json:"group"`
	Desc    string `json:"description,omitempty"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

func ListNacosNamespace(nacosID string, log *zap.SugaredLogger) ([]*NacosNamespace, error) {
	resp := []*NacosNamespace{}
	return resp, nil
}

func ListNacosGroup(nacosID, namespaceID string, log *zap.SugaredLogger) ([]string, error) {
	resp := []string{}
	return resp, nil
}

func ListNacosConfig(nacosID, namespaceID, group string, log *zap.SugaredLogger) ([]*NacosConfig, error) {
	resp := []*NacosConfig{}
	return resp, nil
}

func GetNacosConfig(nacosID, namespaceID, group, dataID string, log *zap.SugaredLogger) (*NacosConfig, error) {
	resp := &NacosConfig{}
	return resp, nil
}
