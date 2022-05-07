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

package test_multi_envs

import (
	"net/http"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMultiEnvs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestDemo Suite")
}

type SiteConfig struct {
	WWWSite     string
	DocSite     string
	InvalidSite string
}

func GetConfig() SiteConfig {
	if os.Getenv("ENV_NAME") == "test" {
		return SiteConfig{
			WWWSite:     "https://www.koderover-test.com",
			DocSite:     "https://docs.koderover-test.com",
			InvalidSite: "https://www.koderover-test.com.cn",
		}
	}
	return SiteConfig{
		WWWSite:     "https://www.koderover.com",
		DocSite:     "https://docs.koderover.com",
		InvalidSite: "https://www.koderover.com.cn",
	}
}

func LifeProbe(host string) (int, error) {
	req, _ := http.NewRequest("GET", host, nil)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, err
}

var _ = Describe("ZadigWebsitesLifeProbe", func() {
	It("Case1: WWW website should be OK", func() {
		host := GetConfig().WWWSite
		code, err := LifeProbe(host)
		Expect(err).Should(BeNil())
		Expect(code).Should(Equal(200))
	})

	It("Case2: Docs website should be OK", func() {
		host := GetConfig().DocSite
		code, err := LifeProbe(host)
		Expect(err).Should(BeNil())
		Expect(code).Should(Equal(200))
	})

	It("Case3: Sorry, invalid site won't work", func() {
		host := GetConfig().InvalidSite
		_, err := LifeProbe(host)
		Expect(err).Should(Not(BeNil()))
	})
})
