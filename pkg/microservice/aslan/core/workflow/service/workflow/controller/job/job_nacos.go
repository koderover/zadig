/*
Copyright 2025 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type NacosJobController struct {
	*BasicInfo

	jobSpec *commonmodels.NacosJobSpec
}

func CreateNacosJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.NacosJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return NacosJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j NacosJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j NacosJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j NacosJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.NacosJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	configSet := sets.NewString(currJobSpec.NacosDataRange...)
	for _, data := range j.jobSpec.NacosDatas {
		if !isAllowedNacosData(data, configSet) {
			return fmt.Errorf("can't select the nacos config outside the config range, key: %s", getNacosConfigKey(data.Group, data.DataID))
		}
	}

	return nil
}

func (j NacosJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.NacosJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.NacosID = currJobSpec.NacosID
	j.jobSpec.Source = currJobSpec.Source
	j.jobSpec.DataFixed = currJobSpec.DataFixed
	j.jobSpec.NacosDataRange = currJobSpec.NacosDataRange
	if currJobSpec.Source == "fixed" {
		j.jobSpec.NamespaceID = currJobSpec.NamespaceID
	}

	if currJobSpec.DataFixed {
		newDataList := make([]*types.NacosConfig, 0)

		nacosConfigs, err := commonservice.ListNacosConfig(j.jobSpec.NacosID, j.jobSpec.NamespaceID, log.SugaredLogger())
		if err != nil {
			return fmt.Errorf("fail to list nacos config: %w", err)
		}

		namespaces, err := commonservice.ListNacosNamespace(j.jobSpec.NacosID, log.SugaredLogger())
		if err != nil {
			return fmt.Errorf("failed to list nacos namespace")
		}

		namespaceName := ""
		for _, namespace := range namespaces {
			if namespace.NamespaceID == j.jobSpec.NamespaceID {
				namespaceName = namespace.NamespacedName
				break
			}
		}

		nacosConfigsMap := make(map[string]*types.NacosConfig)
		for _, config := range nacosConfigs {
			config.NamespaceID = j.jobSpec.NamespaceID
			config.NamespaceName = namespaceName
			config.OriginalContent = config.Content

			nacosConfigsMap[getNacosConfigKey(config.Group, config.DataID)] = config
		}

		userInputMap := make(map[string]*types.NacosConfig)
		for _, userInput := range j.jobSpec.NacosDatas {
			userInputMap[getNacosConfigKey(userInput.Group, userInput.DataID)] = userInput
		}

		for _, fixedDataID := range currJobSpec.DefaultNacosDatas {
			if input, ok := userInputMap[getNacosConfigKey(fixedDataID.Group, fixedDataID.DataID)]; ok {
				newDataList = append(newDataList, input)
				continue
			}

			if nacosConfig, ok := nacosConfigsMap[getNacosConfigKey(fixedDataID.Group, fixedDataID.DataID)]; ok {
				newDataList = append(newDataList, nacosConfig)
				continue
			}

			return fmt.Errorf("config: %s is not found in nacos or user input", getNacosConfigKey(fixedDataID.Group, fixedDataID.DataID))
		}

		j.jobSpec.NacosDatas = newDataList
	}

	return nil
}

func (j NacosJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	nacosConfigs, err := commonservice.ListNacosConfig(j.jobSpec.NacosID, j.jobSpec.NamespaceID, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("fail to list nacos config: %w", err)
	}

	namespaces, err := commonservice.ListNacosNamespace(j.jobSpec.NacosID, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("failed to list nacos namespace")
	}

	namespaceName := ""
	for _, namespace := range namespaces {
		if namespace.NamespaceID == j.jobSpec.NamespaceID {
			namespaceName = namespace.NamespacedName
			break
		}
	}

	nacosConfigsMap := map[string]*types.NacosConfig{}
	for _, config := range nacosConfigs {
		config.NamespaceID = j.jobSpec.NamespaceID
		config.NamespaceName = namespaceName
		config.OriginalContent = config.Content

		nacosConfigsMap[getNacosConfigKey(config.Group, config.DataID)] = config
	}

	configSet := sets.NewString(j.jobSpec.NacosDataRange...)
	nacosDataOptions := make([]*types.NacosConfig, 0)
	for _, data := range nacosConfigsMap {
		if !isAllowedNacosData(data, configSet) {
			continue
		}

		nacosDataOptions = append(nacosDataOptions, data)
	}

	j.jobSpec.NacosDataOptions = nacosDataOptions

	return nil
}

func (j NacosJobController) ClearOptions() {
	j.jobSpec.NacosDataOptions = make([]*types.NacosConfig, 0)
}

func (j NacosJobController) ClearSelection() {
	j.jobSpec.NacosDatas = make([]*types.NacosConfig, 0)
}

func (j NacosJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	info, err := mongodb.NewConfigurationManagementColl().GetNacosByID(context.Background(), j.jobSpec.NacosID)
	if err != nil {
		return nil, fmt.Errorf("failed to get nacos info, error: %s", err)
	}
	client, err := commonservice.GetNacosClient(j.jobSpec.NacosID)
	if err != nil {
		return nil, fmt.Errorf("get nacos client error: %v", err)
	}
	namespaces, err := client.ListNamespaces()
	if err != nil {
		return nil, err
	}
	namespaceName := ""
	for _, namespace := range namespaces {
		if namespace.NamespaceID == j.jobSpec.NamespaceID {
			namespaceName = namespace.NamespacedName
			break
		}
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobNacos),
		Spec: commonmodels.JobTaskNacosSpec{
			NacosID:       j.jobSpec.NacosID,
			NamespaceID:   j.jobSpec.NamespaceID,
			NamespaceName: namespaceName,
			NacosAddr:     info.ServerAddress,
			Type:          info.Type,
			AuthConfig:    info.NacosAuthConfig,
			NacosDatas:    transNacosDatas(j.jobSpec.NacosDatas),
		},
		ErrorPolicy: j.errorPolicy,
	}
	resp = append(resp, jobTask)

	return resp, nil
}

func (j NacosJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j NacosJobController) SetRepoCommitInfo() error {
	return nil
}

func (j NacosJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j NacosJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j NacosJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j NacosJobController) IsServiceTypeJob() bool {
	return false
}

func transNacosDatas(confs []*types.NacosConfig) []*commonmodels.NacosData {
	resp := make([]*commonmodels.NacosData, 0)
	for _, conf := range confs {
		resp = append(resp, &commonmodels.NacosData{
			NacosConfig: *conf,
		})
	}
	return resp
}

func isAllowedNacosData(data *types.NacosConfig, filters sets.String) bool {
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

func getNacosConfigKey(group, id string) string {
	return group + "/" + id
}
