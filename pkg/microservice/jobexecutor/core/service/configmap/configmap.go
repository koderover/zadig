/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package configmap

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/pkg/tool/log"
)

type Updater interface {
	Get() *v1.ConfigMap
	Update() error
	UpdateWithRetry(retryCount int, retryInterval time.Duration) error
}

type updater struct {
	cm     *v1.ConfigMap
	ns     string
	client *kubernetes.Clientset
}

func NewUpdater(cm *v1.ConfigMap, namespace string, client *kubernetes.Clientset) Updater {
	return &updater{
		cm:     cm,
		ns:     namespace,
		client: client,
	}
}

func (u *updater) Get() *v1.ConfigMap {
	return u.cm
}

func (u *updater) Update() error {
	_, err := u.client.CoreV1().ConfigMaps(u.ns).Update(context.Background(), u.cm, metav1.UpdateOptions{})
	return err
}

func (u *updater) UpdateWithRetry(retryCount int, retryInterval time.Duration) (err error) {
	for i := 0; i < retryCount; i++ {
		err = u.Update()
		if err == nil {
			return nil
		}
		log.Warnf("update configmap %s/%s error: %v, retry", u.ns, u.cm.Name, err)
		time.Sleep(retryInterval)
	}
	return err
}
