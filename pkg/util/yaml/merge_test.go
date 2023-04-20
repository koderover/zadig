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

// vars in tmplSvc
var testOverrideYaml1 = `
ip_hostnames:
- ip: 2.2.2.2
  hostnames: tes1.zadig.com
- ip: 3.3.3.3
  hostnames: tes2.zadig.com
`

// vars in render
var testOverrideYaml2 = `
ip_hostnames:
 - ip: 1.1.1.1
host_names:
- host_names1.zadig.com
hostnames:
- hostnames1.zadig.com
`

// merged vars
var testOverrideYaml3 = `
ip_hostnames:
- ip: 2.2.2.2
  hostnames: tes1.zadig.com
- ip: 3.3.3.3
  hostnames: tes2.zadig.com
host_names:
- host_names1.zadig.com
hostnames:
- hostnames1.zadig.com
`

var testFlatYaml1 = `
base:
  property: test1
container_port: 22
cpu_limit: 11
hello: 0
memory_limit: 22
newcmd: dd
ports_config: <nil>
protocol: http
skywalking: ""
`

var testFlatYaml2 = `
base:
  property:  test1
cpu_limit: 11
hello: 0
memory_limit: 22
newcmd: dd
ports_config:
  - container_port: 22
    protocol: http
skywalking: ""
a: b
`

var testFlatYaml3 = `
base:
  property:  test1
cpu_limit: 11
hello: 0
memory_limit: 22
newcmd: dd
ports_config:
  - container_port: 22
    protocol: http
skywalking: ""
container_port: 22
protocol: http
a: b
`

// vars in tmplSvc
var testSimpleYaml1 = `
A:
  - G: 2
  - G: 5
C:
D: 100
`

// vars in render
var testSimpleYaml2 = `
A:
  B: 1
C: 2
E: 123
`

// merged
var testSimpleYaml3 = `
A:
  - G: 2
  - G: 5
C:
D: 100
E: 123
`

var _ = Describe("Testing merge", func() {
	// [0]: yaml in tmplSvc
	// [1]: yaml in render
	// [2]: merged yaml
	var testCases [][3]string

	BeforeEach(func() {
		testCases = [][3]string{
			{
				testYaml1, testYaml2, testYaml3,
			},
			{
				testOverrideYaml2, testOverrideYaml1, testOverrideYaml3,
			},
			{
				testFlatYaml1, testFlatYaml2, testFlatYaml3,
			},
			{
				testSimpleYaml2, testSimpleYaml1, testSimpleYaml3,
			},
		}
	})

	Context("merge to yamls into one", func() {
		It("should work as expected", func() {
			for _, testCase := range testCases {
				actual, err := yamlutil.Merge([][]byte{[]byte(testCase[0]), []byte(testCase[1])})
				Expect(err).ShouldNot(HaveOccurred())

				currentMap := map[string]interface{}{}
				err = yaml.Unmarshal([]byte(testCase[2]), &currentMap)
				Expect(err).ShouldNot(HaveOccurred())

				expected, err := yaml.Marshal(currentMap)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(actual)).To(Equal(string(expected)))
			}
		})
	})
})
