package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

const (
	EnvironmentType = "Environment"
	ClusterType     = "Cluster"
)

type resourceBundleService struct {
	endpoint, resourceType string
}

var resourceBundleMap = map[string]resourceBundleService{
	EnvironmentType: {resourceType: EnvironmentType, endpoint: fmt.Sprintf("%s/api/environment/bundle-resources", config.AslanServiceAddress())},
	ClusterType:     {resourceType: ClusterType, endpoint: fmt.Sprintf("%s/api/cluster/bundle-resources", config.AslanServiceAddress())},
}

type ResourceSpec struct {
	ResourceID  string                 `json:"resourceID"`
	ProjectName string                 `json:"projectName"`
	Spec        map[string]interface{} `json:"spec"`
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

	cl := httpclient.New()
	for resourceType, bundleService := range resourceBundleMap {
		objs := make([]*ResourceSpec, 0)
		_, err := cl.Get(bundleService.endpoint, httpclient.SetResult(&objs))
		if err != nil {
			log.Warnf("Failed to get %s bundle, err: %s", resourceType, err)
			continue
		}

		res = AppendOPAResources(res, resourceType, objs)
	}

	return res
}
