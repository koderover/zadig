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
	"github.com/koderover/zadig/pkg/util"
)

type GetCollaborationUpdateResp struct {
	Update []UpdateItem                   `json:"update"`
	New    []models.CollaborationMode     `json:"new"`
	Delete []models.CollaborationInstance `json:"delete"`
}
type UpdateItem struct {
	CollaborationMode string     `json:"collaboration_mode"`
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
	CollaborationType config.CollaborationType   `json:"collaboration_type"`
	BaseName          string                     `json:"base_name"`
	CollaborationMode string                     `json:"collaboration_mode"`
	Name              string                     `json:"name"`
	DeployType        string                     `json:"deploy_type"`
	Vars              []*templatemodels.RenderKV `json:"vars"`
}
type GetCollaborationNewResp struct {
	Code     int64       `json:"code"`
	Workflow []*Workflow `json:"workflow"`
	Product  []*Product  `json:"product"`
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

func getDiff(cmMap map[string]*models.CollaborationMode, ciMap map[string]*models.CollaborationInstance) (*GetCollaborationUpdateResp, error) {
	var updateItems []UpdateItem
	var newItems []models.CollaborationMode
	var deleteItems []models.CollaborationInstance
	for name, cm := range cmMap {
		if ci, ok := ciMap[name]; ok {
			if cm.Revision < ci.Revision {
				return nil, fmt.Errorf("CollaborationMode:%s revision error", name)
			} else if cm.Revision > ci.Revision {
				updateItems = append(updateItems, getUpdateDiff(cm, ci))
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
		Update: updateItems,
		New:    newItems,
		Delete: deleteItems,
	}, nil
}

func GetCollaborationUpdate(projectName, uid string, logger *zap.SugaredLogger) (*GetCollaborationUpdateResp, error) {
	collaborations, err := mongodb.NewCollaborationModeColl().List(&mongodb.CollaborationModeListOptions{
		Projects: []string{projectName},
		Members:  uid,
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
	resp, err := getDiff(cmMap, ciMap)
	if err != nil {
		logger.Errorf("GetCollaborationUpdate error, err msg:%s", err)
		return nil, err
	}
	return resp, nil
}
func buildName(baseName, modeName, userName string) string {
	return modeName + "-" + baseName + "-" + userName + "-" + util.GetRandomString(6)
}

func GetCollaborationNew(projectName, uid, userName string, logger *zap.SugaredLogger) (*GetCollaborationNewResp, error) {
	var newWorkflow []*Workflow
	var newProduct []*Product
	newProductName := sets.String{}
	updateResp, err := GetCollaborationUpdate(projectName, uid, logger)
	if err != nil {
		logger.Errorf("GetCollaborationNew error, err msg:%s", err)
		return nil, err
	}
	for _, mode := range updateResp.New {
		for _, workflow := range mode.Workflows {
			newWorkflow = append(newWorkflow, &Workflow{
				CollaborationType: workflow.CollaborationType,
				BaseName:          workflow.Name,
				CollaborationMode: mode.Name,
				Name:              buildName(workflow.Name, mode.Name, userName),
			})
		}
		for _, product := range mode.Products {
			newProduct = append(newProduct, &Product{
				CollaborationType: product.CollaborationType,
				BaseName:          product.Name,
				CollaborationMode: mode.Name,
				Name:              buildName(product.Name, mode.Name, userName),
				DeployType:        mode.DeployType,
			})
			newProductName.Insert(product.Name)
		}
	}
	for _, item := range updateResp.Update {
		for _, workflow := range item.NewSpec.Workflows {
			newWorkflow = append(newWorkflow, &Workflow{
				CollaborationType: workflow.CollaborationType,
				BaseName:          workflow.Name,
				CollaborationMode: item.CollaborationMode,
				Name:              buildName(workflow.Name, item.CollaborationMode, userName),
			})
		}
		for _, product := range item.NewSpec.Products {
			newProduct = append(newProduct, &Product{
				CollaborationType: product.CollaborationType,
				BaseName:          product.Name,
				CollaborationMode: item.CollaborationMode,
				Name:              buildName(product.Name, item.CollaborationMode, userName),
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
	renderSets, err := getRenderSetKvs(projectName, newProductName.List())
	if err != nil {
		return nil, err
	}
	if renderSets != nil {
		productVarsMap := make(map[string][]*templatemodels.RenderKV)
		for _, set := range renderSets {
			productVarsMap[set.EnvName] = set.KVs
		}
		for _, product := range newProduct {
			if vars, ok := productVarsMap[product.BaseName]; ok {
				product.Vars = vars
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

func getRenderSetKvs(projectName string, envs []string) ([]models2.RenderSet, error) {
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
