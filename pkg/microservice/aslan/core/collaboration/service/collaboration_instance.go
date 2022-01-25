package service

import (
	"fmt"
	"reflect"

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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/shared/client/label"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/util"
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
	Old models.WorkflowCMItem `json:"old"`
	New models.WorkflowCIItem `json:"new"`
}

type UpdateProductItem struct {
	Old models.ProductCMItem `json:"old"`
	New models.ProductCIItem `json:"new"`
}

type Workflow struct {
	CollaborationType config.CollaborationType `json:"collaboration_type"`
	BaseName          string                   `json:"base_name"`
	CollaborationMode string                   `json:"collaboration_mode"`
	Name              string                   `json:"name"`
	Description       string                   `json:"description"`
}

type Product struct {
	CollaborationType config.CollaborationType        `json:"collaboration_type"`
	BaseName          string                          `json:"base_name"`
	CollaborationMode string                          `json:"collaboration_mode"`
	Name              string                          `json:"name"`
	DeployType        string                          `json:"deploy_type"`
	Vars              []*templatemodels.RenderKV      `json:"vars"`
	DefaultValues     string                          `json:"defaultValues"`
	ChartValues       []*commonservice.RenderChartArg `json:"chartValues"`
}
type GetCollaborationNewResp struct {
	Code     int64       `json:"code"`
	Workflow []*Workflow `json:"workflow"`
	Product  []*Product  `json:"product"`
}

