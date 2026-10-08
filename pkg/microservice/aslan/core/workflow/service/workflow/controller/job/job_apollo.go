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
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/apollo"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type ApolloJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ApolloJobSpec
}

func CreateApolloJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ApolloJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return ApolloJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j ApolloJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j ApolloJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j ApolloJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ApolloJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if j.jobSpec.ApolloID != currJobSpec.ApolloID {
		return fmt.Errorf("given apollo job spec does not match current apollo job")
	}

	if isExecution {
		j.jobSpec.DisableConfigRange = currJobSpec.DisableConfigRange
		j.jobSpec.NamespaceListOption = currJobSpec.NamespaceListOption

		envOptionMap := make(map[string]*commonmodels.ApolloNamespace)
		for _, ns := range currJobSpec.NamespaceListOption {
			if ns != nil {
				key := fmt.Sprintf("%s++%s++%s", ns.ClusterID, ns.AppID, ns.Env)
				envOptionMap[key] = ns
			}
		}

		nsMap := make(map[string]*commonmodels.ApolloNamespace)
		createdNamespaceMap := make(map[string]struct{})
		for _, ns := range j.jobSpec.NamespaceList {
			if ns == nil {
				return fmt.Errorf("apollo namespace is not allowed to be nil")
			}
			if ns.Action == "" {
				ns.Action = commonmodels.ApolloActionUpdate
			}

			switch ns.Action {
			case commonmodels.ApolloActionUpdate:
				namespaceKey := fmt.Sprintf("%s++%s++%s++%s", ns.ClusterID, ns.AppID, ns.Env, ns.Namespace)
				if _, ok := nsMap[namespaceKey]; ok {
					return fmt.Errorf("duplicate apollo namespace: %s", ns.Namespace)
				}
				nsMap[namespaceKey] = ns

				if !currJobSpec.DisableConfigRange {
					envKey := fmt.Sprintf("%s++%s++%s", ns.ClusterID, ns.AppID, ns.Env)
					if _, ok := envOptionMap[envKey]; !ok {
						return fmt.Errorf("apollo target [appID=%s, env=%s, cluster=%s] is not allowed to be changed", ns.AppID, ns.Env, ns.ClusterID)
					}
				}

			case commonmodels.ApolloActionCreate:
				ns.AppID = strings.TrimSpace(ns.AppID)
				ns.Namespace = strings.TrimSpace(ns.Namespace)
				ns.Type = strings.ToLower(strings.TrimSpace(ns.Type))
				if ns.Type == "" {
					ns.Type = apollo.FormatYAML
				}
				if ns.AppID == "" || ns.Namespace == "" {
					return fmt.Errorf("apollo appID and namespace are required")
				}
				if ns.Env != "" || ns.ClusterID != "" {
					return fmt.Errorf("env and clusterID must be empty when creating an apollo namespace")
				}
				switch ns.Type {
				case apollo.FormatYAML, apollo.FormatYML, apollo.FormatJSON, apollo.FormatProperties, apollo.FormatXML:
				default:
					return fmt.Errorf("unsupported apollo namespace type: %s", ns.Type)
				}
				if err := apollo.ValidateNamespaceName(ns.Namespace); err != nil {
					return err
				}

				namespaceKey := ns.AppID + "++" + apollo.NormalizeNamespaceName(ns.Namespace, ns.Type)
				if _, ok := createdNamespaceMap[namespaceKey]; ok {
					return fmt.Errorf("duplicate apollo namespace: %s", ns.Namespace)
				}
				createdNamespaceMap[namespaceKey] = struct{}{}

			default:
				return fmt.Errorf("unsupported apollo action: %s", ns.Action)
			}
		}

		if len(j.jobSpec.NamespaceList) == 0 {
			return fmt.Errorf("job namespace list is not allowed to be empty when executing workflow")
		}
	}

	return nil
}

