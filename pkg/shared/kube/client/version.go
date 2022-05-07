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

package client

import (
	"k8s.io/apimachinery/pkg/util/version"
	k8sversion "k8s.io/apimachinery/pkg/version"
)

var KubernetesVersion122 *version.Version

func init() {
	// as of zadig v1.11.0. Only kubernetes version 1.17+ is supported
	// There is only 1 major api version change from v1.17+
	// More versions of kubernetes should be added if there are more API changes
	// in future kubernetes version
	v122, _ := version.ParseGeneric("v1.22.0")
	KubernetesVersion122 = v122
}

func VersionLessThan122(ver *k8sversion.Info) bool {
	currVersion, _ := version.ParseGeneric(ver.String())
	return currVersion.LessThan(KubernetesVersion122)
}
