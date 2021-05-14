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

package webhook

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/lib/tool/kube/serializer"
)

var testDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: test
spec:
  template:
    spec:
      containers:
      - image: test-image
        name: test
`

var testStatefulSet = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
  namespace: test
spec:
  template:
    spec:
      containers:
      - image: test-image
        name: test
`

var testJob = `
apiVersion: batch/v1
kind: Job
metadata:
  name: test
  namespace: test
spec:
  template:
    spec:
      containers:
      - image: test-image
        name: test
`

var _ = Describe("Testing utils", func() {

	Context("test getContainers", func() {
		It("should work for Deployment", func() {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(testDeployment))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(u.GetKind()).To(Equal("Deployment"))
			Expect(u.GetName()).To(Equal("test"))

			cs, err := getContainers(u)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cs).To(HaveLen(1))
			Expect(cs[0].Name).To(Equal("test"))
			Expect(cs[0].Image).To(Equal("test-image"))
		})

		It("should work for StatefulSet", func() {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(testStatefulSet))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(u.GetKind()).To(Equal("StatefulSet"))
			Expect(u.GetName()).To(Equal("test"))

			cs, err := getContainers(u)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cs).To(HaveLen(1))
			Expect(cs[0].Name).To(Equal("test"))
			Expect(cs[0].Image).To(Equal("test-image"))
		})

		It("should work for Job", func() {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(testJob))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(u.GetKind()).To(Equal("Job"))
			Expect(u.GetName()).To(Equal("test"))

			cs, err := getContainers(u)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cs).To(HaveLen(1))
			Expect(cs[0].Name).To(Equal("test"))
			Expect(cs[0].Image).To(Equal("test-image"))
		})
	})
})
