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

package serializer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/lib/tool/kube/serializer"
)

var testDeployment = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: test
  namespace: test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
      tier: backend
  template:
    metadata:
      labels:
        app: postgres
        tier: backend
    spec:
      containers:
      - image: ccr.ccs.tencentyun.com/koderover-public/library-postgres:9.6
        imagePullPolicy: IfNotPresent
        name: postgres
        ports:
        - containerPort: 5432
          protocol: TCP
`

var testService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    app: postgres
    tier: backend
  name: test
  namespace: test
spec:
  ports:
  - nodePort: 31762
    port: 5432
    protocol: TCP
    targetPort: 5432
  selector:
    app: postgres
    tier: backend
  type: NodePort
`

var testSecret = `
apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: test
type: kubernetes.io/service-account-token
data:
  token: test
`

var _ = Describe("Testing decoder", func() {

	Context("convert yaml to Unstructured", func() {
		It("should work for Deployment", func() {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(testDeployment))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(u.GetKind()).To(Equal("Deployment"))
			Expect(u.GetName()).To(Equal("test"))
		})

		It("should work for Service", func() {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(testService))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(u.GetKind()).To(Equal("Service"))
			Expect(u.GetName()).To(Equal("test"))
		})

		It("should work for Secret", func() {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(testSecret))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(u.GetKind()).To(Equal("Secret"))
			Expect(u.GetName()).To(Equal("test"))
		})
	})
})
