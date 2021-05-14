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
)

// event is the wrapper for corev1.Event type.
type event struct {
	*corev1.Event
}

func Event(w *corev1.Event) *event {
	if w == nil {
		return nil
	}

	return &event{
		Event: w,
	}
}

// Unwrap returns the corev1.Event object.
func (w *event) Unwrap() *corev1.Event {
	return w.Event
}

func (w *event) Resource() *resource.Event {
	return &resource.Event{
		Reason:    w.Reason,
		Message:   w.Message,
		FirstSeen: w.FirstTimestamp.Unix(),
		LastSeen:  w.LastTimestamp.Unix(),
		Count:     w.Count,
		Type:      w.Type,
	}
}
