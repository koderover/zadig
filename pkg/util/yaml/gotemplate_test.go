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
	"bytes"
	"text/template"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var templateYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: service-1
  labels: 
    app.kubernetes.io/name: project-1
    app.kubernetes.io/instance: service-1
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: project-1
      app.kubernetes.io/instance: service-1
  template:
    metadata: 
      labels:
        app.kubernetes.io/name: {{.base.property}}
        app.kubernetes.io/instance: service-1
    spec:
      containers:
        - name: service-1
          image: ccr.ccs.tencentyun.com/koderover-public/service-1:latest
          imagePullPolicy: Always 
          env:
            - name: DOWNSTREAM_ADDR
              value: "hahaha"
            - name: HEADERS
              value: "x-request-id"
            {{- if eq .skywalking "hello" }}
            - name: ENHANCE
              value: "true"
            {{- end}}
			{{- if eq .hello 2 }}
            - name: ENHANCE2
              value: "true"
            {{- end}}
          command:
            - /workspace/{{.newcmd}}
          ports:
          {{- range .ports_config}}
            - protocol: {{ .protocol }}
              containerPort: {{.container_port}}
          {{- end}}
          resources:
            limits:
              memory: {{.memory_limit}}
              cpu: {{.cpu_limit}}
`

var _ = Describe("Testing go template", func() {
	Context("extracting go template variables", func() {
		It("extracted variables should cover all keys", func() {

			variableYaml, err := ExtractVariableYaml(templateYaml)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			valuesMap := make(map[string]interface{})
			err = yaml.Unmarshal([]byte(variableYaml), &valuesMap)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			tpl, err := template.New("test").Parse(templateYaml)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// execute source file should work
			buffer := bytes.NewBufferString("")
			err = tpl.Execute(buffer, valuesMap)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// the result of rendered template should be legal yaml
			outMap := make(map[string]interface{})
			err = yaml.Unmarshal(buffer.Bytes(), &outMap)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
	})
})
