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

package taskplugin

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/pkg/util/converter"
)

var testYaml = `
# Default values for go-sample-site.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

env: dev

ingressClassName: koderover-admin-nginx

image:
  - repository: koderover.tencentcloudcr.com/test/go-sample-site
    pullPolicy: IfNotPresent
    tag: "0.1.0"

imageNormal:
    repository: koderover.tencentcloudcr.com/test/go-sample-site
    pullPolicy: IfNotPresent
    tag: "0.1.0"

imagePullSecrets:
  - name: default-registry-secret

nameOverride: ""
fullnameOverride: ""

service:
  type: ClusterIP
  port: 8080

`

var _ = Describe("Testing replace image", func() {
	Context("replace image", func() {

		It("replace normal", func() {
			replaceMap := map[string]interface{}{
				"imageNormal.repository": "newNormalImageName",
				"imageNormal.tag":        "2.1.0",
			}

			replacedYaml, err := replaceImage(testYaml, replaceMap)
			Expect(err).NotTo(HaveOccurred())

			flatMap, err := converter.YamlToFlatMap([]byte(replacedYaml))
			Expect(err).NotTo(HaveOccurred())

			Expect(flatMap["imageNormal.repository"]).To(Equal("newNormalImageName"))
			Expect(flatMap["imageNormal.tag"]).To(Equal("2.1.0"))
		})

		It("replace array situation", func() {
			replaceMap := map[string]interface{}{
				"image[0].repository": "newImageName",
				"image[0].tag":        "latest",
			}

			replacedYaml, err := replaceImage(testYaml, replaceMap)
			Expect(err).NotTo(HaveOccurred())

			flatMap, err := converter.YamlToFlatMap([]byte(replacedYaml))
			Expect(err).NotTo(HaveOccurred())

			Expect(flatMap["image[0].repository"]).To(Equal("newImageName"))
			Expect(flatMap["image[0].tag"]).To(Equal("latest"))
		})

	})
})
