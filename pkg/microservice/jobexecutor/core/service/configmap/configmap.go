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

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/koderover/zadig/pkg/tool/log"
)

type Updater interface {
	Get() (*v1.ConfigMap, error)
	Update(cm *v1.ConfigMap) error
	UpdateWithRetry(cm *v1.ConfigMap, retryCount int, retryInterval time.Duration) error
}

type updater struct {
	//cm            *v1.ConfigMap
	configMapName string
	ns            string
	client        *kubernetes.Clientset
}

func NewUpdater(cmName, namespace string, client *kubernetes.Clientset) Updater {
	return &updater{
		configMapName: cmName,
		ns:            namespace,
		client:        client,
	}
}

func (u *updater) Get() (*v1.ConfigMap, error) {
	configMap, err := u.client.CoreV1().ConfigMaps(u.ns).Get(context.Background(), u.configMapName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get ConfigMap, err: %v", err)
		return nil, errors.Wrap(err, "get configMap")
	}
	return configMap, nil
}

func (u *updater) Update(cm *v1.ConfigMap) error {
	_, err := u.client.CoreV1().ConfigMaps(u.ns).Update(context.Background(), cm, metav1.UpdateOptions{})
	return err
}

func (u *updater) UpdateWithRetry(cm *v1.ConfigMap, retryCount int, retryInterval time.Duration) (err error) {
	for i := 0; i < retryCount; i++ {
		err = u.Update(cm)
		if err == nil {
			return nil
		}
		log.Warnf("update configmap %s/%s error: %v, retry", u.ns, cm.Name, err)
		time.Sleep(retryInterval)
	}
	return err
}
