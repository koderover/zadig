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
	"encoding/json"
	"fmt"
	"strings"

	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/log"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
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

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// MergeOverrideValues merge values.yaml and override values
// overrideYaml used for -f option
// overrideValues used for --set option
func MergeOverrideValues(valuesYaml, overrideYaml, overrideValues string) (string, error) {

	if overrideYaml == "" && overrideValues == "" {
		return valuesYaml, nil
	}

	// merge files for -f option
	values, err := yamlutil.MergeAndUnmarshal([][]byte{[]byte(valuesYaml), []byte(overrideYaml)})
	if err != nil {
		return "", err
	}

	if overrideValues != "" {
		kvList := make([]*KV, 0)
		err = json.Unmarshal([]byte(overrideValues), &kvList)
		if err != nil {
			return "", err
		}
		kvStr := make([]string, 0)
		for _, kv := range kvList {
			kvStr = append(kvStr, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
		}
		// override values for --set option
		err = strvals.ParseInto(strings.Join(kvStr, ","), values)
		if err != nil {
			return "", err
		}
	}

	bs, err := yaml.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
