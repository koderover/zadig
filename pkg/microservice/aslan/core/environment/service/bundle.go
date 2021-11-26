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
	"sort"
	"strconv"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/opa"
)

const (
	resourcesPath = "resources/data.json"

	resourcesRoot = "resources"

	EnvironmentType = "Environment"
)

var revision string

type opaBundleMeta struct {
	ResourceID  string `json:"resourceID"`
	ProjectName string `json:"projectName"`
}

type opaMeta interface {
	Meta() *opaBundleMeta
}

type opaResources map[string]resources

type resources []opaMeta

func (o resources) Len() int      { return len(o) }
func (o resources) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o resources) Less(i, j int) bool {
	metaI := o[i].Meta()
	metaJ := o[j].Meta()
	if metaI.ProjectName == metaJ.ProjectName {
		return metaI.ResourceID < metaJ.ResourceID
	}
	return metaI.ProjectName < metaJ.ProjectName
}

func AppendOPAResources(res opaResources, resourceType string, objs []opaMeta) opaResources {
	if res == nil {
		res = make(map[string]resources)
	}

	sort.Sort(resources(objs))
	res[resourceType] = objs

	return res
}

func GenerateOPABundle() error {
	log.Info("Generating OPA bundle")
	defer log.Info("OPA bundle is generated")

	objs, err := GenerateEnvironmentBundle()
	if err != nil {
		log.Errorf("Failed to generate environment bundle, err: %s", err)
		return err
	}

	res := AppendOPAResources(nil, EnvironmentType, objs)

	bundle := &opa.Bundle{
		Data: []*opa.DataSpec{
			{Data: res, Path: resourcesPath},
		},
		Roots: []string{resourcesRoot},
	}

	hash, err := bundle.Rehash()
	if err != nil {
		log.Errorf("Failed to calculate bundle hash, err: %s", err)
		return err
	}
	revision = hash

	return bundle.Save(config.DataPath())
}

func GetRevision() string {
	return revision
}

type EnvironmentBundle struct {
	*opaBundleMeta
	Production string `json:"production"`
}

func (b *EnvironmentBundle) Meta() *opaBundleMeta {
	return b.opaBundleMeta
}

func GenerateEnvironmentBundle() ([]opaMeta, error) {
	var res []opaMeta

	envs, err := mongodb.NewProductColl().List(nil)
	if err != nil {
		log.Errorf("Failed to list envs, err: %s", err)
		return nil, err
	}

	clusterMap := make(map[string]*models.K8SCluster)
	clusters, err := mongodb.NewK8SClusterColl().Find("")
	if err != nil {
		log.Errorf("Failed to list clusters in db, err: %s", err)
		return nil, err
	}

	for _, cls := range clusters {
		clusterMap[cls.ID.Hex()] = cls
	}

	for _, env := range envs {
		clusterID := env.ClusterID
		production := false
		cluster, ok := clusterMap[clusterID]
		if ok {
			production = cluster.Production
		}

		res = append(res, &EnvironmentBundle{
			opaBundleMeta: &opaBundleMeta{
				ResourceID:  env.EnvName,
				ProjectName: env.ProductName,
			},
			Production: strconv.FormatBool(production),
		})
	}

	return res, nil
}
