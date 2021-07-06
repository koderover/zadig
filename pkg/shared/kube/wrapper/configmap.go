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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	kubeutil "github.com/koderover/zadig/pkg/tool/kube/util"
	"github.com/koderover/zadig/pkg/util"
)

// configMap is the wrapper for corev1.ConfigMap type.
type configMap struct {
	*corev1.ConfigMap
}

func ConfigMap(w *corev1.ConfigMap) *configMap {
	if w == nil {
		return nil
	}

	return &configMap{
		ConfigMap: w,
	}
}

// Unwrap returns the corev1.ConfigMap object.
func (w *configMap) Unwrap() *corev1.ConfigMap {
	return w.ConfigMap
}

func (w *configMap) LastUpdateTime() (time.Time, error) {
	// for compatibility
	if updateTime, ok := w.Labels[setting.UpdateTime]; ok {
		return time.ParseInLocation("20060102150405", updateTime, time.Local)
	}

	if t, ok := w.Annotations[setting.LastUpdateTimeAnnotation]; ok {
		return kubeutil.ParseTime(t)
	}

	return w.CreationTimestamp.Time, nil
}

func (w *configMap) Active() bool {
	// for compatibility
	if _, ok := w.Labels[setting.ConfigBackupLabel]; ok {
		return false
	}

	if inactive, ok := w.Annotations[setting.InactiveConfigLabel]; ok {
		ia, err := strconv.ParseBool(inactive)
		if err == nil {
			return !ia
		}
	}

	return true
}

func (w *configMap) ModifiedBy() string {
	// for compatibility
	if u, ok := w.Labels[setting.UpdateBy]; ok {
		return u
	}

	if u, ok := w.Annotations[setting.ModifiedByAnnotation]; ok {
		return u
	}

	return ""
}

func (w *configMap) Owner() string {
	// for compatibility
	s := strings.SplitN(w.Name, "-bak-", 2)
	if len(s) == 2 {
		return s[0]
	}

	if owner, ok := w.Labels[setting.OwnerLabel]; ok {
		return owner
	}

	return ""
}

func (w *configMap) Resource() *resource.ConfigMap {
	updateTime, _ := w.LastUpdateTime()
	return &resource.ConfigMap{
		Name:              w.Name,
		Age:               util.Age(w.CreationTimestamp.Unix()),
		Labels:            w.Labels,
		Data:              w.Data,
		CreationTimestamp: w.CreationTimestamp.Time,
		LastUpdateTime:    updateTime,
	}
}
