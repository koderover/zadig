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
	"math/rand"
	"testing"
	"time"

	"go.uber.org/zap"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func TestNewDockerHosts(t *testing.T) {
	hubServerAddr := "hub-server.zadig"
	logger, _ := zap.NewProduction()
	dockerhosts := NewDockerHosts(hubServerAddr, logger.Sugar())

	clusters := []ClusterID{ClusterID("local"), ClusterID("worker0"), ClusterID("worker1")}
	svcs := []string{"svc0", "svc1", "svc2", "svc3", "svc4"}

	for i := 0; i < len(clusters); i++ {
		t.Logf("In Cluster %q\n", clusters[i])

		for j := 0; j < 10; j++ {
			svc := svcs[rand.Intn(len(svcs))]
			host := dockerhosts.GetBestHost(clusters[i], svc)
			t.Logf("\tHost for svc %q: %q\n", svc, host)
		}

		t.Log("\n")
	}
}
