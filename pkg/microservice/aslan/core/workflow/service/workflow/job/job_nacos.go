/*
Copyright 2023 The KodeRover Authors.

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

package job

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type NacosJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.NacosJobSpec
}

func (j *NacosJob) Instantiate() error {
	j.spec = &commonmodels.NacosJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *NacosJob) SetPreset() error {
	j.spec = &commonmodels.NacosJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	j.job.Spec = j.spec
	originNamespaceID := strings.ReplaceAll(j.spec.NamespaceID, setting.FixedValueMark, "")

	nacosConfigs, err := commonservice.ListNacosConfig(j.spec.NacosID, originNamespaceID, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("fail to list nacos config: %w", err)
	}

	nacosConfigsMap := map[string]*types.NacosConfig{}
	for _, config := range nacosConfigs {
		nacosConfigsMap[getNacosConfigKey(config.Group, config.DataID)] = config
	}

	var configSet sets.String
	if strings.HasPrefix(j.spec.NamespaceID, setting.FixedValueMark) {
		configSet = sets.NewString(j.spec.NacosDataRange...)
	}

	newDatas := []*types.NacosConfig{}
	for _, data := range j.spec.NacosDatas {
		if !isNacosDataFiltered(data, configSet) {
			continue
		}

		newData, ok := nacosConfigsMap[getNacosConfigKey(data.Group, data.DataID)]
		if !ok {
			log.Errorf("can't find nacos config %s/%s", data.DataID, data.Group)
			continue
		}
		newDatas = append(newDatas, newData)
	}

	newFilterDatas := []*types.NacosConfig{}
	for _, data := range nacosConfigsMap {
		if !isNacosDataFiltered(data, configSet) {
			continue
		}

		newFilterDatas = append(newFilterDatas, data)
	}

	j.spec.NacosDatas = newDatas
	j.spec.NacosFilteredData = newFilterDatas
	return nil
}

func (j *NacosJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.NacosJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.NacosJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.NamespaceID = argsSpec.NamespaceID
		if !j.spec.DataFixed {
			j.spec.NacosDatas = argsSpec.NacosDatas
		}
	}
	return nil
}

func (j *NacosJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.NacosJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	info, err := mongodb.NewConfigurationManagementColl().GetNacosByID(context.Background(), j.spec.NacosID)
	if err != nil {
		return nil, errors.Wrap(err, "get nacos info")
	}
	client, err := commonservice.GetNacosClient(j.spec.NacosID)
	if err != nil {
		return nil, errors.Errorf("get nacos client error: %v", err)
	}
	namespaces, err := client.ListNamespaces()
	if err != nil {
		return nil, err
	}
	namespaceName := ""
	for _, namespace := range namespaces {
		if namespace.NamespaceID == j.spec.NamespaceID {
			namespaceName = namespace.NamespacedName
			break
		}
	}

	jobTask := &commonmodels.JobTask{
		Name: j.job.Name,
		Key:  j.job.Name,
		JobInfo: map[string]string{
			JobNameKey: j.job.Name,
		},
		JobType: string(config.JobNacos),
		Spec: commonmodels.JobTaskNacosSpec{
			NacosID:       j.spec.NacosID,
			NamespaceID:   j.spec.NamespaceID,
			NamespaceName: namespaceName,
			NacosAddr:     info.ServerAddress,
			UserName:      client.UserName,
			Password:      client.Password,
			NacosDatas:    transNacosDatas(j.spec.NacosDatas),
		},
	}

	return []*commonmodels.JobTask{jobTask}, nil
}

func (j *NacosJob) LintJob() error {
	j.spec = &commonmodels.NacosJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}

	if strings.HasPrefix(j.spec.NamespaceID, setting.FixedValueMark) {
		return nil
	}

	configSet := sets.NewString(j.spec.NacosDataRange...)
	for _, data := range j.spec.NacosDatas {
		if !isNacosDataFiltered(data, configSet) {
			return fmt.Errorf("can't select the nacos config outside the config range, key: %s", getNacosConfigKey(data.Group, data.DataID))
		}
	}

	return nil
}

func transNacosDatas(confs []*types.NacosConfig) []*commonmodels.NacosData {
	resp := []*commonmodels.NacosData{}
	for _, conf := range confs {
		resp = append(resp, &commonmodels.NacosData{
			NacosConfig: *conf,
		})
	}
	return resp
}

func getNacosConfigKey(group, id string) string {
	return group + "/" + id
}

func filterNacosDatas(inputDatas []*types.NacosConfig, filters []string) []*types.NacosConfig {
	resp := []*types.NacosConfig{}
	set := sets.NewString(filters...)
	for _, data := range inputDatas {
		if isNacosDataFiltered(data, set) {
			resp = append(resp, data)
		}
	}
	return resp
}

func isNacosDataFiltered(data *types.NacosConfig, filters sets.String) bool {
	if filters.Len() == 0 {
		return true
	}

	if filters.Has("*") {
		return true
	}

	if filters.Has(getNacosConfigKey(data.Group, data.DataID)) {
		return true
	}

	return false
}
