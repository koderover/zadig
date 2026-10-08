/*
Copyright 2026 The KodeRover Authors.

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

package util

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	registrytool "github.com/koderover/zadig/v2/pkg/tool/registries"
)

var dindProxySyncLocks sync.Map

// SyncDinDProxy updates only proxy settings, independently of registry synchronization.
func SyncDinDProxy(clusterID, namespace string) error {
	v, _ := dindProxySyncLocks.LoadOrStore(clusterID+"/"+namespace, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	proxy, err := GetDinDProxy()
	if err != nil {
		return err
	}
	client, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return registrytool.UpdateDinDProxy(ctx, client, namespace, proxy)
}

// GetDinDProxy excludes registries managed by Zadig from the image pull proxy.
func GetDinDProxy() (*registrytool.DinDProxy, error) {
	proxies, err := mongodb.NewProxyColl().List(&mongodb.ProxyArgs{})
	if err != nil {
		return nil, fmt.Errorf("failed to list proxy to update dind: %w", err)
	}
	if len(proxies) == 0 || !proxies[0].EnableDinDProxy || proxies[0].Type == "no" {
		return &registrytool.DinDProxy{}, nil
	}
	registries, err := mongodb.NewRegistryNamespaceColl().FindAll(&mongodb.FindRegOps{})
	if err != nil {
		return nil, fmt.Errorf("failed to list registry to update dind proxy: %w", err)
	}

	noProxy := []string{"localhost", "127.0.0.1", ".svc", ".cluster.local", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	noProxy = append(noProxy, strings.FieldsFunc(proxies[0].DinDNoProxy, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})...)
	for _, reg := range registries {
		addr := reg.RegAddr
		if !strings.Contains(addr, "://") {
			addr = "https://" + addr
		}
		if u, err := url.Parse(addr); err == nil && u.Hostname() != "" {
			noProxy = append(noProxy, u.Hostname())
		}
	}

	proxyURL := proxies[0].GetProxyURL()
	return &registrytool.DinDProxy{
		HTTPProxy:  proxyURL,
		HTTPSProxy: proxyURL,
		NoProxy:    strings.Join(noProxy, ","),
	}, nil
}
