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

package client

import (
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
)

var clientOnce sync.Once
var clientset kubernetes.Interface

// Clientset is a singleton, it will be initialized only once.
func Clientset() kubernetes.Interface {
	clientOnce.Do(func() {
		clientset = initClientset()
	})

	return clientset
}

func NewClientsetFromAPIConfig(cfg *api.Config) (kubernetes.Interface, error) {
	restConfig, err := RESTConfigFromAPIConfig(cfg)
	if err != nil {
		return nil, err
	}

	cl, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return cl, nil
}

func initClientset() kubernetes.Interface {
	c, err := kubernetes.NewForConfig(RESTConfig())
	if err != nil {
		panic(err)
	}

	return c
}
