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
	"github.com/onsi/gomega"

	"github.com/koderover/zadig/pkg/util/converter"
)

var testYaml1 = `
env: dev
image:
  repo: repo.com
  name: go-sample-site
  tag: "0.2.1"
imagePullSecrets:
  - name: default-secret
`

var testYaml2 = `
env: dev
svc1: 
  image:
    repo: repo.com
    name: go-sample-site
    tag: "0.2.1"
svc2:
  image:
    repo: repo.com
    name: go-sample-site
    tag: "0.2.1"
imagePullSecrets:
  - name: default-secret
`

var testYaml3 = `
env: dev
svc1: 
  image:
    repo: repo.com
    name: go-sample-site
    tag: "0.2.1"
svc2:
  image:
    repo: repo.com
    detail:
      name: go-sample-site
      tag: "0.2.1"
fakeSvc:
  repo: notrepo.com
imagePullSecrets:
  - name: default-secret
`

var testYaml8 = `
env: dev

repoData1:
  global:
    hub: koderover.tencentcloudcr.com/koderover-public 

testSpec:
  image:
    image: go-sample-site-new
    pullPolicy: IfNotPresent
    tag: 0.1.0

svc1:
  image: svc1-image
  tag: 0.2.0

svc2:
  image:
    image: svc2-image
    tag: 0.3.0

imagePullSecrets:
  - name: default-registry-secret

nameOverride: ""
fullnameOverride: ""

service:
  type: ClusterIP
  port: 8080
`

var testYaml9 = `
  image:
    pullPolicy: IfNotPresent

  imagePullSecrets:
    - name: default-registry-secret

  global:
    hub: koderover.tencentcloudcr.com/test

  deploy:
    image:
      name: go-sample-site
      tag: 0.1.0 
`

var err error
var lcpMatedPaths []map[string]string

var _ = Describe("Testing search", func() {
	Context("search matched paths from yaml", func() {
		It("single absolute match", func() {
			pattern := []map[string]string{
				{"repo": "repo", "tag": "tag", "image": "name"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml1))
			lcpMatedPaths, err = SearchByPattern(flatMap, pattern)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(lcpMatedPaths).Should(gomega.ConsistOf([]map[string]string{
				{"image": "image.name", "repo": "image.repo", "tag": "image.tag"},
			}))
		})

		It("single relative match", func() {
			pattern := []map[string]string{
				{"repo": "image.repo", "tag": "image.tag", "image": "image.name"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml1))
			lcpMatedPaths, err = SearchByPattern(flatMap, pattern)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(lcpMatedPaths).Should(gomega.ConsistOf([]map[string]string{
				{"image": "image.name", "repo": "image.repo", "tag": "image.tag"},
			}))
		})

		It("multiple match", func() {
			pattern := []map[string]string{
				{"repo": "repo", "tag": "tag", "image": "name"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml2))

			lcpMatedPaths, err = SearchByPattern(flatMap, pattern)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(lcpMatedPaths).Should(gomega.ConsistOf([]map[string]string{
				{"image": "svc1.image.name", "repo": "svc1.image.repo", "tag": "svc1.image.tag"},
				{"image": "svc2.image.name", "repo": "svc2.image.repo", "tag": "svc2.image.tag"},
			}))
		})

		It("complex multiple match", func() {
			pattern := []map[string]string{
				{"repo": "repo", "tag": "tag", "image": "name"},
			}
			flatMap, err := converter.YamlToFlatMap([]byte(testYaml3))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			lcpMatedPaths, err = SearchByPattern(flatMap, pattern)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(lcpMatedPaths).Should(gomega.ConsistOf([]map[string]string{
				{"image": "svc1.image.name", "repo": "svc1.image.repo", "tag": "svc1.image.tag"},
				{"image": "svc2.image.detail.name", "repo": "svc2.image.repo", "tag": "svc2.image.detail.tag"},
			}))
		})

		It("complex match with pattern component reuse", func() {
			pattern := []map[string]string{
				{"image": "image", "tag": "tag", "repo": "global.hub"},
			}
			flatMap, err := converter.YamlToFlatMap([]byte(testYaml8))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			lcpMatedPaths, err = SearchByPattern(flatMap, pattern)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(lcpMatedPaths).Should(gomega.ConsistOf([]map[string]string{
				{"image": "testSpec.image.image", "repo": "repoData1.global.hub", "tag": "testSpec.image.tag"},
				{"image": "svc1.image", "repo": "repoData1.global.hub", "tag": "svc1.tag"},
				{"image": "svc2.image.image", "repo": "repoData1.global.hub", "tag": "svc2.image.tag"},
			}))
		})

		It("match all rules of pattern", func() {
			pattern := []map[string]string{
				{"image": "image.repository", "tag": "image.tag"},
				{"image": "image"},
				{"repo": "global.hub", "tag": "deploy.image.tag", "image": "deploy.image.name"},
			}
			flatMap, _ := converter.YamlToFlatMap([]byte(testYaml9))
			lcpMatedPaths, err = SearchByPattern(flatMap, pattern)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(lcpMatedPaths).Should(gomega.ConsistOf([]map[string]string{
				{"image": "deploy.image.name", "repo": "global.hub", "tag": "deploy.image.tag"},
			}))
		})

	})
})
