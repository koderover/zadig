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
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

var testYaml = `
# Default values for go-sample-site.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

env: dev

ingressClassName: koderover-admin-nginx

image:
  - repository: ccr.ccs.tencentyun.com/trial/go-sample-site
    pullPolicy: IfNotPresent
    tag: "0.1.0"

imageNormal:
    repository: ccr.ccs.tencentyun.com/trial/go-sample-site
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

func TestReplaceImage(t *testing.T) {
	vMap := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(testYaml), &vMap)
	if err != nil {
		t.Fatal(err)
	}

	replaceMap := map[string]interface{}{
		"image[0].repository": "newImageName",
		"image[0].tag":        "latest",

		"imageNormal.repository": "newNormalImageName",
		"imageNormal.tag":        "2.1.0",
	}

	replacedYaml, err := replaceImage(testYaml, replaceMap)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(replacedYaml)
}
