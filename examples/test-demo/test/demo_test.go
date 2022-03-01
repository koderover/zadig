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

package test

import (
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDemo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TestDemo Suite")
}

func HelloGeek(host string) (int, error) {
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
	It("Case1: Hello, Welcome to https://www.koderover.com", func() {
		host := "https://www.koderover.com"
		code, err := HelloGeek(host)
		Expect(err).Should(BeNil())
		Expect(code).Should(Equal(200))
	})

	It("Case2: Hello, Welcome to https://docs.koderover.com/", func() {
		host := "https://docs.koderover.com/"
		code, err := HelloGeek(host)
		Expect(err).Should(BeNil())
		Expect(code).Should(Equal(200))
	})

	It("Case3: Sorry, https://www.koderover.com.cn won't work", func() {
		host := "https://www.koderover.com.cn"
		_, err := HelloGeek(host)
		Expect(err).Should(Not(BeNil()))
	})
})
