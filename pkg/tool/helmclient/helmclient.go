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

package helmclient

import (
	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/log"
)

// NewClientFromRestConf returns a new Helm client constructed with the provided REST config options
func NewClientFromRestConf(restConfig *rest.Config, ns string) (hc.Client, error) {
	return hc.NewClientFromRestConf(&hc.RestConfClientOptions{
		Options: &hc.Options{
			Namespace: ns,
			DebugLog:  log.Debugf,
		},
		RestConfig: restConfig,
	})
}

// MergeOverrideValues merge values.yaml and override values
func MergeOverrideValues(valuesYaml, overrideValues string) (string, error) {

	if overrideValues == "" {
		return valuesYaml, nil
	}

	// turn key=value1,key2=values2 to map
	ovals, err := strvals.Parse(overrideValues)
	if err != nil {
		return "", err
	}

	values := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(valuesYaml), &values)
	if err != nil {
		return "", err
	}

	coalescedValues := chartutil.CoalesceTables(make(map[string]interface{}, len(ovals)), ovals)
	values = chartutil.CoalesceTables(coalescedValues, values)

	bs, err := yaml.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
