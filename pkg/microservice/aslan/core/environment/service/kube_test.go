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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/koderover/zadig/pkg/tool/kube/serializer"
)

var testConfigMap1 = `
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    koderover.io/editor-id: "41"
    koderover.io/last-update-time: "2021-06-03T07:43:54Z"
    koderover.io/modified-by: test_user
  labels:
    koderover.io/modified-since-last-update: "true"
    s-product: test_product
    s-service: test_service
  name: test1
`

var testConfigMap2 = `
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    koderover.io/editor-id: "41"
    koderover.io/last-update-time: "2021-06-04T07:43:54Z"
    koderover.io/modified-by: test_user2
  labels:
    koderover.io/modified-since-last-update: "true"
    s-product: test_product
    s-service: test_service
  name: test2
`

var _ = Describe("Testing kube", func() {

	Describe("test getModifiedServiceFromObjectMetaList", func() {

		Context("there is one dirty object", func() {
			It("should return right result", func() {
				cm, err := serializer.NewDecoder().YamlToConfigMap([]byte(testConfigMap1))
				Expect(err).ShouldNot(HaveOccurred())

				sis := getModifiedServiceFromObjectMetaList([]metav1.Object{cm})
				Expect(sis).To(HaveLen(1))
				Expect(sis[0].Name).To(Equal("test_service"))
				Expect(sis[0].ModifiedBy).To(Equal("test_user"))
			})
		})

		Context("there is two dirty objects in one service", func() {
			It("should return the newer one", func() {
				cm1, err := serializer.NewDecoder().YamlToConfigMap([]byte(testConfigMap1))
				Expect(err).ShouldNot(HaveOccurred())
				cm2, err := serializer.NewDecoder().YamlToConfigMap([]byte(testConfigMap2))
				Expect(err).ShouldNot(HaveOccurred())

				sis := getModifiedServiceFromObjectMetaList([]metav1.Object{cm1, cm2})
				Expect(sis).To(HaveLen(1))
				Expect(sis[0].Name).To(Equal("test_service"))
				Expect(sis[0].ModifiedBy).To(Equal("test_user2"))
			})
		})
	})

})