// Update does 2 things:
// 1. ALWAYS use the configured apollo system and options.
// 2. if there is a given selection and the configured system changed, clear it.
func (j ApolloJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ApolloJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}
	j.errorPolicy = currJob.ErrorPolicy
	j.executePolicy = currJob.ExecutePolicy

	if j.jobSpec.ApolloID != currJobSpec.ApolloID {
		// if the configured system change, old selection no longer applies
		j.jobSpec.NamespaceList = make([]*commonmodels.ApolloNamespace, 0)
	}
	j.jobSpec.ApolloID = currJobSpec.ApolloID
	j.jobSpec.DisableConfigRange = currJobSpec.DisableConfigRange

	if j.jobSpec.DisableConfigRange {
		j.jobSpec.NamespaceListOption = make([]*commonmodels.ApolloNamespace, 0)
	} else {
		j.jobSpec.NamespaceListOption = currJobSpec.NamespaceListOption
	}

	return nil
}

// SetOptions sets the actual kv for each configured apollo namespace for users to select and edit
func (j ApolloJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), j.jobSpec.ApolloID)
	if err != nil {
		return fmt.Errorf("failed to get apollo info from mongo: %v", err)
	}

	newNamespaces := []*commonmodels.ApolloNamespace{}
	if j.jobSpec.DisableConfigRange {
		j.jobSpec.NamespaceListOption = newNamespaces
		return nil
	}

	client := apollo.NewClient(info.ServerAddress, info.Token)
	for _, namespace := range j.jobSpec.NamespaceListOption {
		if namespace.Namespace == "*" {
			namespaces, err := client.ListAppNamespace(namespace.AppID, namespace.Env, namespace.ClusterID)
			if err != nil {
				log.Warnf("ApolloJob: list namespace %s-%s-%s error: %v", namespace.AppID, namespace.Env, namespace.ClusterID, err)
				continue
			}
			for _, ns := range namespaces {
				newNamespaces = append(newNamespaces, &commonmodels.ApolloNamespace{
					Action:    commonmodels.ApolloActionUpdate,
					AppID:     namespace.AppID,
					Env:       namespace.Env,
					ClusterID: namespace.ClusterID,
					Namespace: ns.NamespaceName,
					Type:      ns.Format,
				})
			}
		} else {
			if namespace.Action == "" {
				namespace.Action = commonmodels.ApolloActionUpdate
			}
			newNamespaces = append(newNamespaces, namespace)
		}
	}
	j.jobSpec.NamespaceListOption = newNamespaces

	return nil
}

// ClearOptions does nothing since the option field happens to be the user configured field, clear it would cause problems
func (j ApolloJobController) ClearOptions() {
	return
}

func (j ApolloJobController) ClearSelection() {
	j.jobSpec.NamespaceList = make([]*commonmodels.ApolloNamespace, 0)
}

func (j ApolloJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobApollo),
		Spec: &commonmodels.JobTaskApolloSpec{
			ApolloID:            j.jobSpec.ApolloID,
			DisableConfigRange:  j.jobSpec.DisableConfigRange,
			NamespaceListOption: j.jobSpec.NamespaceListOption,
			NamespaceList: func() (list []*commonmodels.JobTaskApolloNamespace) {
				for _, namespace := range j.jobSpec.NamespaceList {
					if namespace.Action == "" {
						namespace.Action = commonmodels.ApolloActionUpdate
					}
					list = append(list, &commonmodels.JobTaskApolloNamespace{
						ApolloNamespace: *namespace,
						Status:          string(config.StatusCreated),
					})
				}
				return list
			}(),
		},
		Timeout:       0,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}

	return []*commonmodels.JobTask{jobTask}, nil
}

func (j ApolloJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j ApolloJobController) SetRepoCommitInfo() error {
	return nil
}

func (j ApolloJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}
	return resp, nil
}

func (j ApolloJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j ApolloJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j ApolloJobController) IsServiceTypeJob() bool {
	return false
}
