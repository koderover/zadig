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

package wrapper

import (
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	"github.com/koderover/zadig/lib/internal/kube/resource"
	"github.com/koderover/zadig/lib/util"
)

// ingress is the wrapper for extensionsv1beta1.Ingress type.
type ingress struct {
	*extensionsv1beta1.Ingress
}

func Ingress(ing *extensionsv1beta1.Ingress) *ingress {
	if ing == nil {
		return nil
	}

	return &ingress{
		Ingress: ing,
	}
}

// Unwrap returns the extensionsv1beta1.Ingress object.
func (ing *ingress) Unwrap() *extensionsv1beta1.Ingress {
	return ing.Ingress
}

func (ing *ingress) HostInfo() []resource.HostInfo {
	var res []resource.HostInfo
	for _, rule := range ing.Spec.Rules {
		info := resource.HostInfo{
			Host:     rule.Host,
			Backends: []resource.Backend{},
		}

		if rule.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			backend := resource.Backend{
				ServiceName: path.Backend.ServiceName,
				ServicePort: path.Backend.ServicePort.String(),
			}
			info.Backends = append(info.Backends, backend)
		}

		res = append(res, info)
	}

	return res
}

func (ing *ingress) Resource() *resource.Ingress {
	resp := &resource.Ingress{
		Name:     ing.Name,
		Labels:   ing.Labels,
		Age:      util.Age(ing.CreationTimestamp.Unix()),
		IPs:      []string{},
		HostInfo: ing.HostInfo(),
	}

	for _, lb := range ing.Status.LoadBalancer.Ingress {
		resp.IPs = append(resp.IPs, lb.IP)
	}

	return resp
}
