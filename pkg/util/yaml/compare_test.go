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

package yaml

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var fromYaml = `
env: dev
image:
  repository: go-sample-site
  tag: "0.2.1"
imagePullSecrets:
  - name: default-secret
`

var toYaml = `
imagePullSecrets:
  - name: default-secret
env: dev
image:
  repository: go-sample-site
  tag: "0.2.1"
`

var toYaml2 = `
# 这是一个注释
# 这是一个注释2
imagePullSecrets:
  - name: default-secret
env: dev
image:
  repository: go-sample-site
  tag: "0.2.1"
`

var _ = Describe("Testing yaml equal", func() {

	Context("yaml equal", func() {
		It("should work as expected", func() {
			equal, err := Equal(fromYaml, toYaml)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(equal).Should(gomega.BeTrue())
		})

		It("should work as expected", func() {
			equal, err := Equal(fromYaml, toYaml2)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(equal).Should(gomega.BeTrue())
		})

		It("should work as expected", func() {
			equal, err := Equal(toYaml, toYaml2)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(equal).Should(gomega.BeTrue())
		})

	})
})
