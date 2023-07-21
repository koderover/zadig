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

package klock

import (
	"context"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krclient "github.com/koderover/zadig/pkg/tool/kube/client"
)

const (
	CreateTimeKey = "create_time"
)

var (
	DefaultTTL           = 60 * time.Second
	DefaultTimeout       = 3 * time.Second
	DefaultRetryInterval = 100 * time.Millisecond
)

var c *Client

type Client struct {
	client.Client
	namespace string
}

func Init(namespace string) error {
	c = &Client{
		Client:    krclient.Client(),
		namespace: namespace,
	}
	return nil
}

func Lock(key string) error {
	if c == nil {
		return ErrNotInit
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	configMap := configMapBuilder(key, c.namespace)
	if err := c.Create(ctx, configMap); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return ErrLockExist
		}
		return err
	}
	return nil
}

func checkLockTTLAndRemove(key string) {
	configMap := &corev1.ConfigMap{}
	err := c.Get(context.Background(), client.ObjectKey{
		Namespace: c.namespace,
		Name:      key,
	}, configMap)
	if err != nil {
		return
	}
	if configMap.Data == nil || configMap.Data[CreateTimeKey] == "" {
		_ = Unlock(key)
		return
	}

	createTime, err := strconv.ParseInt(configMap.Data[CreateTimeKey], 10, 64)
	if err != nil {
		_ = Unlock(key)
		return
	}
	if time.Now().Unix()-createTime > int64(DefaultTTL.Seconds()) {
		_ = Unlock(key)
	}
	return
}

func LockWithRetry(key string, retry int) error {
	if c == nil {
		return ErrNotInit
	}

	for i := 0; i < retry; i++ {
		if err := Lock(key); err != nil {
			if err == ErrLockExist {
				time.Sleep(DefaultRetryInterval)
				continue
			}
			return err
		}
		return nil
	}
	checkLockTTLAndRemove(key)
	return ErrCreateLockTimeout
}

func Unlock(key string) error {
	if c == nil {
		return ErrNotInit
	}

	configMap := configMapBuilder(key, c.namespace)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	if err := c.Delete(ctx, configMap); err != nil {
		return err
	}
	return nil
}

func configMapBuilder(key, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key,
			Namespace: namespace,
		},
		Data: map[string]string{
			CreateTimeKey: strconv.FormatInt(time.Now().Unix(), 10),
		},
	}
}
