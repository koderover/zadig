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

package yaml

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/pkg/util/converter"
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
env: dev
svc1: 
  image:
    repository: go-sample-site
    tag: "0.2.1"
svc2:
  image:
    repository: go-sample-site-2
    tag: "0.2.2"
imagePullSecrets:
  - name: default-secret
`

var testYaml3 = `
env: dev
svc1: 
  image:
    repository: go-sample-site:0.2.1
svc2:
  image:
    repository: go-sample-site-2:0.2.2
svc3:
  image:
    repository: go-sample-site-3:0.2.3
imagePullSecrets:
  - name: default-secret
`

var testYaml4 = `
env: dev
svc1: 
  image:
    repository: go-sample-site
    tag: 0.2.1
svc2:
  image:
    repository: go-sample-site-2:0.2.2
svc3:
  image:
    repository: go-sample-site-3:0.2.3
svc4:
  image:
    repositoryNew: go-sample-site-3
    tagNew: 0.2.4
svc5:
  second:
    image:
      repositorySpec: go-sample-site-3
    tagNew: 0.2.4
imagePullSecrets:
  - name: default-secret
`

var testYaml5 = `
# Default values for go-sample-site.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

env: dev

ingressClassName: koderover-admin-nginx

image:
  repository: ccr.ccs.tencentyun.com/trial/go-sample-site
  pullPolicy: IfNotPresent
  tag: "0.1.0"

testSpec:
  imageNew:
    repo: ccr.ccs.tencentyun.com/trial/go-sample-site-new
    pullPolicy: IfNotPresent
    tag: "0.2.1"

imagePullSecrets:
  - name: default-registry-secret

nameOverride: ""
fullnameOverride: ""

service:
  type: ClusterIP
  port: 8080

`

var err error
var matedPaths []map[string]string

var _ = Describe("Testing search", func() {
	Context("search matched paths from yaml", func() {
		It("single match", func() {
			pattern := []map[string]string{
				{"image": "repository", "tag": "tag"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml1))
			matedPaths, err = SearchByPattern(flatMap, pattern)
			Expect(err).NotTo(HaveOccurred())
			Expect(matedPaths).To(Equal([]map[string]string{{"image": "image.repository", "tag": "image.tag"}}))
		})

		It("multiple match", func() {
			pattern := []map[string]string{
				{"image": "repository", "tag": "tag"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml2))
			matedPaths, err = SearchByPattern(flatMap, pattern)
			Expect(err).NotTo(HaveOccurred())

			Expect(matedPaths).Should(ConsistOf([]map[string]string{
				{"image": "svc1.image.repository", "tag": "svc1.image.tag"},
				{"image": "svc2.image.repository", "tag": "svc2.image.tag"},
			}))
		})

		It("multiple match pattern 3", func() {
			pattern := []map[string]string{
				{"image": "repository"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml3))
			matedPaths, err = SearchByPattern(flatMap, pattern)
			Expect(err).NotTo(HaveOccurred())
			Expect(matedPaths).Should(ConsistOf([]map[string]string{
				{"image": "svc1.image.repository"},
				{"image": "svc2.image.repository"},
				{"image": "svc3.image.repository"},
			}))
		})

		It("multiple match pattern complex", func() {
			pattern := []map[string]string{
				{"image": "repository", "tag": "tag"},
				{"image": "repository"},
				{"image": "repositoryNew", "tag": "tagNew"},
				{"image": "image.repositorySpec", "tag": "tagNew"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml4))
			matedPaths, err = SearchByPattern(flatMap, pattern)
			Expect(err).NotTo(HaveOccurred())
			Expect(matedPaths).Should(ConsistOf([]map[string]string{
				{"image": "svc1.image.repository", "tag": "svc1.image.tag"},
				{"image": "svc2.image.repository"},
				{"image": "svc3.image.repository"},
				{"image": "svc4.image.repositoryNew", "tag": "svc4.image.tagNew"},
				{"image": "svc5.second.image.repositorySpec", "tag": "svc5.second.tagNew"},
			}))

		})

		It("multiple match pattern complex2", func() {
			pattern := []map[string]string{
				{"image": "imageNew.repo", "tag": "imageNew.tag"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml5))
			matedPaths, err = SearchByPattern(flatMap, pattern)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(matedPaths)).To(Equal(1))
			Expect(matedPaths).To(Equal([]map[string]string{{"image": "testSpec.imageNew.repo", "tag": "testSpec.imageNew.tag"}}))
		})
	})
})
