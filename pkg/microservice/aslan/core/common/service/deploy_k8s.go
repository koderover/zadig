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

import (
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceApplyParam struct {
	Namespace           string
	CurrentResourceYaml string
	UpdateResourceYaml  string
	informer            informers.SharedInformerFactory
	kubeClient          client.Client
}

// CreateOrPatchResource create or patch resources defined in UpdateResourceYaml
// `CurrentResourceYaml` will be used to determine if some resources will be uninstalled
func CreateOrPatchResource(applyParam *ResourceApplyParam) error {

	return nil
}
