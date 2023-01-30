/*
Copyright 2022 The KodeRover Authors.

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
	"fmt"
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	models2 "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	config2 "github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	mongodb2 "github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type GetCollaborationUpdateResp struct {
	UpdateInstance []models.CollaborationInstance `json:"update_instance"`
	Update         []UpdateItem                   `json:"update"`
	New            []models.CollaborationMode     `json:"new"`
	Delete         []models.CollaborationInstance `json:"delete"`
}

type UpdateItem struct {
	CollaborationMode string     `json:"collaboration_mode"`
	PolicyName        string     `json:"policy_name"`
	DeployType        string     `json:"deploy_type"`
	DeleteSpec        DeleteSpec `json:"delete_spec"`
	UpdateSpec        UpdateSpec `json:"update_spec"`
	NewSpec           NewSpec    `json:"new_spec"`
}

type DeleteSpec struct {
	Workflows []models.WorkflowCIItem `json:"workflows"`
	Products  []models.ProductCIItem  `json:"products"`
}

type UpdateSpec struct {
	Workflows []UpdateWorkflowItem `json:"workflows"`
	Products  []UpdateProductItem  `json:"products"`
}

type NewSpec struct {
	Workflows []models.WorkflowCMItem `json:"workflows"`
	Products  []models.ProductCMItem  `json:"products"`
}

type UpdateWorkflowItem struct {
	Old models.WorkflowCIItem `json:"old"`
	New models.WorkflowCMItem `json:"new"`
}

type UpdateProductItem struct {
	Old models.ProductCIItem `json:"old"`
	New models.ProductCMItem `json:"new"`
}

type Workflow struct {
	CollaborationType config.CollaborationType `json:"collaboration_type"`
	BaseName          string                   `json:"base_name"`
	CollaborationMode string                   `json:"collaboration_mode"`
	Name              string                   `json:"name"`
	DisplayName       string                   `json:"display_name"`
	Description       string                   `json:"description"`
	WorkflowType      string                   `json:"workflow_type"`
}

type Product struct {
	CollaborationType config.CollaborationType `json:"collaboration_type"`
	BaseName          string                   `json:"base_name"`
	CollaborationMode string                   `json:"collaboration_mode"`
	Name              string                   `json:"name"`
	DeployType        string                   `json:"deploy_type"`
	//Vars              []*templatemodels.RenderKV        `json:"vars"`
	DefaultValues string                            `json:"default_values,omitempty"`
	ValuesData    *commonservice.ValuesDataArgs     `json:"valuesData,omitempty"`
	YamlData      *templatemodels.CustomYaml        `json:"yaml_data,omitempty"`
	ChartValues   []*commonservice.HelmSvcRenderArg `json:"chartValues,omitempty"`
	Services      []*commonservice.K8sSvcRenderArg  `json:"services"`
}

type GetCollaborationNewResp struct {
	Code     int64       `json:"code"`
	Workflow []*Workflow `json:"workflow"`
	Product  []*Product  `json:"product"`
	IfSync   bool        `json:"ifSync"`
}

type GetCollaborationDeleteResp struct {
	CommonWorkflows []string
	Workflows       []string
	Products        []string
}

func getUpdateWorkflowDiff(cmwMap map[string]models.WorkflowCMItem, ciwMap map[string]models.WorkflowCIItem) (
	[]UpdateWorkflowItem, []models.WorkflowCMItem, []models.WorkflowCIItem) {
	var updateWorkflowItems []UpdateWorkflowItem
	var newWorkflowItems []models.WorkflowCMItem
	var deleteWorkflowItems []models.WorkflowCIItem
	for name, cm := range cmwMap {
		if ci, ok := ciwMap[name]; ok {
			if !reflect.DeepEqual(cm.Verbs, ci.Verbs) || cm.CollaborationType != ci.CollaborationType {
				updateWorkflowItems = append(updateWorkflowItems, UpdateWorkflowItem{
					Old: ci,
					New: cm,
				})
			}
		} else {
			newWorkflowItems = append(newWorkflowItems, cm)
		}
	}
	for name, ci := range ciwMap {
		if _, ok := cmwMap[name]; !ok {
			deleteWorkflowItems = append(deleteWorkflowItems, ci)
		}
	}
	return updateWorkflowItems, newWorkflowItems, deleteWorkflowItems
}

func getUpdateProductDiff(cmpMap map[string]models.ProductCMItem, cipMap map[string]models.ProductCIItem) (
	[]UpdateProductItem, []models.ProductCMItem, []models.ProductCIItem) {
	var updateProductItems []UpdateProductItem
	var newProductItems []models.ProductCMItem
	var deleteProductItems []models.ProductCIItem
	for name, cm := range cmpMap {
		if ci, ok := cipMap[name]; ok {
			if !reflect.DeepEqual(cm.Verbs, ci.Verbs) || cm.CollaborationType != ci.CollaborationType {
				updateProductItems = append(updateProductItems, UpdateProductItem{
					Old: ci,
					New: cm,
				})
			}
		} else {
			newProductItems = append(newProductItems, cm)
		}
	}
	for name, ci := range cipMap {
		if _, ok := cmpMap[name]; !ok {
			deleteProductItems = append(deleteProductItems, ci)
		}
	}
	return updateProductItems, newProductItems, deleteProductItems
}

func getUpdateDiff(cm *models.CollaborationMode, ci *models.CollaborationInstance) UpdateItem {
	cmwMap := make(map[string]models.WorkflowCMItem)
	for _, workflow := range cm.Workflows {
		cmwMap[workflow.Name] = workflow
	}
	ciwMap := make(map[string]models.WorkflowCIItem)
	for _, workflow := range ci.Workflows {
		ciwMap[workflow.BaseName] = workflow
	}
	updateWorkflowItems, newWorkflowItems, deleteWorkflowItems := getUpdateWorkflowDiff(cmwMap, ciwMap)
	cmpMap := make(map[string]models.ProductCMItem)
	for _, product := range cm.Products {
		cmpMap[product.Name] = product
	}
	cipMap := make(map[string]models.ProductCIItem)
	for _, product := range ci.Products {
		cipMap[product.BaseName] = product
	}
	updateProductItems, newProductItems, deleteProductItems := getUpdateProductDiff(cmpMap, cipMap)
	return UpdateItem{
		CollaborationMode: cm.Name,
		PolicyName:        ci.PolicyName,
		DeployType:        cm.DeployType,
		NewSpec: NewSpec{
			Workflows: newWorkflowItems,
			Products:  newProductItems,
		},
		UpdateSpec: UpdateSpec{
			Workflows: updateWorkflowItems,
			Products:  updateProductItems,
		},
		DeleteSpec: DeleteSpec{
			Workflows: deleteWorkflowItems,
			Products:  deleteProductItems,
		},
	}
}

func buildPolicyName(projectName, mode, identityType, userName string) string {
	return projectName + "-" + mode + "-" + identityType + "-" + userName
}

func genCollaborationInstance(mode models.CollaborationMode, projectName, uid, identityType, userName string) *models.CollaborationInstance {
	var workflows []models.WorkflowCIItem
	for _, workflow := range mode.Workflows {
		name := workflow.Name
		if workflow.CollaborationType == config.CollaborationNew {
			name = buildName(workflow.Name, mode.Name, identityType, userName)
		}
		workflows = append(workflows, models.WorkflowCIItem{
			Name:              name,
			BaseName:          workflow.Name,
			Verbs:             workflow.Verbs,
			CollaborationType: workflow.CollaborationType,
			WorkflowType:      workflow.WorkflowType,
			DisplayName:       workflow.DisplayName,
		})
	}
	var products []models.ProductCIItem
	for _, product := range mode.Products {
		name := product.Name
		if product.CollaborationType == config.CollaborationNew {
			name = buildName(product.Name, mode.Name, identityType, userName)
		}
		products = append(products, models.ProductCIItem{
			Name:              name,
			BaseName:          product.Name,
			CollaborationType: product.CollaborationType,
			Verbs:             product.Verbs,
		})
	}
	return &models.CollaborationInstance{
		ProjectName:       mode.ProjectName,
		CollaborationName: mode.Name,
		UserUID:           uid,
		PolicyName:        buildPolicyName(projectName, mode.Name, identityType, userName),
		Revision:          mode.Revision,
		RecycleDay:        mode.RecycleDay,
		Workflows:         workflows,
		Products:          products,
		LastVisitTime:     time.Now().Unix(),
	}
}

func getDiff(cmMap map[string]*models.CollaborationMode, ciMap map[string]*models.CollaborationInstance, projectName,
	uid, identityType, userName string) (*GetCollaborationUpdateResp, error) {
	var updateItems []UpdateItem
	var updateInstance []models.CollaborationInstance
	var newItems []models.CollaborationMode
	var deleteItems []models.CollaborationInstance
	for name, cm := range cmMap {
		if ci, ok := ciMap[name]; ok {
			if cm.Revision < ci.Revision {
				return nil, fmt.Errorf("CollaborationMode:%s revision error", name)
			} else if cm.Revision > ci.Revision {
				updateItems = append(updateItems, getUpdateDiff(cm, ci))
				instance := genCollaborationInstance(*cm, projectName, uid, identityType, userName)
				updateInstance = append(updateInstance, *instance)
			}
		} else {
			newItems = append(newItems, *cm)
		}
	}
	for name, ci := range ciMap {
		if _, ok := cmMap[name]; !ok {
			deleteItems = append(deleteItems, *ci)
		}
	}
	return &GetCollaborationUpdateResp{
		UpdateInstance: updateInstance,
		Update:         updateItems,
		New:            newItems,
		Delete:         deleteItems,
	}, nil
}

func updateVisitTime(uid string, cis []*models.CollaborationInstance, logger *zap.SugaredLogger) error {
	for _, instance := range cis {
		instance.LastVisitTime = time.Now().Unix()
		err := mongodb.NewCollaborationInstanceColl().Update(uid, instance)
		if err != nil {
			logger.Errorf("syncInstance Update error, error msg:%s", err)
			return err
		}
	}
	return nil
}

func GetCollaborationUpdate(projectName, uid, identityType, userName string, logger *zap.SugaredLogger) (*GetCollaborationUpdateResp, error) {
	collaborations, err := mongodb.NewCollaborationModeColl().List(&mongodb.CollaborationModeListOptions{
		Projects: []string{projectName},
		Members:  []string{uid},
	})
	if err != nil {
		logger.Errorf("GetCollaborationUpdate error, err msg:%s", err)
		return nil, err
	}
	cmMap := make(map[string]*models.CollaborationMode)
	for _, collaboration := range collaborations {
		cmMap[collaboration.Name] = collaboration
	}
	collaborationInstances, err := mongodb.NewCollaborationInstanceColl().List(&mongodb.CollaborationInstanceFindOptions{
		ProjectName: projectName,
		UserUID:     []string{uid},
	})
	if err != nil {
		logger.Errorf("GetCollaborationInstance error, err msg:%s", err)
		return nil, err
	}
	ciMap := make(map[string]*models.CollaborationInstance)
	for _, instance := range collaborationInstances {
		ciMap[instance.CollaborationName] = instance
	}
	resp, err := getDiff(cmMap, ciMap, projectName, uid, identityType, userName)
	if err != nil {
		logger.Errorf("GetCollaborationUpdate error, err msg:%s", err)
		return nil, err
	}
	err = updateVisitTime(uid, collaborationInstances, logger)
	if err != nil {
		logger.Errorf("GetCollaborationUpdate updateVisitTime error, err msg:%s", err)
		return nil, err
	}
	return resp, nil
}

func buildName(baseName, modeName, identityType, userName string) string {
	return modeName + "-" + baseName + "-" + identityType + "-" + userName
}

func syncInstance(updateResp *GetCollaborationUpdateResp, projectName, identityType, userName, uid string,
	logger *zap.SugaredLogger) error {
	var instances []*models.CollaborationInstance
	modeInstanceMap := make(map[string]*models.CollaborationInstance)
	for _, mode := range updateResp.New {
		instance := genCollaborationInstance(mode, projectName, uid, identityType, userName)
		instances = append(instances, instance)
		modeInstanceMap[mode.Name] = instance
	}
	if len(instances) > 0 {
		err := mongodb.NewCollaborationInstanceColl().BulkCreate(instances)
		if err != nil {
			logger.Errorf("syncInstance BulkCreate error, error msg:%s", err)
			return err
		}
	}
	for _, instance := range updateResp.UpdateInstance {
		err := mongodb.NewCollaborationInstanceColl().Update(uid, &instance)
		if err != nil {
			logger.Errorf("syncInstance Update error, error msg:%s", err)
			return err
		}
	}
	var findOpts []mongodb.CollaborationInstanceFindOptions
	for _, instance := range updateResp.Delete {
		findOpts = append(findOpts, mongodb.CollaborationInstanceFindOptions{
			Name:        instance.CollaborationName,
			ProjectName: instance.ProjectName,
			UserUID:     []string{instance.UserUID},
		})
	}
	return mongodb.NewCollaborationInstanceColl().BulkDelete(mongodb.CollaborationInstanceListOptions{
		FindOpts: findOpts,
	})
}

func buildPolicyDescription(mode, userName string) string {
	return mode + " " + userName + " 的权限"
}

func buildPolicybindingName(uid, policyName, projectName string) string {
	return uid + "-" + policyName + "-" + projectName
}

func syncPolicy(updateResp *GetCollaborationUpdateResp, projectName, identityType, userName, uid string,
	logger *zap.SugaredLogger) error {
	var policies []*types.Policy
	var policyBindings []*policy.PolicyBinding
	for _, mode := range updateResp.New {
		var rules []*types.Rule

		policyName := buildPolicyName(projectName, mode.Name, identityType, userName)

		for _, workflow := range mode.Workflows {
			rules = append(rules, &types.Rule{
				Verbs:     workflow.Verbs,
				Kind:      "resource",
				Resources: []string{string(config2.ResourceTypeWorkflow)},
				MatchAttributes: []types.MatchAttribute{
					{
						Key:   "policy",
						Value: buildLabelValue(projectName, mode.Name, identityType, userName, config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.Name),
					},
				},
			})
		}
		for _, product := range mode.Products {
			rules = append(rules, &types.Rule{
				Verbs:     product.Verbs,
				Kind:      "resource",
				Resources: []string{string(config2.ResourceTypeEnvironment)},
				MatchAttributes: []types.MatchAttribute{
					{
						Key:   "policy",
						Value: buildLabelValue(projectName, mode.Name, identityType, userName, string(config2.ResourceTypeEnvironment), product.Name),
					},
				},
			})
		}
		policies = append(policies, &types.Policy{
			Name:        policyName,
			UpdateTime:  time.Now().Unix(),
			Description: buildPolicyDescription(mode.Name, userName),
			Rules:       rules,
		})
		policyBindings = append(policyBindings, &policy.PolicyBinding{
			Name:   buildPolicybindingName(uid, policyName, projectName),
			UID:    uid,
			Policy: policyName,
			Preset: false,
			Type:   setting.ResourceTypeSystem,
		})
	}
	if len(policies) > 0 {
		err := policy.NewDefault().CreatePolicies(projectName, policy.CreatePoliciesArgs{
			Policies: policies,
		})
		if err != nil {
			logger.Errorf("syncPolicy error, error msg:%s", err)
			return err
		}
	}
	if len(policyBindings) > 0 {
		err := policy.NewDefault().CreatePolicyBinding(projectName, policyBindings)
		if err != nil {
			logger.Errorf("syncPolicyBindings error, error msg:%s", err)
			return err
		}
	}
	var updatePolicies []*types.Policy
	for _, instance := range updateResp.UpdateInstance {
		var rules []*types.Rule
		for _, workflow := range instance.Workflows {
			rules = append(rules, &types.Rule{
				Verbs:     workflow.Verbs,
				Kind:      "resource",
				Resources: []string{string(config2.ResourceTypeWorkflow)},
				MatchAttributes: []types.MatchAttribute{
					{
						Key:   "policy",
						Value: buildLabelValue(projectName, instance.CollaborationName, identityType, userName, config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.BaseName),
					},
				},
			})
		}
		for _, product := range instance.Products {
			rules = append(rules, &types.Rule{
				Verbs:     product.Verbs,
				Kind:      "resource",
				Resources: []string{string(config2.ResourceTypeEnvironment)},
				MatchAttributes: []types.MatchAttribute{
					{
						Key:   "policy",
						Value: buildLabelValue(projectName, instance.CollaborationName, identityType, userName, string(config2.ResourceTypeEnvironment), product.BaseName),
					},
				},
			})
		}
		updatePolicies = append(updatePolicies, &types.Policy{
			Name:        instance.PolicyName,
			Description: buildPolicyDescription(instance.CollaborationName, userName),
			UpdateTime:  time.Now().Unix(),
			Rules:       rules,
		})
	}
	for _, updatePolicy := range updatePolicies {
		err := policy.NewDefault().UpdatePolicy(projectName, updatePolicy)
		if err != nil {
			return err
		}
	}
	var deletePolicies []string
	var deletePolicybindings []string
	for _, instance := range updateResp.Delete {
		deletePolicies = append(deletePolicies, instance.PolicyName)
		deletePolicybindings = append(deletePolicybindings, buildPolicybindingName(uid, instance.PolicyName, projectName))
	}
	if len(deletePolicies) > 0 {
		err := policy.NewDefault().DeletePolicies(projectName, policy.DeletePoliciesArgs{
			Names: deletePolicies,
		})
		if err != nil {
			return err
		}
	}
	if len(deletePolicybindings) > 0 {
		err := policy.NewDefault().DeletePolicyBindings(deletePolicybindings, projectName)
		if err != nil {
			return err
		}
	}
	return nil
}

func buildLabelValue(projectName, mode, identityType, userName, resourceType, resourceName string) string {
	return projectName + "-" + mode + "-" + identityType + "-" + userName + "-" + resourceType + "-" + resourceName
}

func syncLabel(updateResp *GetCollaborationUpdateResp, projectName, identityType, userName string, logger *zap.SugaredLogger) error {
	var labels []mongodb2.Label
	var deleteLabels []mongodb2.Label
	var newBindings []*mongodb2.LabelBinding
	var deleteBindings []*mongodb2.LabelBinding
	for _, mode := range updateResp.New {
		for _, workflow := range mode.Workflows {
			labels = append(labels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, mode.Name, identityType, userName,
					config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.Name),
				Type:        setting.ResourceTypeSystem,
				ProjectName: projectName,
			})
		}
		for _, product := range mode.Products {
			labels = append(labels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, mode.Name, identityType, userName,
					string(config2.ResourceTypeEnvironment), product.Name),
				Type:        setting.ResourceTypeSystem,
				ProjectName: projectName,
			})
		}
	}
	for _, item := range updateResp.Update {
		for _, workflow := range item.NewSpec.Workflows {
			labels = append(labels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, item.CollaborationMode, identityType, userName,
					config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.Name),
				Type:        setting.ResourceTypeSystem,
				ProjectName: projectName,
			})
		}
		for _, product := range item.NewSpec.Products {
			labels = append(labels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, item.CollaborationMode, identityType, userName,
					string(config2.ResourceTypeEnvironment), product.Name),
				Type:        setting.ResourceTypeSystem,
				ProjectName: projectName,
			})
		}
		for _, workflow := range item.DeleteSpec.Workflows {
			deleteLabels = append(deleteLabels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, item.CollaborationMode, identityType, userName,
					config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.BaseName),
				Type:        setting.ResourceTypeSystem,
				ProjectName: projectName,
			})
		}
		for _, product := range item.DeleteSpec.Products {
			deleteLabels = append(deleteLabels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, item.CollaborationMode, identityType, userName,
					string(config2.ResourceTypeEnvironment), product.BaseName),
				Type:        setting.ResourceTypeSystem,
				ProjectName: projectName,
			})
		}

	}
	for _, instance := range updateResp.Delete {
		for _, workflow := range instance.Workflows {
			deleteLabels = append(deleteLabels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, instance.CollaborationName, identityType, userName,
					config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.BaseName),
				Type: setting.ResourceTypeSystem,
			})
		}
		for _, product := range instance.Products {
			deleteLabels = append(deleteLabels, mongodb2.Label{
				Key: "policy",
				Value: buildLabelValue(projectName, instance.CollaborationName, identityType, userName,
					string(config2.ResourceTypeEnvironment), product.BaseName),
				Type: setting.ResourceTypeSystem,
			})
		}
	}
	var err error
	if len(labels) > 0 {
		_, err := service.CreateLabels(&service.CreateLabelsArgs{
			Labels: labels,
		}, userName)

		if err != nil {
			logger.Errorf("create labels error, error msg:%s", err)
			return err
		}
	}

	if len(deleteLabels) > 0 {
		resp, err := service.ListLabels(&service.ListLabelsArgs{
			Labels: deleteLabels,
		})
		if err != nil {
			return err
		}
		if len(resp.Labels) != len(deleteLabels) {
			return fmt.Errorf("deletelabels not exist,%v:%v", resp.Labels, deleteLabels)
		}
		var ids []string
		for _, label := range resp.Labels {
			ids = append(ids, label.ID.Hex())
		}
		err = service.DeleteLabels(ids, true, userName, logger)
		if err != nil {
			logger.Errorf("delete labels error, error msg:%s", err)
			return err
		}
	}

	for _, item := range updateResp.Update {
		for _, workflow := range item.UpdateSpec.Workflows {
			labels = append(labels, mongodb2.Label{
				Key:   "policy",
				Value: buildLabelValue(projectName, item.CollaborationMode, identityType, userName, config2.GetWorkflowResourceType(workflow.New.WorkflowType), workflow.New.Name),
				Type:  setting.ResourceTypeSystem,
			})
		}
		for _, product := range item.UpdateSpec.Products {
			labels = append(labels, mongodb2.Label{
				Key:   "policy",
				Value: buildLabelValue(projectName, item.CollaborationMode, identityType, userName, string(config2.ResourceTypeEnvironment), product.New.Name),
				Type:  setting.ResourceTypeSystem,
			})
		}

	}
	labelIdMap := make(map[string]string)
	if len(labels) > 0 {
		labelListResp, err := service.ListLabels(&service.ListLabelsArgs{
			Labels: labels,
		})
		if err != nil {
			return err
		}
		if labelListResp == nil {
			return fmt.Errorf("label not exist %v", labels)
		}
		if len(labelListResp.Labels) != len(labels) {
			return fmt.Errorf("label not exist %v:%v", labels, labelListResp.Labels)
		}

		for _, label := range labelListResp.Labels {
			labelIdMap[service.BuildLabelString(label.Key, label.Value)] = label.ID.Hex()
		}

	}

	for _, mode := range updateResp.New {
		for _, workflow := range mode.Workflows {
			labelValue := buildLabelValue(projectName, mode.Name, identityType, userName, config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.Name)
			labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
			if !ok {
				return fmt.Errorf("label:%s not exist", labelValue)
			}
			name := workflow.Name
			if workflow.CollaborationType == config.CollaborationNew {
				name = buildName(workflow.Name, mode.Name, identityType, userName)
			}

			newBindings = append(newBindings, &mongodb2.LabelBinding{
				LabelID: labelId,
				Resource: mongodb2.Resource{
					Name:        name,
					ProjectName: projectName,
					Type:        config2.GetWorkflowResourceType(workflow.WorkflowType),
				},
			})

		}
		for _, product := range mode.Products {
			labelValue := buildLabelValue(projectName, mode.Name, identityType, userName, string(config2.ResourceTypeEnvironment), product.Name)
			labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
			if !ok {
				return fmt.Errorf("label:%s not exist", labelValue)
			}
			name := product.Name
			if product.CollaborationType == config.CollaborationNew {
				name = buildName(product.Name, mode.Name, identityType, userName)
			}
			newBindings = append(newBindings, &mongodb2.LabelBinding{
				LabelID: labelId,
				Resource: mongodb2.Resource{
					Name:        name,
					ProjectName: projectName,
					Type:        string(config2.ResourceTypeEnvironment),
				},
			})
		}
	}
	for _, item := range updateResp.Update {
		for _, workflow := range item.NewSpec.Workflows {
			labelValue := buildLabelValue(projectName, item.CollaborationMode, identityType, userName, config2.GetWorkflowResourceType(workflow.WorkflowType), workflow.Name)
			labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
			if !ok {
				return fmt.Errorf("label:%s not exist", labelValue)
			}
			name := workflow.Name
			if workflow.CollaborationType == config.CollaborationNew {
				name = buildName(workflow.Name, item.CollaborationMode, identityType, userName)
			}
			newBindings = append(newBindings, &mongodb2.LabelBinding{
				LabelID: labelId,
				Resource: mongodb2.Resource{
					Name:        name,
					Type:        config2.GetWorkflowResourceType(workflow.WorkflowType),
					ProjectName: projectName,
				},
			})
		}
		for _, product := range item.NewSpec.Products {
			labelValue := buildLabelValue(projectName, item.CollaborationMode, identityType, userName, string(config2.ResourceTypeEnvironment), product.Name)
			labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
			if !ok {
				return fmt.Errorf("label:%s not exist", labelValue)
			}
			name := product.Name
			if product.CollaborationType == config.CollaborationNew {
				name = buildName(product.Name, item.CollaborationMode, identityType, userName)
			}
			newBindings = append(newBindings, &mongodb2.LabelBinding{
				LabelID: labelId,
				Resource: mongodb2.Resource{
					Name:        name,
					Type:        string(config2.ResourceTypeEnvironment),
					ProjectName: projectName,
				},
			})
		}
		for _, workflow := range item.UpdateSpec.Workflows {
			if workflow.Old.CollaborationType == config.CollaborationShare && workflow.New.CollaborationType == config.CollaborationNew {
				labelValue := buildLabelValue(projectName, item.CollaborationMode, identityType, userName, config2.GetWorkflowResourceType(workflow.New.WorkflowType), workflow.New.Name)
				labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
				if !ok {
					return fmt.Errorf("label:%s not exist", labelValue)
				}
				newBindings = append(newBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        buildName(workflow.New.Name, item.CollaborationMode, identityType, userName),
						Type:        config2.GetWorkflowResourceType(workflow.New.WorkflowType),
						ProjectName: projectName,
					},
				})

				deleteBindings = append(deleteBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        workflow.Old.BaseName,
						Type:        config2.GetWorkflowResourceType(workflow.Old.WorkflowType),
						ProjectName: projectName,
					},
				})
			}
			if workflow.Old.CollaborationType == config.CollaborationNew && workflow.New.CollaborationType == config.CollaborationShare {
				labelValue := buildLabelValue(projectName, item.CollaborationMode, identityType, userName, config2.GetWorkflowResourceType(workflow.New.WorkflowType), workflow.New.Name)
				labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
				if !ok {
					return fmt.Errorf("label:%s not exist", labelValue)
				}
				newBindings = append(newBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        workflow.New.Name,
						Type:        config2.GetWorkflowResourceType(workflow.New.WorkflowType),
						ProjectName: projectName,
					},
				})
				deleteBindings = append(deleteBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        workflow.Old.Name,
						Type:        config2.GetWorkflowResourceType(workflow.Old.WorkflowType),
						ProjectName: projectName,
					},
				})
			}
		}
		for _, product := range item.UpdateSpec.Products {
			if product.Old.CollaborationType == config.CollaborationShare && product.New.CollaborationType == config.CollaborationNew {
				labelValue := buildLabelValue(projectName, item.CollaborationMode, identityType, userName, string(config2.ResourceTypeEnvironment), product.New.Name)
				labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
				if !ok {
					return fmt.Errorf("label:%s not exist", labelValue)
				}
				newBindings = append(newBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        buildName(product.New.Name, item.CollaborationMode, identityType, userName),
						Type:        string(config2.ResourceTypeEnvironment),
						ProjectName: projectName,
					},
				})
				deleteBindings = append(deleteBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        product.Old.BaseName,
						Type:        string(config2.ResourceTypeEnvironment),
						ProjectName: projectName,
					},
				})
			}
			if product.Old.CollaborationType == config.CollaborationNew && product.New.CollaborationType == config.CollaborationShare {
				labelValue := buildLabelValue(projectName, item.CollaborationMode, identityType, userName, string(config2.ResourceTypeEnvironment), product.New.Name)
				labelId, ok := labelIdMap[service.BuildLabelString("policy", labelValue)]
				if !ok {
					return fmt.Errorf("label:%s not exist", labelValue)
				}
				newBindings = append(newBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        product.New.Name,
						Type:        string(config2.ResourceTypeEnvironment),
						ProjectName: projectName,
					},
				})
				deleteBindings = append(deleteBindings, &mongodb2.LabelBinding{
					LabelID: labelId,
					Resource: mongodb2.Resource{
						Name:        product.Old.Name,
						Type:        string(config2.ResourceTypeEnvironment),
						ProjectName: projectName,
					},
				})
			}
		}
	}

	if len(newBindings) > 0 {
		err = service.CreateLabelBindings(&service.CreateLabelBindingsArgs{
			LabelBindings: newBindings,
		}, userName, logger)
		if err != nil {
			logger.Errorf("create labelbindings error:%s", err)
			return err
		}
	}
	if len(deleteBindings) > 0 {
		logger.Infof("start syncLabel DeleteLabelBindings:%v user:%s", deleteBindings, userName)
		err = service.DeleteLabelBindings(&service.DeleteLabelBindingsArgs{
			LabelBindings: deleteBindings,
		}, userName, logger)
		if err != nil {
			logger.Errorf("delete labelbindings error:%s", err)
			return err
		}
	}
	return nil
}

func syncResource(products *SyncCollaborationInstanceArgs, updateResp *GetCollaborationUpdateResp, projectName, identityType, userName, requestID string,
	logger *zap.SugaredLogger) error {
	err := syncNewResource(products, updateResp, projectName, identityType, userName, requestID, logger)
	if err != nil {
		return err
	}
	err = syncDeleteResource(updateResp, userName, projectName, requestID, logger)
	if err != nil {
		return err
	}
	return nil
}

func syncDeleteResource(updateResp *GetCollaborationUpdateResp, username, projectName, requestID string,
	log *zap.SugaredLogger) (err error) {
	deleteResp := getCollaborationDelete(updateResp)
	for _, product := range deleteResp.Products {
		err := service2.DeleteProduct(username, product, projectName, requestID, true, log)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Errorf("delete product err:%v", err)
			return err
		}
	}
	for _, workflow := range deleteResp.Workflows {
		err := commonservice.DeleteWorkflow(workflow, requestID, false, log)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Errorf("delete workflow err:%v", err)
			return err
		}
	}
	for _, workflow := range deleteResp.CommonWorkflows {
		err := commonservice.DeleteWorkflowV4(workflow, log)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Errorf("delete workflow err:%v", err)
			return err
		}
	}
	return nil
}

func syncNewResource(products *SyncCollaborationInstanceArgs, updateResp *GetCollaborationUpdateResp, projectName, identityType, userName, requestID string,
	logger *zap.SugaredLogger) error {
	newResp, err := getCollaborationNew(updateResp, projectName, identityType, userName, logger)
	if err != nil {
		return err
	}
	if newResp.Workflow == nil && newResp.Product == nil {
		return nil
	}
	var newWorkflows []workflowservice.WorkflowCopyItem
	var newCommonWorkflows []workflowservice.WorkflowCopyItem
	for _, workflow := range newResp.Workflow {
		if workflow.CollaborationType == config.CollaborationNew {
			if config2.IsCustomWorkflow(workflow.WorkflowType) {
				newCommonWorkflows = append(newCommonWorkflows, workflowservice.WorkflowCopyItem{
					ProjectName:    projectName,
					Old:            workflow.BaseName,
					New:            workflow.Name,
					NewDisplayName: workflow.DisplayName,
					BaseName:       workflow.BaseName,
				})
			} else {
				newWorkflows = append(newWorkflows, workflowservice.WorkflowCopyItem{
					ProjectName:    projectName,
					Old:            workflow.BaseName,
					New:            workflow.Name,
					NewDisplayName: workflow.DisplayName,
					BaseName:       workflow.BaseName,
				})
			}

		}
	}
	if len(newWorkflows) > 0 {
		err = workflowservice.BulkCopyWorkflow(workflowservice.BulkCopyWorkflowArgs{
			Items: newWorkflows,
		}, userName, logger)
		if err != nil {
			return err
		}
	}

	if len(newCommonWorkflows) > 0 {
		logger.Infof("start bulkcopyworkflowv4:%s", newWorkflows)
		err = workflowservice.BulkCopyWorkflowV4(workflowservice.BulkCopyWorkflowArgs{
			Items: newCommonWorkflows,
		}, userName, logger)
		if err != nil {
			return err
		}
	}

	productMap := make(map[string]Product)
	for _, product := range products.Products {
		productMap[product.BaseName] = product
	}
	var yamlProductItems []service2.YamlProductItem
	var helmProductArgs []service2.HelmProductItem
	for _, product := range newResp.Product {
		if productArg, ok := productMap[product.BaseName]; ok {
			if productArg.DeployType == setting.HelmDeployType {
				helmProductArgs = append(helmProductArgs, service2.HelmProductItem{
					OldName:       product.BaseName,
					NewName:       product.Name,
					BaseName:      product.BaseName,
					DefaultValues: productArg.DefaultValues,
					ChartValues:   productArg.ChartValues,
					ValuesData:    productArg.ValuesData,
				})
			}
			if productArg.DeployType == setting.K8SDeployType {
				yamlProductItems = append(yamlProductItems, service2.YamlProductItem{
					OldName:       product.BaseName,
					NewName:       product.Name,
					BaseName:      product.BaseName,
					DefaultValues: productArg.DefaultValues,
					Services:      productArg.Services,
					//Vars:          productArg.Vars,
				})
			}
		}
	}
	if len(yamlProductItems) > 0 {
		err = service2.BulkCopyYamlProduct(projectName, userName, requestID, service2.CopyYamlProductArg{
			Items: yamlProductItems,
		}, logger)
		if err != nil {
			return err
		}
	}
	if len(helmProductArgs) > 0 {
		err = service2.BulkCopyHelmProduct(projectName, userName, requestID, service2.CopyHelmProductArg{
			Items: helmProductArgs,
		}, logger)
		if err != nil {
			return err
		}
	}
	return nil
}

type SyncCollaborationInstanceArgs struct {
	Products []Product `json:"products"`
}

func SyncCollaborationInstance(products *SyncCollaborationInstanceArgs, projectName, uid, identityType, userName, requestID string, logger *zap.SugaredLogger) error {
	updateResp, err := GetCollaborationUpdate(projectName, uid, identityType, userName, logger)
	if err != nil {
		logger.Errorf("GetCollaborationNew error, err msg:%s", err)
		return err
	}
	if updateResp.Update == nil && updateResp.UpdateInstance == nil && updateResp.New == nil && updateResp.Delete == nil {
		return nil
	}
	err = syncInstance(updateResp, projectName, identityType, userName, uid, logger)
	if err != nil {
		logger.Errorf("syncInstance error, err msg:%s", err)
		return err
	}
	err = syncResource(products, updateResp, projectName, identityType, userName, requestID, logger)
	if err != nil {
		logger.Errorf("syncResource error, err msg:%s", err)
		return err
	}
	err = syncPolicy(updateResp, projectName, identityType, userName, uid, logger)
	if err != nil {
		logger.Errorf("syncPolicy error, err msg:%s", err)
		return err
	}
	err = syncLabel(updateResp, projectName, identityType, userName, logger)
	if err != nil {
		logger.Errorf("syncLabel error, err msg:%s", err)
		return err
	}
	return nil
}

func getCollaborationDelete(updateResp *GetCollaborationUpdateResp) *GetCollaborationDeleteResp {
	productSet := sets.String{}
	workflowSet := sets.String{}
	commonWorkflowSet := sets.String{}
	for _, item := range updateResp.Delete {
		for _, product := range item.Products {
			if product.CollaborationType == config.CollaborationNew {
				productSet.Insert(product.Name)
			}
		}
		for _, workflow := range item.Workflows {
			if workflow.CollaborationType == config.CollaborationNew {
				if config2.IsCustomWorkflow(workflow.WorkflowType) {
					commonWorkflowSet.Insert(workflow.Name)
				} else {
					workflowSet.Insert(workflow.Name)
				}
			}
		}
	}
	for _, item := range updateResp.Update {
		for _, deleteWorkflow := range item.DeleteSpec.Workflows {
			if deleteWorkflow.CollaborationType == config.CollaborationNew {
				if config2.IsCustomWorkflow(deleteWorkflow.WorkflowType) {
					commonWorkflowSet.Insert(deleteWorkflow.Name)
				} else {
					workflowSet.Insert(deleteWorkflow.Name)
				}
			}
		}
		for _, deleteProduct := range item.DeleteSpec.Products {
			if deleteProduct.CollaborationType == config.CollaborationNew {
				productSet.Insert(deleteProduct.Name)
			}
		}
		for _, workflow := range item.UpdateSpec.Workflows {
			if workflow.Old.CollaborationType == config.CollaborationNew &&
				workflow.New.CollaborationType == config.CollaborationShare {
				if config2.IsCustomWorkflow(workflow.Old.WorkflowType) {
					commonWorkflowSet.Insert(workflow.Old.Name)
				} else {
					workflowSet.Insert(workflow.Old.Name)
				}

			}
		}
		for _, product := range item.UpdateSpec.Products {
			if product.Old.CollaborationType == config.CollaborationNew &&
				product.New.CollaborationType == config.CollaborationShare {
				productSet.Insert(product.Old.Name)
			}
		}
	}
	return &GetCollaborationDeleteResp{
		CommonWorkflows: commonWorkflowSet.List(),
		Workflows:       workflowSet.List(),
		Products:        productSet.List(),
	}
}

func getCollaborationNew(updateResp *GetCollaborationUpdateResp, projectName, identityType, userName string,
	logger *zap.SugaredLogger) (*GetCollaborationNewResp, error) {
	var newWorkflow []*Workflow
	var newProduct []*Product
	newProductName := sets.String{}
	for _, mode := range updateResp.New {
		for _, workflow := range mode.Workflows {
			name := workflow.Name
			displayName := getWorkflowDisplayName(workflow.Name, workflow.WorkflowType)
			if workflow.CollaborationType == config.CollaborationNew {
				name = buildName(workflow.Name, mode.Name, identityType, userName)
				displayName = buildName(displayName, mode.Name, identityType, userName)
			}
			newWorkflow = append(newWorkflow, &Workflow{
				CollaborationType: workflow.CollaborationType,
				BaseName:          workflow.Name,
				CollaborationMode: mode.Name,
				Name:              name,
				WorkflowType:      workflow.WorkflowType,
				DisplayName:       displayName,
			})
		}
		for _, product := range mode.Products {
			name := product.Name
			if product.CollaborationType == config.CollaborationNew {
				name = buildName(product.Name, mode.Name, identityType, userName)
			}
			newProduct = append(newProduct, &Product{
				CollaborationType: product.CollaborationType,
				BaseName:          product.Name,
				CollaborationMode: mode.Name,
				Name:              name,
				DeployType:        mode.DeployType,
			})
			newProductName.Insert(product.Name)
		}
	}
	for _, item := range updateResp.Update {
		for _, workflow := range item.NewSpec.Workflows {
			name := workflow.Name
			displayName := getWorkflowDisplayName(workflow.Name, workflow.WorkflowType)
			if workflow.CollaborationType == config.CollaborationNew {
				name = buildName(workflow.Name, item.CollaborationMode, identityType, userName)
				displayName = buildName(displayName, item.CollaborationMode, identityType, userName)
			}
			newWorkflow = append(newWorkflow, &Workflow{
				WorkflowType:      workflow.WorkflowType,
				CollaborationType: workflow.CollaborationType,
				BaseName:          workflow.Name,
				CollaborationMode: item.CollaborationMode,
				Name:              name,
				DisplayName:       displayName,
			})
		}
		for _, product := range item.NewSpec.Products {
			name := product.Name
			if product.CollaborationType == config.CollaborationNew {
				name = buildName(product.Name, item.CollaborationMode, identityType, userName)
			}
			newProduct = append(newProduct, &Product{
				CollaborationType: product.CollaborationType,
				BaseName:          product.Name,
				CollaborationMode: item.CollaborationMode,
				Name:              name,
				DeployType:        item.DeployType,
			})
			newProductName.Insert(product.Name)
		}
		for _, workflow := range item.UpdateSpec.Workflows {
			if workflow.Old.CollaborationType == config.CollaborationShare && workflow.New.CollaborationType == config.CollaborationNew {
				displayName := getWorkflowDisplayName(workflow.Old.BaseName, workflow.Old.WorkflowType)
				newWorkflow = append(newWorkflow, &Workflow{
					WorkflowType:      workflow.Old.WorkflowType,
					CollaborationType: workflow.New.CollaborationType,
					BaseName:          workflow.Old.BaseName,
					CollaborationMode: item.CollaborationMode,
					Name:              buildName(workflow.Old.BaseName, item.CollaborationMode, identityType, userName),
					DisplayName:       buildName(displayName, item.CollaborationMode, identityType, userName),
				})
			}
		}
		for _, product := range item.UpdateSpec.Products {
			if product.Old.CollaborationType == config.CollaborationShare && product.New.CollaborationType == config.CollaborationNew {
				newProduct = append(newProduct, &Product{
					CollaborationType: product.New.CollaborationType,
					BaseName:          product.Old.BaseName,
					CollaborationMode: item.CollaborationMode,
					DeployType:        item.DeployType,
					Name:              buildName(product.Old.BaseName, item.CollaborationMode, identityType, userName),
				})
				newProductName.Insert(product.Old.BaseName)
			}
		}
	}
	if len(newProduct) > 0 && newProduct[0].DeployType == setting.K8SDeployType {
		for _, product := range newProduct {
			services, rendersetData, err := commonservice.GetK8sSvcRenderArgs(projectName, product.BaseName, "", logger)
			if err != nil {
				return nil, fmt.Errorf("failed to find product renderset :%s, err: %s", product.BaseName, err)
			}
			if rendersetData == nil {
				logger.Errorf("product renderset:%s not exist", product.BaseName)
				return nil, fmt.Errorf("product renderset :%s not exist", product.BaseName)
			}

			product.Services = services
			product.DefaultValues = rendersetData.DefaultValues
		}
	}
	if len(newProduct) > 0 && newProduct[0].DeployType == setting.HelmDeployType {
		for _, product := range newProduct {
			//chart, ok := envChartsMap[product.BaseName]

			renderChartArgs, rendersetData, err := commonservice.GetSvcRenderArgs(projectName, product.BaseName, "", logger)
			if err != nil {
				return nil, fmt.Errorf("failed to find product renderset :%s, err: %s", product.BaseName, err)
			}
			if rendersetData == nil {
				logger.Errorf("product renderset:%s not exist", product.BaseName)
				return nil, fmt.Errorf("product renderset :%s not exist", product.BaseName)
			}

			product.ChartValues = renderChartArgs
			product.DefaultValues = rendersetData.DefaultValues
			product.YamlData = rendersetData.YamlData
		}
	}
	var workNames []string
	for _, workflow := range newWorkflow {
		workNames = append(workNames, workflow.Name)
	}
	workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{
		Projects: []string{projectName},
		Names:    workNames,
	})
	if err != nil {
		logger.Errorf("GetCollaborationNew list workflows:%v error:%s", workNames, err)
		return nil, err
	}
	workflowDescMap := make(map[string]string)
	for _, workflow := range workflows {
		workflowDescMap[workflow.Name] = workflow.Description
	}
	for _, workflow := range newWorkflow {
		if desc, ok := workflowDescMap[workflow.Name]; ok {
			workflow.Description = desc
		}
	}
	ifSync := false
	if len(newWorkflow) == 0 && len(newProductName) == 0 && (len(updateResp.Update) != 0 || len(updateResp.Delete) != 0) {
		ifSync = true
	}
	return &GetCollaborationNewResp{
		//Code 10000 means it is a filtered success result
		Code:     10000,
		Workflow: newWorkflow,
		Product:  newProduct,
		IfSync:   ifSync,
	}, nil
}

type DeleteCIResourcesRequest struct {
	CollaborationInstances []models.CollaborationInstance `json:"collaboration_instances"`
}

func CleanCIResources(userName, requestID string, logger *zap.SugaredLogger) error {
	cis, err := mongodb.NewCollaborationInstanceColl().List(&mongodb.CollaborationInstanceFindOptions{})
	if err != nil {
		return err
	}
	var fileterdInstances []*models.CollaborationInstance
	for _, ci := range cis {

		if ci.RecycleDay != 0 && ((time.Now().Unix()-ci.LastVisitTime)/60 > ci.RecycleDay*24*60) {
			fileterdInstances = append(fileterdInstances, ci)
		}
	}
	return DeleteCIResources(userName, requestID, fileterdInstances, logger)
}

func DeleteCIResources(userName, requestID string, cis []*models.CollaborationInstance, logger *zap.SugaredLogger) error {
	var names string
	var policyNames []string
	if len(cis) == 0 {
		return nil
	}
	var findOpts []mongodb.CollaborationInstanceFindOptions

	for _, ci := range cis {
		findOpts = append(findOpts, mongodb.CollaborationInstanceFindOptions{
			ProjectName: ci.ProjectName,
			Name:        ci.CollaborationName,
			UserUID:     []string{ci.UserUID},
		})
		policyNames = append(policyNames, ci.PolicyName)
	}

	err := mongodb.NewCollaborationInstanceColl().BulkDelete(mongodb.CollaborationInstanceListOptions{
		FindOpts: findOpts,
	})
	if err != nil {
		logger.Errorf("BulkDelete CollaborationInstance error:%s", err)
		return err
	}
	names = cis[0].PolicyName
	for i := 1; i < len(cis); i++ {
		names = names + "," + cis[i].PolicyName
	}
	res, err := policy.NewDefault().GetPolicies(names)
	if err != nil {
		return err
	}

	var labels []mongodb2.Label
	labelSet := sets.String{}
	for _, re := range res {
		for _, rule := range re.Rules {
			for _, attribute := range rule.MatchAttributes {
				if attribute.Key != "placeholder" &&
					!labelSet.Has(attribute.Key+"-"+attribute.Value) {
					labels = append(labels, mongodb2.Label{
						Key:   attribute.Key,
						Value: attribute.Value,
					})
					labelSet.Insert(attribute.Key + "-" + attribute.Value)
				}
			}
		}
	}

	labelRes, err := service.ListLabels(&service.ListLabelsArgs{
		Labels: labels,
	})
	if err != nil {
		return err
	}
	var labelIds []string
	for _, l := range labelRes.Labels {
		labelIds = append(labelIds, l.ID.Hex())
	}

	err = service.DeleteLabels(labelIds, true, "system", logger)
	if err != nil {
		return err
	}
	for _, ci := range cis {
		for _, workflow := range ci.Workflows {
			if workflow.CollaborationType == config.CollaborationNew {
				err = commonservice.DeleteWorkflow(workflow.Name, requestID, false, logger)
				if err != nil {
					return err
				}
			}
		}
		for _, product := range ci.Products {
			if product.CollaborationType == config.CollaborationNew {
				err = service2.DeleteProduct(userName, product.Name, ci.ProjectName, requestID, true, logger)
				if err != nil {
					return err
				}
			}
		}
	}
	err = policy.NewDefault().DeletePolicies("", policy.DeletePoliciesArgs{
		Names: policyNames,
	})
	if err != nil {
		logger.Errorf("BulkDelete policy error:%s", err)
		return err
	}

	return nil
}

func GetCollaborationNew(projectName, uid, identityType, userName string, logger *zap.SugaredLogger) (*GetCollaborationNewResp, error) {
	updateResp, err := GetCollaborationUpdate(projectName, uid, identityType, userName, logger)
	if err != nil {
		logger.Errorf("GetCollaborationNew error, err msg:%s", err)
		return nil, err
	}
	if updateResp == nil || (updateResp.Update == nil && updateResp.New == nil && updateResp.UpdateInstance == nil && updateResp.Delete == nil) {
		return nil, nil
	}
	return getCollaborationNew(updateResp, projectName, identityType, userName, logger)
}

func getRenderSet(projectName string, envs []string) ([]models2.RenderSet, error) {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		InProjects: []string{projectName},
		InEnvs:     envs,
	})
	if err != nil {
		return nil, err
	}
	var findOpts []commonrepo.RenderSetFindOption
	for _, product := range products {
		findOpts = append(findOpts, commonrepo.RenderSetFindOption{
			Revision:    product.Render.Revision,
			ProductTmpl: projectName,
			EnvName:     product.EnvName,
			Name:        product.Namespace,
		})
	}
	renderSets, err := commonrepo.NewRenderSetColl().ListByFindOpts(&commonrepo.RenderSetListOption{
		ProductTmpl: projectName,
		FindOpts:    findOpts,
	})
	if err != nil {
		return nil, err
	}
	return renderSets, nil
}

func getWorkflowDisplayName(workflowName, workflowType string) string {
	resp := workflowName
	if config2.IsCustomWorkflow(workflowType) {
		workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
		if err != nil {
			log.Errorf("workflow v4 :%s not found", workflowName)
			return resp
		}
		return workflow.DisplayName
	}
	workflow, err := commonrepo.NewWorkflowColl().Find(workflowName)
	if err != nil {
		log.Errorf("workflow :%s not found", workflowName)
		return resp
	}
	return workflow.DisplayName
}
