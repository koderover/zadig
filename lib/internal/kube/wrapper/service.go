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
	corev1 "k8s.io/api/core/v1"

	"github.com/koderover/zadig/lib/internal/kube/resource"
	"github.com/koderover/zadig/lib/util"
)

// service is the wrapper for corev1.Service type.
type service struct {
	*corev1.Service
}

func Service(w *corev1.Service) *service {
	if w == nil {
		return nil
	}

	return &service{
		Service: w,
	}
}

// Unwrap returns the corev1.Service object.
func (w *service) Unwrap() *corev1.Service {
	return w.Service
}

func (w *service) Ports() []corev1.ServicePort {
	return w.Spec.Ports
}

func (w *service) Resource() *resource.Service {
	ports := w.Ports()
	ps := make([]resource.ServicePort, 0, len(ports))
	for _, p := range ports {
		ps = append(ps, resource.ServicePort{
			ServiceName: w.Name,
			ServicePort: p.Port,
			NodePort:    p.NodePort,
		})
	}

	return &resource.Service{
		Name:   w.Name,
		Age:    util.Age(w.CreationTimestamp.Unix()),
		Labels: w.Labels,
		Ports:  ps,
	}
}
