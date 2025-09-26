/*
Copyright 2025 The KodeRover Authors.

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

package service

import (
	"sync"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type InstanceMember string

func (m InstanceMember) String() string {
	return string(m)
}

type instanceHasher struct{}

func (h instanceHasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

var (
	instanceConsistentHash *consistent.Consistent
	instanceHashMutex      sync.RWMutex
	instanceInitOnce       sync.Once

	instanceCfg = consistent.Config{
		PartitionCount:    7,
		ReplicationFactor: 20,
		Load:              1.25,
		Hasher:            instanceHasher{},
	}
)

// InitInstanceRouting initializes the consistent hashing for instance routing
func InitInstanceRouting() {
	instanceInitOnce.Do(func() {
		instanceConsistentHash = consistent.New(nil, instanceCfg)
		go monitorAslanInstances()
	})
}

// GetInstanceForKey returns the instance that should handle the given key
func GetInstanceForKey(key string) string {
	instanceHashMutex.RLock()
	defer instanceHashMutex.RUnlock()

	if instanceConsistentHash == nil {
		return config.PodIP()
	}

	member := instanceConsistentHash.LocateKey([]byte(key))
	if member == nil {
		return config.PodIP()
	}
	return member.String()
}

// IsCurrentInstanceResponsible checks if current instance should handle the given key
func IsCurrentInstanceResponsible(key string) bool {
	expectedInstance := GetInstanceForKey(key)
	currentInstance := config.PodIP()
	return expectedInstance == currentInstance
}

// GetCurrentInstanceID returns the current instance identifier
func GetCurrentInstanceID() string {
	return config.PodIP()
}

// monitorAslanInstances monitors aslan pod instances and updates the consistent hash ring
func monitorAslanInstances() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		updateInstanceHashRing()
	}
}

func updateInstanceHashRing() {
	logger := log.SugaredLogger()

	informer, err := clientmanager.NewKubeClientManager().GetInformer(setting.LocalClusterID, config.Namespace())
	if err != nil {
		logger.Errorf("failed to get informer for instance monitoring: %v", err)
		return
	}

	selector := labels.SelectorFromSet(labels.Set{
		"app.kubernetes.io/component": "aslan",
		"app.kubernetes.io/name":      "zadig",
	})
	pods, err := getter.ListPodsWithCache(selector, informer)
	if err != nil {
		logger.Errorf("failed to list aslan pods: %v", err)
		return
	}

	var readyIPs []string
	for _, pod := range pods {
		if wrapper.Pod(pod).Ready() {
			readyIPs = append(readyIPs, wrapper.Pod(pod).Status.PodIP)
		}
	}

	if len(readyIPs) == 0 {
		logger.Warn("no ready aslan instances found")
		return
	}

	// Check if there are changes
	instanceHashMutex.RLock()
	currentMembers := make(map[string]struct{})
	if instanceConsistentHash != nil {
		for _, member := range instanceConsistentHash.GetMembers() {
			currentMembers[member.String()] = struct{}{}
		}
	}

	hasChange := false
	newIPs := make(map[string]struct{})
	for _, ip := range readyIPs {
		newIPs[ip] = struct{}{}
		if _, exists := currentMembers[ip]; !exists {
			hasChange = true
		}
	}

	// Check for removed instances
	for member := range currentMembers {
		if _, exists := newIPs[member]; !exists {
			hasChange = true
		}
	}
	instanceHashMutex.RUnlock()

	// Update hash ring if there are changes
	if hasChange {
		instanceHashMutex.Lock()
		defer instanceHashMutex.Unlock()

		if instanceConsistentHash == nil {
			instanceConsistentHash = consistent.New(nil, instanceCfg)
		}

		// Clear existing members
		for _, member := range instanceConsistentHash.GetMembers() {
			instanceConsistentHash.Remove(member.String())
		}

		// Add new members
		for _, ip := range readyIPs {
			instanceConsistentHash.Add(InstanceMember(ip))
		}

		logger.Infof("Updated aslan instance hash ring with IPs: %v", readyIPs)
	}
}