type GetCollaborationDeleteResp struct {
	Workflows []string
	Products  []string
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
					Old: cm,
					New: ci,
				})
			}
		} else {
			newWorkflowItems = append(newWorkflowItems, cm)
		}
	}
	for name, ci := range ciwMap {
		if cm, ok := cmwMap[name]; ok {
			if !reflect.DeepEqual(cm.Verbs, ci.Verbs) || cm.CollaborationType != ci.CollaborationType {
				deleteWorkflowItems = append(deleteWorkflowItems, ci)
			}
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
					Old: cm,
					New: ci,
				})
			}
		} else {
			newProductItems = append(newProductItems, cm)
		}
	}
	for name, ci := range cipMap {
		if cm, ok := cmpMap[name]; ok {
			if !reflect.DeepEqual(cm.Verbs, ci.Verbs) || cm.CollaborationType != ci.CollaborationType {
				deleteProductItems = append(deleteProductItems, ci)
			}
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
		ciwMap[workflow.Name] = workflow
	}
	updateWorkflowItems, newWorkflowItems, deleteWorkflowItems := getUpdateWorkflowDiff(cmwMap, ciwMap)
	cmpMap := make(map[string]models.ProductCMItem)
	for _, product := range cm.Products {
		cmpMap[product.Name] = product
	}
	cipMap := make(map[string]models.ProductCIItem)
	for _, product := range ci.Products {
		cipMap[product.Name] = product
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

func buildPolicyName(projectName, mode string, userName string) string {
	return projectName + "-" + mode + "-" + userName + "-" + util.GetRandomString(6)
}

func genCollaborationInstance(mode models.CollaborationMode, projectName, uid, userName string) *models.CollaborationInstance {
	var workflows []models.WorkflowCIItem
	for _, workflow := range mode.Workflows {
		name := workflow.Name
		if workflow.CollaborationType == config.CollaborationNew {
			name = buildName(workflow.Name, mode.Name, userName)
		}
		workflows = append(workflows, models.WorkflowCIItem{
			Name:              name,
			BaseName:          workflow.Name,
			Verbs:             workflow.Verbs,
			CollaborationType: workflow.CollaborationType,
		})
	}
	var products []models.ProductCIItem
	for _, product := range mode.Products {
		name := product.Name
		if product.CollaborationType == config.CollaborationNew {
			name = buildName(product.Name, mode.Name, userName)
		}
		products = append(products, models.ProductCIItem{
			Name:              name,
			BaseName:          product.Name,
			CollaborationType: product.CollaborationType,
			Verbs:             product.Verbs,
		})
	}
	return &models.CollaborationInstance{
		ProjectName:       mode.Name,
		CollaborationName: mode.Name,
		User:              uid,
		PolicyName:        buildPolicyName(projectName, mode.Name, userName),
		Revision:          mode.Revision,
		Workflows:         workflows,
		Products:          products,
	}
}

func getDiff(cmMap map[string]*models.CollaborationMode, ciMap map[string]*models.CollaborationInstance, projectName,
	uid, userName string) (*GetCollaborationUpdateResp, error) {
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
				instance := genCollaborationInstance(*cm, projectName, uid, userName)
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

func GetCollaborationUpdate(projectName, uid, userName string, logger *zap.SugaredLogger) (*GetCollaborationUpdateResp, error) {
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
		UserUID:     uid,
	})
	ciMap := make(map[string]*models.CollaborationInstance)
	for _, instance := range collaborationInstances {
		ciMap[instance.CollaborationName] = instance
	}
	resp, err := getDiff(cmMap, ciMap, projectName, uid, userName)
	if err != nil {
		logger.Errorf("GetCollaborationUpdate error, err msg:%s", err)
		return nil, err
	}
	return resp, nil
}
func buildName(baseName, modeName, userName string) string {
	return modeName + "-" + baseName + "-" + userName + "-" + util.GetRandomString(6)
}

func syncInstance(updateResp *GetCollaborationUpdateResp, projectName, userName, uid string,
	logger *zap.SugaredLogger) (map[string]*models.CollaborationInstance, error) {
	var instances []*models.CollaborationInstance
	modeInstanceMap := make(map[string]*models.CollaborationInstance)
	for _, mode := range updateResp.New {
		instance := genCollaborationInstance(mode, projectName, uid, userName)
		instances = append(instances, instance)
		modeInstanceMap[mode.Name] = instance
	}
	err := mongodb.NewCollaborationInstanceColl().BulkCreate(instances)
	if err != nil {
		logger.Errorf("syncInstance BulkCreate error, error msg:%s", err)
		return nil, err
	}
	for _, instance := range updateResp.UpdateInstance {
		err = mongodb.NewCollaborationInstanceColl().Update(uid, &instance)
		if err != nil {
			logger.Errorf("syncInstance Update error, error msg:%s", err)
			return nil, err
		}
	}
	var findOpts []mongodb.CollaborationInstanceFindOptions
	for _, instance := range updateResp.Delete {
		findOpts = append(findOpts, mongodb.CollaborationInstanceFindOptions{
			Name:        instance.CollaborationName,
			ProjectName: instance.ProjectName,
			UserUID:     instance.User,
		})
	}
	err = mongodb.NewCollaborationInstanceColl().BulkDelete(mongodb.CollaborationInstanceListOptions{
		FindOpts: findOpts,
	})
	if err != nil {
		return nil, err
	}
	return modeInstanceMap, nil
}

func buildPolicyDescription(mode, userName string) string {
	return mode + " " + userName + "的权限"
}

func syncPolicy(updateResp *GetCollaborationUpdateResp, modeInstanceMap map[string]*models.CollaborationInstance, projectName, userName string,
	logger *zap.SugaredLogger) error {
	var policies []*policy.Policy
	for _, mode := range updateResp.New {
		var rules []*policy.Rule
		var policyName string
		if instance, ok := modeInstanceMap[mode.Name]; ok {
			policyName = instance.PolicyName
		} else {
			return fmt.Errorf("mode:%s not exist policyName", mode.Name)
		}
		for _, workflow := range mode.Workflows {
			rules = append(rules, &policy.Rule{
				Verbs:     workflow.Verbs,
				Kind:      "resource",
				Resources: []string{"Workflow"},
				MatchAttributes: []policy.MatchAttribute{
					{
						Key:   "policy",
						Value: policyName,
					},
				},
			})
		}
		for _, product := range mode.Products {
			rules = append(rules, &policy.Rule{
				Verbs:     product.Verbs,
				Kind:      "resource",
				Resources: []string{string(config2.ResourceTypeProduct)},
				MatchAttributes: []policy.MatchAttribute{
					{
						Key:   "policy",
						Value: policyName,
					},
				},
			})
		}
		policies = append(policies, &policy.Policy{
			Name:        policyName,
			Description: buildPolicyDescription(mode.Name, userName),
			Rules:       rules,
		})
	}
	err := policy.NewDefault().CreatePolicies(projectName, policy.CreatePoliciesArgs{
		Policies: policies,
	})
	if err != nil {
		logger.Errorf("syncPolicy error, error msg:%s", err)
		return err
	}
	var updatePolicies []*policy.Policy
	for _, instance := range updateResp.UpdateInstance {
		var rules []*policy.Rule
		for _, workflow := range instance.Workflows {
			rules = append(rules, &policy.Rule{
				Verbs:     workflow.Verbs,
				Kind:      "resource",
				Resources: []string{"Workflow"},
				MatchAttributes: []policy.MatchAttribute{
					{
						Key:   "policy",
						Value: instance.PolicyName,
					},
				},
			})
		}
		policies = append(policies, &policy.Policy{
			Name:  instance.PolicyName,
			Rules: rules,
		})
	}
	for _, updatePolicy := range updatePolicies {
		err = policy.NewDefault().UpdatePolicy(projectName, updatePolicy)
		if err != nil {
			return err
		}
	}
	var deletePolicies []string
	for _, instance := range updateResp.Delete {
		deletePolicies = append(deletePolicies, instance.PolicyName)
	}
	err = policy.NewDefault().DeletePolicies(projectName, policy.DeletePoliciesArgs{
		Names: deletePolicies,
	})
	if err != nil {
		return err
	}
	return nil
}

func syncLabel(updateResp *GetCollaborationUpdateResp, modeInstanceMap map[string]*models.CollaborationInstance,
	projectName string, logger *zap.SugaredLogger) error {
	var labels []label.Label
	var newBindings []label.LabelBinding
	var deleteBindings []label.LabelBinding
	for _, mode := range updateResp.New {
		instance, ok := modeInstanceMap[mode.Name]
		if ok {
			labels = append(labels, label.Label{
				Key:   "policy",
				Value: instance.PolicyName,
			})
		} else {
			return fmt.Errorf("mode:%s not exist policyName", mode.Name)
		}
	}
	var labelModels []label.LabelModel
	for _, item := range updateResp.Update {
		labelModels = append(labelModels, label.LabelModel{
			Key:   "policy",
			Value: item.PolicyName,
		})
	}
	for _, item := range updateResp.Delete {
		labelModels = append(labelModels, label.LabelModel{
			Key:   "policy",
			Value: item.PolicyName,
		})
	}
	labelListResp, err := label.New().ListLabels(label.ListLabelsArgs{
		Labels: labelModels,
	})
	labelIdMap := make(map[string]string)
	for _, label := range labelListResp.Labels {
		labelIdMap[service.BuildLabelString(label.Key, label.Value)] = label.ID.Hex()
	}
	labelResp, err := label.New().CreateLabels(label.CreateLabelsArgs{
		Labels: labels,
	})

	if err != nil {
		logger.Errorf("create labels error, error msg:%s", err)
		return err
	}
	for _, mode := range updateResp.New {
		instance, ok := modeInstanceMap[mode.Name]
		if !ok {
			return fmt.Errorf("mode:%s not exist instance", mode.Name)
		}
		labelId, ok := labelResp.LabelMap[service.BuildLabelString("policy", instance.PolicyName)]
		if !ok {
			return fmt.Errorf("label:%s not exist", instance.PolicyName)
		}
		for _, workflow := range instance.Workflows {
			newBindings = append(newBindings, label.LabelBinding{
				LabelID: labelId,
				Resource: label.Resource{
					Name:        workflow.Name,
					ProjectName: projectName,
					Type:        "Workflow",
				},
			})
		}
		for _, product := range instance.Products {
			newBindings = append(newBindings, label.LabelBinding{
				LabelID: labelId,
				Resource: label.Resource{
					Name:        product.Name,
					ProjectName: projectName,
					Type:        string(config2.ResourceTypeProduct),
				},
			})
		}
	}
	for _, item := range updateResp.Update {
		labelId, ok := labelResp.LabelMap[service.BuildLabelString("policy", item.PolicyName)]
		if !ok {
			return fmt.Errorf("label:%s not exist", item.PolicyName)
		}
		for _, workflow := range item.NewSpec.Workflows {
			newBindings = append(newBindings, label.LabelBinding{
				LabelID: labelId,
				Resource: label.Resource{
					Name:        workflow.Name,
					Type:        "Workflow",
					ProjectName: projectName,
				},
			})
		}
		for _, product := range item.NewSpec.Products {
			newBindings = append(newBindings, label.LabelBinding{
				LabelID: labelId,
				Resource: label.Resource{
					Name:        product.Name,
					Type:        string(config2.ResourceTypeProduct),
					ProjectName: projectName,
				},
			})
		}
		for _, workflow := range item.UpdateSpec.Workflows {
			if workflow.Old.CollaborationType == config.CollaborationShare && workflow.New.CollaborationType ==
				config.CollaborationNew {
				newBindings = append(newBindings, label.LabelBinding{
					LabelID: labelId,
					Resource: label.Resource{
						Name:        workflow.New.Name,
						Type:        "Workflow",
						ProjectName: projectName,
					},
				})
			}
			if workflow.Old.CollaborationType == config.CollaborationNew && workflow.New.CollaborationType ==
				config.CollaborationShare {
				deleteBindings = append(deleteBindings, label.LabelBinding{
					LabelID: labelId,
					Resource: label.Resource{
						Name:        workflow.New.Name,
						Type:        "Workflow",
						ProjectName: projectName,
					},
				})
			}
		}
		for _, product := range item.UpdateSpec.Products {
			if product.Old.CollaborationType == config.CollaborationShare && product.New.CollaborationType ==
				config.CollaborationNew {
				newBindings = append(newBindings, label.LabelBinding{
					LabelID: labelId,
					Resource: label.Resource{
						Name:        product.New.Name,
						Type:        string(config2.ResourceTypeProduct),
						ProjectName: projectName,
					},
				})
			}
			if product.Old.CollaborationType == config.CollaborationNew && product.New.CollaborationType ==
				config.CollaborationShare {
				deleteBindings = append(deleteBindings, label.LabelBinding{
					LabelID: labelId,
					Resource: label.Resource{
						Name:        product.Old.Name,
						Type:        string(config2.ResourceTypeProduct),
						ProjectName: projectName,
					},
				})
			}
		}
	}
	var deleteLabels []string
	for _, item := range updateResp.Delete {
		labelId, ok := labelResp.LabelMap[service.BuildLabelString("policy", item.PolicyName)]
		if !ok {
			return fmt.Errorf("label:%s not exist", item.PolicyName)
		}
		for _, workflow := range item.Workflows {
			deleteBindings = append(deleteBindings, label.LabelBinding{
				LabelID: labelId,
				Resource: label.Resource{
					Name:        workflow.Name,
					ProjectName: projectName,
					Type:        "Workflow",
				},
			})
		}
		for _, product := range item.Products {
			deleteBindings = append(deleteBindings, label.LabelBinding{
				LabelID: labelId,
				Resource: label.Resource{
					Name:        product.Name,
					ProjectName: projectName,
					Type:        string(config2.ResourceTypeProduct),
				},
			})
		}
	}
	err = label.New().DeleteLabels(deleteLabels)
	if err != nil {
		logger.Errorf("delete labels error:%s", err)
		return err
	}
	err = label.New().DeleteLabelBindings(label.DeleteLabelBindingsArgs{
		LabelBindings: deleteBindings,
	})
	if err != nil {
		logger.Errorf("delete labelbindings error:%s", err)
		return err
	}
	err = label.New().CreateLabelBindings(label.CreateLabelBindingsArgs{
		LabelBindings: newBindings,
	})
	if err != nil {
		logger.Errorf("create labelbindings error:%s", err)
		return err
	}
	return nil
}

func syncResource(products *SyncCollaborationInstanceArgs, updateResp *GetCollaborationUpdateResp, projectName, userName, requestID string,
	logger *zap.SugaredLogger) error {
	err := syncNewResource(products, updateResp, projectName, userName, requestID, logger)
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
		err := commonservice.DeleteProduct(username, product, projectName, requestID, log)
		if err != nil {
			return err
		}
	}
	for _, workflow := range deleteResp.Workflows {
		err := commonservice.DeleteWorkflow(workflow, requestID, false, log)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncNewResource(products *SyncCollaborationInstanceArgs, updateResp *GetCollaborationUpdateResp, projectName, userName, requestID string,
	logger *zap.SugaredLogger) error {
	newResp, err := getCollaborationNew(updateResp, projectName, userName, logger)
	if err != nil {
		return err
	}
	var newWorkflows []workflowservice.WorkflowCopyItem
	for _, workflow := range newResp.Workflow {
		if workflow.CollaborationType == config.CollaborationNew {
			newWorkflows = append(newWorkflows, workflowservice.WorkflowCopyItem{
				ProjectName: projectName,
				Old:         workflow.BaseName,
				New:         workflow.Name,
			})
		}
	}
	err = workflowservice.BulkCopyWorkflow(workflowservice.BulkCopyWorkflowArgs{
		Items: newWorkflows,
	}, userName, logger)
	if err != nil {
		return err
	}
	productMap := make(map[string]Product)
	for _, product := range products.Products {
		productMap[product.BaseName] = product
	}
	var yamlProductItems []service2.YamlProductItem
	var helmProductArgs []service2.HelmProductItem
	for _, product := range newResp.Product {
		if productArg, ok := productMap[product.BaseName]; ok {
			if product.DeployType == string(config.HelmDeploy) {
				yamlProductItems = append(yamlProductItems, service2.YamlProductItem{
					OldName: product.BaseName,
					NewName: product.Name,
					Vars:    productArg.Vars,
				})
			}
			if product.DeployType == string(config.K8sDeploy) {
				helmProductArgs = append(helmProductArgs, service2.HelmProductItem{
					OldName:       product.BaseName,
					NewName:       product.Name,
					DefaultValues: product.DefaultValues,
					ChartValues:   product.ChartValues,
				})
			}
		}
	}
	err = service2.BulkCopyYamlProduct(projectName, userName, requestID, service2.CopyYamlProductArg{
		Items: yamlProductItems,
	}, logger)
	if err != nil {
		return err
	}
	err = service2.BulkCopyHelmProduct(projectName, userName, requestID, service2.CopyHelmProductArg{
		Items: helmProductArgs,
	}, logger)
	if err != nil {
		return err
	}
	return nil
}

type SyncCollaborationInstanceArgs struct {
	Products []Product `json:"products"`
}

func SyncCollaborationInstance(products *SyncCollaborationInstanceArgs, projectName, uid, userName, requestID string, logger *zap.SugaredLogger) error {
	updateResp, err := GetCollaborationUpdate(projectName, uid, userName, logger)
	if err != nil {
		logger.Errorf("GetCollaborationNew error, err msg:%s", err)
		return err
	}
	modeInstanceMap, err := syncInstance(updateResp, projectName, userName, uid, logger)
	if err != nil {
		return err
	}
	err = syncResource(products, updateResp, projectName, userName, requestID, logger)
	err = syncPolicy(updateResp, modeInstanceMap, projectName, userName, logger)
	if err != nil {
		return err
	}
	err = syncLabel(updateResp, modeInstanceMap, projectName, logger)
	return nil
}

func getCollaborationDelete(updateResp *GetCollaborationUpdateResp) *GetCollaborationDeleteResp {
	productSet := sets.String{}
	workflowSet := sets.String{}
	for _, item := range updateResp.Delete {
		for _, product := range item.Products {
			productSet.Insert(product.Name)
		}
		for _, workflow := range item.Workflows {
			workflowSet.Insert(workflow.Name)
		}
	}
	for _, item := range updateResp.Update {
		for _, deleteWorkflow := range item.DeleteSpec.Workflows {
			workflowSet.Insert(deleteWorkflow.Name)
		}
		for _, deleteProduct := range item.DeleteSpec.Products {
			productSet.Insert(deleteProduct.Name)
		}
	}
	return &GetCollaborationDeleteResp{
		Workflows: workflowSet.List(),
		Products:  productSet.List(),
	}
}

func getCollaborationNew(updateResp *GetCollaborationUpdateResp, projectName, userName string,
	logger *zap.SugaredLogger) (*GetCollaborationNewResp, error) {
	var newWorkflow []*Workflow
	var newProduct []*Product
	newProductName := sets.String{}
	for _, mode := range updateResp.New {
		for _, workflow := range mode.Workflows {
			name := workflow.Name
			if workflow.CollaborationType == config.CollaborationNew {
				name = buildName(workflow.Name, mode.Name, userName)
			}
			newWorkflow = append(newWorkflow, &Workflow{
				CollaborationType: workflow.CollaborationType,
				BaseName:          workflow.Name,
				CollaborationMode: mode.Name,
				Name:              name,
			})
		}
		for _, product := range mode.Products {
			name := product.Name
			if product.CollaborationType == config.CollaborationNew {
				name = buildName(product.Name, mode.Name, userName)
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
			if workflow.CollaborationType == config.CollaborationNew {
				name = buildName(workflow.Name, item.CollaborationMode, userName)
			}
			newWorkflow = append(newWorkflow, &Workflow{
				CollaborationType: workflow.CollaborationType,
				BaseName:          workflow.Name,
				CollaborationMode: item.CollaborationMode,
				Name:              name,
			})
		}
		for _, product := range item.NewSpec.Products {
			name := product.Name
			if product.CollaborationType == config.CollaborationNew {
				name = buildName(product.Name, item.CollaborationMode, userName)
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
			if workflow.Old.CollaborationType == "share" && workflow.New.CollaborationType == "new" {
				newWorkflow = append(newWorkflow, &Workflow{
					CollaborationType: workflow.New.CollaborationType,
					BaseName:          workflow.New.BaseName,
					CollaborationMode: item.CollaborationMode,
					Name:              buildName(workflow.New.BaseName, item.CollaborationMode, userName),
				})
			}
		}
		for _, product := range item.UpdateSpec.Products {
			if product.Old.CollaborationType == "share" && product.New.CollaborationType == "new" {
				newProduct = append(newProduct, &Product{
					CollaborationType: product.New.CollaborationType,
					BaseName:          product.New.BaseName,
					CollaborationMode: item.CollaborationMode,
					DeployType:        item.DeployType,
					Name:              buildName(product.New.BaseName, item.CollaborationMode, userName),
				})
				newProductName.Insert(product.New.BaseName)
			}
		}
	}
	renderSets, err := getRenderSet(projectName, newProductName.List())
	if err != nil {
		return nil, err
	}
	if renderSets != nil {
		productRenderSetMap := make(map[string]models2.RenderSet)
		for _, set := range renderSets {
			productRenderSetMap[set.EnvName] = set
		}
		for _, product := range newProduct {
			if set, ok := productRenderSetMap[product.BaseName]; ok {
				product.Vars = set.KVs
				product.DefaultValues = set.DefaultValues
				product.ChartValues = buildRenderChartArg(set.ChartInfos, product.BaseName)
			} else {
				return nil, fmt.Errorf("product:%s not exist", product.BaseName)
			}
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
	return &GetCollaborationNewResp{
		Code:     10000,
		Workflow: newWorkflow,
		Product:  newProduct,
	}, nil
}
func GetCollaborationNew(projectName, uid, userName string, logger *zap.SugaredLogger) (*GetCollaborationNewResp, error) {
	updateResp, err := GetCollaborationUpdate(projectName, uid, userName, logger)
	if err != nil {
		logger.Errorf("GetCollaborationNew error, err msg:%s", err)
		return nil, err
	}
	return getCollaborationNew(updateResp, projectName, userName, logger)
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
			Revision: product.Revision,
			Name:     product.Namespace,
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

func buildRenderChartArg(chartInfos []*templatemodels.RenderChart, envName string) []*commonservice.RenderChartArg {
	ret := make([]*commonservice.RenderChartArg, 0)
	for _, singleChart := range chartInfos {
		rcaObj := new(commonservice.RenderChartArg)
		rcaObj.LoadFromRenderChartModel(singleChart)
		rcaObj.EnvName = envName
		ret = append(ret, rcaObj)
	}
	return ret
}
