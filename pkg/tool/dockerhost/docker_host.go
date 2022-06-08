/*
Copyright 2022 The KodeRover Authors.

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

package dockerhost

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
)

var once sync.Once

type Member string

func (m Member) String() string {
	return string(m)
}

type hasher struct{}

func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

type ClusterID string

type DockerHostsI interface {
	GetBestHost(ClusterID, string) string

	Sync()
}

type dockerhosts struct {
	rwLock        *sync.RWMutex
	store         map[ClusterID]*consistent.Consistent
	hubServerAddr string
	syncInterval  time.Duration

	logger *zap.SugaredLogger
}

var dockerHosts DockerHostsI

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func NewDockerHosts(hubServerAddr string, logger *zap.SugaredLogger) DockerHostsI {
	once.Do(func() {
		dockerHosts = &dockerhosts{
			rwLock:        &sync.RWMutex{},
			store:         map[ClusterID]*consistent.Consistent{},
			hubServerAddr: hubServerAddr,
			logger:        logger,
		}

		go dockerHosts.Sync()
	})

	return dockerHosts
}

func (d *dockerhosts) GetBestHost(clusterID ClusterID, key string) string {
	if d.store[clusterID] == nil {
		d.initClusterInfo(clusterID)
	}

	member := d.store[clusterID].LocateKey([]byte(key))

	return member.String()
}

func (d *dockerhosts) initClusterInfo(clusterID ClusterID) {
	d.rwLock.Lock()
	defer d.rwLock.Unlock()

	members := d.getDockerHostsSvc(clusterID)

	cfg := consistent.Config{
		PartitionCount:    271,
		ReplicationFactor: 20,
		Load:              1.25,
		Hasher:            hasher{},
	}

	d.store[clusterID] = consistent.New(members, cfg)
}

func (d *dockerhosts) getDockerHostsSvc(clusterID ClusterID) []consistent.Member {
	ns := config.Namespace()
	if string(clusterID) != setting.LocalClusterID {
		ns = setting.AttachedClusterNamespace
	}

	kclient, err := kubeclient.GetKubeClient(d.hubServerAddr, string(clusterID))
	if err != nil {
		d.logger.Warnf("Failed to get kubeclient for cluster %q: %s. Try to use default dockerhosts.", clusterID, err)
		return d.getDefaultDockerHosts()
	}

	dindSts := &appsv1.StatefulSet{}
	err = kclient.Get(context.TODO(), client.ObjectKey{
		Name:      "dind",
		Namespace: ns,
	}, dindSts)
	if err != nil {
		d.logger.Warnf("Failed to get dind statefuleset in namespace %q of cluster %q: %s", ns, clusterID, err)
		return d.getDefaultDockerHosts()
	}

	members := []consistent.Member{}
	for i := 0; i < int(*dindSts.Spec.Replicas); i++ {
		members = append(members, d.genDindAddr(i))
	}

	return members
}

func (d *dockerhosts) getDefaultDockerHosts() []consistent.Member {
	return []consistent.Member{d.genDindAddr(0)}
}

func (d *dockerhosts) genDindAddr(idx int) Member {
	return Member(fmt.Sprintf("tcp://dind-%d.dind:2375", idx))
}

func (d *dockerhosts) Sync() {
	d.logger.Info("Begin to sync")

	wait.Forever(func() {
		d.rwLock.Lock()
		defer d.rwLock.Unlock()

		for clusterID, consistentHash := range d.store {
			currentMembers := d.getDockerHostsSvc(clusterID)
			oldMembers := consistentHash.GetMembers()

			d.logger.Infof("Cluster: %q. Current Members: %d. Old Members: %d", clusterID, len(currentMembers), len(oldMembers))

			addedMembers, deletedMembers := d.diffMembers(oldMembers, currentMembers)
			for _, member := range addedMembers {
				consistentHash.Add(member)
			}
			for _, member := range deletedMembers {
				consistentHash.Remove(member.String())
			}
		}
	}, 3*time.Minute)
}

func (d *dockerhosts) diffMembers(old, current []consistent.Member) (added, deleted []consistent.Member) {
	curMap := make(map[consistent.Member]struct{}, len(current))
	for _, member := range current {
		curMap[member] = struct{}{}
	}

	oldMap := make(map[consistent.Member]struct{}, len(old))
	for _, member := range old {
		oldMap[member] = struct{}{}
	}

	for _, member := range old {
		if _, found := curMap[member]; !found {
			deleted = append(deleted, member)
		}
	}
	for _, member := range current {
		if _, found := oldMap[member]; !found {
			added = append(added, member)
		}
	}

	return added, deleted
}
