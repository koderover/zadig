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
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/strvals"
	"sigs.k8s.io/yaml"
)

// GetValuesMap returns the mapped out values of a chart
func (spec *ChartSpec) GetValuesMap() (map[string]interface{}, error) {
	var values map[string]interface{}

	err := yaml.Unmarshal([]byte(spec.ValuesYaml), &values)
	if err != nil {
		return nil, err
	}

	// coalesce override values and values.yaml, helm `--set` option
	if spec.ValuesOverride != "" {
		// turn key=value1,key2=values2 to map
		ovals, err := strvals.Parse(spec.ValuesOverride)
		if err != nil {
			return nil, err
		}

		// coalesce values.yaml and override values
		coalescedValues := chartutil.CoalesceTables(make(map[string]interface{}, len(ovals)), ovals)
		values = chartutil.CoalesceTables(coalescedValues, values)
	}

	return values, nil
}
