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

package yaml_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

var testYaml1 = `
env: dev
image:
  repository: go-sample-site
  tag: "0.2.1"
imagePullSecrets:
  - name: default-secret
`

var testYaml2 = `
env: dev1
image:
  tag: 0.2.2
imagePullSecrets:
  - name: default-secret1
`

var testYaml3 = `
env: dev1
image:
  repository: go-sample-site
  tag: "0.2.2"
imagePullSecrets:
  - name: default-secret1
`

var _ = Describe("Testing merge", func() {

	Context("merge to yamls into one", func() {
		It("should work as expected", func() {
			actual, err := yamlutil.Merge([][]byte{[]byte(testYaml1), []byte(testYaml2)})
			Expect(err).ShouldNot(HaveOccurred())

			currentMap := map[string]interface{}{}
			err = yaml.Unmarshal([]byte(testYaml3), &currentMap)
			Expect(err).ShouldNot(HaveOccurred())

			expected, err := yaml.Marshal(currentMap)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(actual)).To(Equal(string(expected)))
		})

	})
})
