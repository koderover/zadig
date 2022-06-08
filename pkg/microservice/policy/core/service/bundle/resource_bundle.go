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

package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

const configPath = "/config/config.yaml"

type ResourceBundleService struct {
	Endpoint     string `json:"endpoint"`
	ResourceType string `json:"resourceType"`
}

type Config struct {
	Bundles   []*ResourceBundleService `json:"bundles"`
	Namespace string                   `json:"namespace"`
}

type ResourceSpec struct {
	ResourceID  string   `json:"resourceID"`
	ProjectName string   `json:"projectName"`
	Spec        []string `json:"spec"`
}

type ResourceBundle map[string]resources

type resources []*ResourceSpec

func (o resources) Len() int      { return len(o) }
func (o resources) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o resources) Less(i, j int) bool {
	if o[i].ProjectName == o[j].ProjectName {
		return o[i].ResourceID < o[j].ResourceID
	}
	return o[i].ProjectName < o[j].ProjectName
}

func (r ResourceBundle) MarshalJSON() ([]byte, error) {
	var keys []string
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf := &bytes.Buffer{}
	buf.Write([]byte{'{'})
	for i, key := range keys {
		value := r[key]
		b, err := json.Marshal(&value)
		if err != nil {
			return nil, err
		}
		buf.WriteString(fmt.Sprintf("%q:", key))
		buf.Write(b)
		if i < len(keys)-1 {
			buf.Write([]byte{','})
		}
	}
	buf.Write([]byte{'}'})

	return buf.Bytes(), nil
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Errorf("Failed to load config, err: %s", err)
		return nil, err
	}

	conf := &Config{}
	if err = yaml.Unmarshal(data, conf); err != nil {
		log.Errorf("Failed to unmarshal yaml, err: %s", err)
		return nil, err
	}

	return conf, nil
}

func AppendOPAResources(res ResourceBundle, resourceType string, objs []*ResourceSpec) ResourceBundle {
	if res == nil {
		res = make(map[string]resources)
	}

	sort.Sort(resources(objs))
	res[resourceType] = objs

	return res
}

func generateResourceBundle() ResourceBundle {
	var res ResourceBundle

	cfg, err := loadConfig()
	if err != nil {
		return nil
	}

	cl := httpclient.New()
	for _, bundleService := range cfg.Bundles {
		objs := make([]*ResourceSpec, 0)
		_, err := cl.Get(bundleService.Endpoint, httpclient.SetResult(&objs))
		if err != nil {
			log.Warnf("Failed to get %s bundle, err: %s", bundleService.ResourceType, err)
			continue
		}

		res = AppendOPAResources(res, bundleService.ResourceType, objs)
	}

	return res
}
