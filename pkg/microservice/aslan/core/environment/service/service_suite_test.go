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

package service

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"

	// init test env first
	_ "github.com/koderover/zadig/v2/pkg/util/testing"
)

func TestRoutes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "environment service Suite")
}

func TestGetHelmServiceByNameRejectsAmbiguousTarget(t *testing.T) {
	env := &commonmodels.Product{
		EnvName: "dev",
		Source:  setting.SourceFromHelm,
		Services: [][]*commonmodels.ProductService{{
			{ServiceName: "api", Type: setting.HelmDeployType},
			{ServiceName: "chart", ReleaseName: "api", Type: setting.HelmChartDeployType},
			{ServiceName: "chart", ReleaseName: "api-canary", Type: setting.HelmChartDeployType},
		}},
	}
	for name, want := range map[string]string{
		"api":     "matches multiple Helm services",
		"chart":   "matches multiple Helm services",
		"missing": "failed to find service missing",
	} {
		if _, err := getHelmServiceByName(env, name, zap.NewNop().Sugar()); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("getHelmServiceByName(%q) error = %v, want %q", name, err, want)
		}
	}
}
