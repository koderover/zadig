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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/template"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	templaterepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	environmentservice "github.com/koderover/zadig/lib/microservice/aslan/core/environment/service"
	workflowservice "github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types/permission"
)

func GetProductTemplateServices(productName string, log *xlog.Logger) (*template.Product, error) {
	resp, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("GetProductTemplate error: %v", err)
		return nil, e.ErrGetProduct.AddDesc(err.Error())
	}

	err = FillProductTemplateVars([]*template.Product{resp}, log)
	if err != nil {
		return nil, fmt.Errorf("FillProductTemplateVars err : %v", err)
	}

	if resp.Services == nil {
		resp.Services = make([][]string, 0)
	}
	return resp, nil
}

// ListProductTemplate 列出产品模板分页
func ListProductTemplate(userID int, superUser bool, log *xlog.Logger) ([]*template.Product, error) {
	var (
		err            error
		errorList      = &multierror.Error{}
		resp           = make([]*template.Product, 0)
		tmpls          = make([]*template.Product, 0)
		productTmpls   = make([]*template.Product, 0)
		productNameMap = make(map[string][]int64)
		productMap     = make(map[string]*template.Product)
		wg             sync.WaitGroup
		mu             sync.Mutex
		maxRoutineNum  = 20                            // 协程池最大协程数量
		ch             = make(chan int, maxRoutineNum) // 控制协程数量
	)

	poetryCtl := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())

	tmpls, err = templaterepo.NewProductColl().List("")
	if err != nil {
		log.Errorf("ProfuctTmpl.List error: %v", err)
		return resp, e.ErrListProducts.AddDesc(err.Error())
	}

	for _, product := range tmpls {
		if superUser {
			product.Role = setting.RoleAdmin
			product.PermissionUUIDs = []string{}
			product.ShowProject = true
			continue
		}
		productMap[product.ProductName] = product
	}

	if !superUser {
		productNameMap, err = poetryCtl.GetUserProject(userID, log)
		if err != nil {
			log.Errorf("ProfuctTmpl.List GetUserProject error: %v", err)
			return resp, e.ErrListProducts.AddDesc(err.Error())
		}

		// 优先处理客户有明确关联关系的项目
		for productName, roleIDs := range productNameMap {
			wg.Add(1)
			ch <- 1
			// 临时复制range获取的数据，避免重复操作最后一条数据
			tmpProductName := productName
			tmpRoleIDs := roleIDs

			go func(tmpProductName string, tmpRoleIDs []int64) {
				defer func() {
					<-ch
					wg.Done()
				}()

				roleID := tmpRoleIDs[0]
				product, err := templaterepo.NewProductColl().Find(tmpProductName)
				if err != nil {
					errorList = multierror.Append(errorList, err)
					log.Errorf("ProfuctTmpl.List error: %v", err)
					return
				}
				uuids, err := poetryCtl.GetUserPermissionUUIDs(roleID, tmpProductName, log)
				if err != nil {
					errorList = multierror.Append(errorList, err)
					log.Errorf("ProfuctTmpl.List GetUserPermissionUUIDs error: %v", err)
					return
				}
				if roleID == setting.RoleOwnerID {
					product.Role = setting.RoleOwner
					product.PermissionUUIDs = []string{}
				} else {
					product.Role = setting.RoleUser
					product.PermissionUUIDs = uuids
				}
				product.ShowProject = true
				mu.Lock()
				productTmpls = append(productTmpls, product)
				delete(productMap, tmpProductName)
				mu.Unlock()
			}(tmpProductName, tmpRoleIDs)
		}

		wg.Wait()
		if errorList.ErrorOrNil() != nil {
			return resp, errorList
		}

		// 增加项目里面设置过all-users的权限处理
		for _, product := range productMap {
			wg.Add(1)
			ch <- 1
			// 临时复制range获取的数据，避免重复操作最后一条数据
			tmpProduct := product

			go func(tmpProduct *template.Product) {
				defer func() {
					<-ch
					wg.Done()
				}()
				productRole, _ := poetryCtl.ListRoles(tmpProduct.ProductName, log)
				if productRole != nil {
					uuids, err := poetryCtl.GetUserPermissionUUIDs(productRole.ID, tmpProduct.ProductName, log)
					if err != nil {
						errorList = multierror.Append(errorList, err)
						log.Errorf("ProfuctTmpl.List GetUserPermissionUUIDs error: %v", err)
						return
					}
					tmpProduct.Role = setting.RoleUser
					tmpProduct.PermissionUUIDs = uuids
					tmpProduct.ShowProject = true
					mu.Lock()
					productTmpls = append(productTmpls, tmpProduct)
					delete(productMap, tmpProduct.ProductName)
					mu.Unlock()
				}
			}(tmpProduct)
		}
		wg.Wait()
		if errorList.ErrorOrNil() != nil {
			return resp, errorList
		}

		// 最后处理剩余的项目
		for _, product := range productMap {
			wg.Add(1)
			ch <- 1
			// 临时复制range获取的数据，避免重复操作最后一条数据
			tmpProduct := product

			go func(tmpProduct *template.Product) {
				defer func() {
					<-ch
					wg.Done()
				}()

				var uuids []string
				uuids, err = poetryCtl.GetUserPermissionUUIDs(setting.RoleUserID, "", log)
				if err != nil {
					errorList = multierror.Append(errorList, err)
					log.Errorf("ProfuctTmpl.List GetUserPermissionUUIDs error: %v", err)
					return
				}
				tmpProduct.Role = setting.RoleUser
				tmpProduct.PermissionUUIDs = uuids
				tmpProduct.ShowProject = false
				mu.Lock()
				productTmpls = append(productTmpls, tmpProduct)
				mu.Unlock()
			}(tmpProduct)
		}
		wg.Wait()
		if errorList.ErrorOrNil() != nil {
			return resp, errorList
		}
		// 先清空tmpls中的管理员角色数据后，再插入普通用户角色的数据
		tmpls = make([]*template.Product, 0)
		tmpls = append(tmpls, productTmpls...)
	}

	err = FillProductTemplateVars(tmpls, log)
	if err != nil {
		return resp, err
	}

	for _, tmpl := range tmpls {
		wg.Add(1)
		ch <- 1
		// 临时复制range获取的数据，避免重复操作最后一条数据
		tmpTmpl := tmpl

		go func(tmpTmpl *template.Product) {
			defer func() {
				<-ch
				wg.Done()
			}()

			var (
				totalEnvs     []*commonmodels.Product
				totalServices []commonmodels.ServiceTmplRevision
			)

			totalServices, err = commonrepo.NewServiceColl().DistinctServices(&commonrepo.ServiceListOption{ProductName: tmpTmpl.ProductName, ExcludeStatus: setting.ProductStatusDeleting})
			if err != nil {
				errorList = multierror.Append(errorList, err)
				return
			}

			totalEnvs, err = commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: tmpTmpl.ProductName})
			if err != nil {
				errorList = multierror.Append(errorList, err)
				return
			}

			tmpTmpl.TotalServiceNum = len(totalServices)
			tmpTmpl.TotalEnvNum = len(totalEnvs)

			mu.Lock()
			resp = append(resp, tmpTmpl)
			mu.Unlock()
		}(tmpTmpl)
	}
	wg.Wait()
	if errorList.ErrorOrNil() != nil {
		return resp, errorList
	}

	return resp, nil
}

func ListOpenSourceProduct(log *xlog.Logger) ([]*template.Product, error) {
	opt := &templaterepo.ProductListOpt{
		IsOpensource: "true",
	}

	tmpls, err := templaterepo.NewProductColl().ListWithOption(opt)
	if err != nil {
		log.Errorf("ProductTmpl.ListWithOpt error: %v", err)
		return nil, e.ErrListProducts.AddDesc(err.Error())
	}

	return tmpls, nil
}

// CreateProductTemplate 创建产品模板
func CreateProductTemplate(args *template.Product, log *xlog.Logger) (err error) {
	kvs := args.Vars
	// 不保存vas
	args.Vars = nil

	err = commonservice.ValidateKVs(kvs, args.Services, log)
	if err != nil {
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	if err := ensureProductTmpl(args); err != nil {
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	err = templaterepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("ProductTmpl.Create error: %v", err)
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	// 创建一个默认的渲染集
	err = commonservice.CreateRenderSet(&commonmodels.RenderSet{
		Name:        args.ProductName,
		ProductTmpl: args.ProductName,
		UpdateBy:    args.UpdateBy,
		IsDefault:   true,
		KVs:         kvs,
	}, log)

	if err != nil {
		log.Errorf("ProductTmpl.Create error: %v", err)
		// 创建渲染集失败，删除产品模板
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	return
}

// UpdateProductTemplate 更新产品模板
func UpdateProductTemplate(name string, args *template.Product, log *xlog.Logger) (err error) {
	kvs := args.Vars
	args.Vars = nil

	if err = commonservice.ValidateKVs(kvs, args.Services, log); err != nil {
		log.Warnf("ProductTmpl.Update ValidateKVs error: %v", err)
	}

	if err := ensureProductTmpl(args); err != nil {
		return e.ErrUpdateProduct.AddDesc(err.Error())
	}

	if err = templaterepo.NewProductColl().Update(name, args); err != nil {
		log.Errorf("ProductTmpl.Update error: %v", err)
		return e.ErrUpdateProduct
	}
	// 如果是helm的项目，不需要新创建renderset
	if args.ProductFeature != nil && args.ProductFeature.DeployType == setting.HelmDeployType {
		return
	}
	// 更新默认的渲染集
	if err = commonservice.CreateRenderSet(&commonmodels.RenderSet{
		Name:        args.ProductName,
		ProductTmpl: args.ProductName,
		UpdateBy:    args.UpdateBy,
		IsDefault:   true,
		KVs:         kvs,
	}, log); err != nil {
		log.Warnf("ProductTmpl.Update CreateRenderSet error: %v", err)
	}

	for _, envVars := range args.EnvVars {
		//创建集成环境变量
		if err = commonservice.CreateRenderSet(&commonmodels.RenderSet{
			EnvName:     envVars.EnvName,
			Name:        args.ProductName,
			ProductTmpl: args.ProductName,
			UpdateBy:    args.UpdateBy,
			IsDefault:   false,
			KVs:         envVars.Vars,
		}, log); err != nil {
			log.Warnf("ProductTmpl.Update CreateRenderSet error: %v", err)
		}
	}

	// 更新子环境渲染集
	if err = commonservice.UpdateSubRenderSet(args.ProductName, kvs, log); err != nil {
		log.Warnf("ProductTmpl.Update UpdateSubRenderSet error: %v", err)
	}

	return nil
}

// UpdateProductTmplStatus 更新项目onboarding状态
func UpdateProductTmplStatus(productName, onboardingStatus string, log *xlog.Logger) (err error) {
	status, err := strconv.Atoi(onboardingStatus)
	if err != nil {
		log.Errorf("convert onboardingStatus to int failed, error: %v", err)
		return e.ErrUpdateProduct.AddErr(err)
	}

	if err = templaterepo.NewProductColl().UpdateOnboardingStatus(productName, status); err != nil {
		log.Errorf("ProductTmpl.UpdateOnboardingStatus failed, productName:%s, status:%d, error: %v", productName, status, err)
		return e.ErrUpdateProduct.AddErr(err)
	}

	return nil
}

// UpdateProject 更新项目
func UpdateProject(name string, args *template.Product, log *xlog.Logger) (err error) {
	poetryCtl := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())

	//创建团建和项目之间的关系
	_, err = poetryCtl.AddProductTeam(args.ProductName, args.TeamID, args.UserIDs, log)
	if err != nil {
		log.Errorf("Project.Create AddProductTeam error: %v", err)
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	err = templaterepo.NewProductColl().Update(name, args)
	if err != nil {
		log.Errorf("Project.Update error: %v", err)
		return e.ErrUpdateProduct
	}
	return nil
}

// DeleteProductTemplate 删除产品模板
func DeleteProductTemplate(userName, productName string, log *xlog.Logger) (err error) {
	// TODO: do a thorough system check, now only shared svc is checked
	svc, err := commonservice.ListServiceTemplate(productName, log)
	if err != nil {
		log.Errorf("pre delete check failed, err: %+v", err)
		return e.ErrDeleteProduct.AddDesc(err.Error())
	}
	for _, services := range svc.Data {
		if services.Visibility == "public" && services.ProductName == productName {
			//如果有一个属于该项目的共享服务，需要确认是否被其他服务引用
			used, err := CheckServiceUsed(productName, services.Service)
			if err != nil {
				log.Errorf("Check if service used error for service: %s, the error is: %+v", services.Service, err)
				return e.ErrDeleteProduct.AddDesc("验证共享服务是否被引用失败")
			}
			if used {
				return e.ErrDeleteProduct.AddDesc(fmt.Sprintf("共享服务[%s]在其他项目中被引用，请解除引用后删除", services.Service))
			}
		}
	}

	poetryCtl := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())

	//删除项目团队信息
	if err = poetryCtl.DeleteProductTeam(productName, log); err != nil {
		log.Errorf("productTeam.Delete error: %v", err)
		return e.ErrDeleteProduct
	}

	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
	for _, env := range envs {
		if err = commonrepo.NewProductColl().UpdateStatus(env.EnvName, productName, setting.ProductStatusDeleting); err != nil {
			log.Errorf("DeleteProductTemplate Update product Status error: %v", err)
			return e.ErrDeleteProduct
		}
	}

	if err = commonservice.DeleteRenderSet(productName, log); err != nil {
		log.Errorf("DeleteProductTemplate DeleteRenderSet err: %v", err)
		return errors.New(err.Error())
	}

	if err = DeleteTestModules(productName, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s test err: %v", productName, err)
		return errors.New(err.Error())
	}

	if err = commonservice.DeleteWorkflows(productName, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s workflow err: %v", productName, err)
		return errors.New(err.Error())
	}

	if err = commonservice.DeletePipelines(productName, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s pipeline err: %v", productName, err)
		return errors.New(err.Error())
	}

	//删除自由编排工作流
	features, err := commonservice.GetFeatures(log)
	if err != nil {
		log.Errorf("DeleteProductTemplate productName %s getFeatures err: %v", productName, err)
	}

	if strings.Contains(features, string(config.FreestyleType)) {
		if err = DeleteFreeStylePipelines(productName, log); err != nil {
			log.Errorf("DeleteProductTemplate Delete productName %s freestyle pipeline err: %v", productName, err)
		}
	}

	err = templaterepo.NewProductColl().Delete(productName)
	if err != nil {
		log.Errorf("ProductTmpl.Delete error: %v", err)
		return e.ErrDeleteProduct
	}

	err = commonrepo.NewCounterColl().Delete(fmt.Sprintf("product:%s", productName))
	if err != nil {
		log.Errorf("Counter.Delete error: %v", err)
		return errors.New(err.Error())
	}

	//删除交付中心
	//删除构建/删除测试/删除服务
	//删除workflow和历史task
	go func() {
		_ = commonrepo.NewBuildColl().Delete("", "", productName)
		_ = commonrepo.NewServiceColl().Delete("", "", productName, "", 0)
		_ = commonservice.DeleteDeliveryInfos(productName, log)
		_ = DeleteProductsAsync(userName, productName, log)
	}()

	return nil
}

type addUserEnvReq struct {
	UserID          int      `json:"userId"`
	ProductName     string   `json:"productName"`
	EnvName         string   `json:"envName"`
	PermissionUUIDs []string `json:"permissionUUIDs"`
}

func ForkProduct(userId int, username string, args *template.ForkProject, log *xlog.Logger) error {
	poetryClient := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())
	// first check if the product have contributor role, if not, create one
	if !poetryClient.ContributorRoleExist(args.ProductName, log) {
		err := poetryClient.CreateContributorRole(args.ProductName, log)
		if err != nil {
			log.Errorf("Cannot create contributor role for product: %s, the error is: %v", args.ProductName, err)
			return e.ErrForkProduct.AddDesc(err.Error())
		}
	}

	// Give contributor role to this user
	// first look for roleID
	roleID := poetryClient.GetContributorRoleID(args.ProductName, log)
	if roleID < 0 {
		log.Errorf("Failed to get contributor Role ID from poetry client")
		return e.ErrForkProduct.AddDesc("Failed to get contributor Role ID from poetry client")
	}

	prodTmpl, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", args.ProductName, err)
		log.Error(errMsg)
		return e.ErrForkProduct.AddDesc(errMsg)
	}

	prodTmpl.ChartInfos = args.ValuesYamls
	// Load Service
	svcs := [][]*commonmodels.ProductService{}
	for _, names := range prodTmpl.Services {
		servicesResp := make([]*commonmodels.ProductService, 0)

		for _, serviceName := range names {
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   serviceName,
				ExcludeStatus: setting.ProductStatusDeleting,
			}

			serviceTmpls, err := commonrepo.NewServiceColl().List(opt)
			if err != nil {
				errMsg := fmt.Sprintf("[ServiceTmpl.List] %s error: %v", opt.ServiceName, err)
				log.Error(errMsg)
				return e.ErrForkProduct.AddDesc(errMsg)
			}
			for _, serviceTmpl := range serviceTmpls {
				serviceResp := &commonmodels.ProductService{
					ServiceName: serviceTmpl.ServiceName,
					Type:        serviceTmpl.Type,
					Revision:    serviceTmpl.Revision,
				}
				if serviceTmpl.Type == setting.HelmDeployType {
					serviceResp.Containers = make([]*commonmodels.Container, 0)
					for _, c := range serviceTmpl.Containers {
						container := &commonmodels.Container{
							Name:  c.Name,
							Image: c.Image,
						}
						serviceResp.Containers = append(serviceResp.Containers, container)
					}
				}
				servicesResp = append(servicesResp, serviceResp)
			}
		}
		svcs = append(svcs, servicesResp)
	}

	prod := commonmodels.Product{
		ProductName:     prodTmpl.ProductName,
		Revision:        prodTmpl.Revision,
		IsPublic:        false,
		EnvName:         args.EnvName,
		Services:        svcs,
		Source:          setting.HelmDeployType,
		ChartInfos:      prodTmpl.ChartInfos,
		IsForkedProduct: true,
	}

	err = environmentservice.CreateProduct(username, &prod, log)
	if err != nil {
		_, messageMap := e.ErrorMessage(err)
		if description, ok := messageMap["description"]; ok {
			return e.ErrForkProduct.AddDesc(description.(string))
		} else {
			errMsg := fmt.Sprintf("Failed to create env in order to fork product, the error is: %+v", err)
			log.Errorf(errMsg)
			return e.ErrForkProduct.AddDesc(errMsg)
		}
	}

	userList, err := poetryClient.ListPermissionUsers(args.ProductName, roleID, poetry.ProjectType, log)
	newUserList := append(userList, userId)
	err = poetryClient.UpdateUserRole(roleID, poetry.ProjectType, args.ProductName, newUserList, log)
	if err != nil {
		log.Errorf("Failed to update user role, the error is: %v", err)
		return e.ErrForkProduct.AddDesc(fmt.Sprintf("Failed to update user role, the error is: %v", err))
	}

	header := poetryClient.GetRootTokenHeader()
	header.Set("content-type", "application/json")
	permissionList := []string{permission.TestEnvListUUID, permission.TestEnvManageUUID}
	req := addUserEnvReq{
		UserID:          userId,
		ProductName:     args.ProductName,
		EnvName:         args.EnvName,
		PermissionUUIDs: permissionList,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return e.ErrForkProduct.AddDesc(err.Error())
	}
	_, err = poetryClient.Do("/directory/userEnvPermission", "POST", bytes.NewBuffer(reqBody), header)
	if err != nil {
		return e.ErrForkProduct.AddDesc(fmt.Sprintf("Failed to create env permission for user: %s", username))
	}

	workflowPreset, err := workflowservice.PreSetWorkflow(args.ProductName, log)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get workflow preset info, the error is: %+v", err)
		log.Error(errMsg)
		return e.ErrForkProduct.AddDesc(errMsg)
	}

	buildModule := []*commonmodels.BuildModule{}
	artifactModule := []*commonmodels.ArtifactModule{}
	for _, i := range workflowPreset {
		buildModuleVer := "stable"
		if len(i.BuildModuleVers) != 0 {
			buildModuleVer = i.BuildModuleVers[0]
		}
		buildModule = append(buildModule, &commonmodels.BuildModule{
			BuildModuleVer: buildModuleVer,
			Target:         i.Target,
		})
		artifactModule = append(artifactModule, &commonmodels.ArtifactModule{Target: i.Target})
	}

	workflowArgs := &commonmodels.Workflow{
		ArtifactStage:   &commonmodels.ArtifactStage{Enabled: true, Modules: artifactModule},
		BuildStage:      &commonmodels.BuildStage{Enabled: false, Modules: buildModule},
		Name:            args.WorkflowName,
		ProductTmplName: args.ProductName,
		Enabled:         true,
		EnvName:         args.EnvName,
		TestStage:       &commonmodels.TestStage{Enabled: false, Tests: []*commonmodels.TestExecArgs{}},
		SecurityStage:   &commonmodels.SecurityStage{Enabled: false},
		DistributeStage: &commonmodels.DistributeStage{
			Enabled:     false,
			Distributes: []*commonmodels.ProductDistribute{},
			Releases:    []commonmodels.RepoImage{},
		},
		HookCtl:   &commonmodels.WorkflowHookCtrl{Enabled: false, Items: []*commonmodels.WorkflowHook{}},
		Schedules: &commonmodels.ScheduleCtrl{Enabled: false, Items: []*commonmodels.Schedule{}},
		CreateBy:  username,
		UpdateBy:  username,
	}

	return workflowservice.CreateWorkflow(workflowArgs, log)
}

func UnForkProduct(userID int, username, productName, workflowName, envName string, log *xlog.Logger) error {
	poetryClient := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())
	if userEnvPermissions, _ := poetryClient.ListUserEnvPermission(productName, userID, log); len(userEnvPermissions) > 0 {
		if err := poetryClient.DeleteUserEnvPermission(productName, username, userID, log); err != nil {
			return e.ErrUnForkProduct.AddDesc(fmt.Sprintf("Failed to delete env permission for userID: %d, env: %s, productName: %s, the error is: %+v", userID, username, productName, err))
		}
	}

	if _, err := workflowservice.FindWorkflow(workflowName, log); err == nil {
		err = workflowservice.DeleteWorkflow(workflowName, false, log)
		if err != nil {
			log.Errorf("Failed to delete forked workflow: %s, the error is: %+v", workflowName, err)
			return e.ErrUnForkProduct.AddDesc(err.Error())
		}
	}

	if roleId := poetryClient.GetContributorRoleID(productName, log); roleId > 0 {
		err := poetryClient.DeleteUserRole(roleId, poetry.ProjectType, userID, productName, log)
		if err != nil {
			log.Errorf("Failed to Delete user from role candidate, the error is: %v", err)
			return e.ErrUnForkProduct.AddDesc(err.Error())
		}
	}

	if err := commonservice.DeleteProduct(username, envName, productName, log); err != nil {
		_, messageMap := e.ErrorMessage(err)
		if description, ok := messageMap["description"]; ok {
			if description != "not found" {
				return e.ErrUnForkProduct.AddDesc(description.(string))
			}
		} else {
			errMsg := fmt.Sprintf("Failed to delete env %s in order to unfork product, the error is: %+v", envName, err)
			log.Errorf(errMsg)
			return e.ErrUnForkProduct.AddDesc(errMsg)
		}
	}
	return nil
}

func FillProductTemplateVars(productTemplates []*template.Product, log *xlog.Logger) error {
	return commonservice.FillProductTemplateVars(productTemplates, log)
}

// ensureProductTmpl 检查产品模板参数
func ensureProductTmpl(args *template.Product) error {
	if args == nil {
		return errors.New("nil ProductTmpl")
	}

	if len(args.ProductName) == 0 {
		return errors.New("empty product name")
	}

	if !config.ServiceNameRegex.MatchString(args.ProductName) {
		return fmt.Errorf("product name must match %s", config.ServiceNameRegexString)
	}

	// Revision为0表示是新增项目，新增项目不需要进行共享服务的判断，只在编辑项目时进行判断
	if args.Revision != 0 {
		//获取该项目下的所有服务
		serviceTmpls, err := commonrepo.NewServiceColl().DistinctServices(&commonrepo.ServiceListOption{ExcludeStatus: setting.ProductStatusDeleting})
		if err != nil {
			return e.ErrListTemplate.AddDesc(err.Error())
		}

		serviceNames := sets.NewString()
		for _, serviceTmpl := range serviceTmpls {
			if serviceTmpl.ProductName == args.ProductName {
				serviceNames.Insert(serviceTmpl.ServiceName)
				continue
			}

			opt := &commonrepo.ServiceFindOption{
				ServiceName:   serviceTmpl.ServiceName,
				Type:          serviceTmpl.Type,
				Revision:      serviceTmpl.Revision,
				ProductName:   serviceTmpl.ProductName,
				ExcludeStatus: setting.ProductStatusDeleting,
			}

			resp, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				return e.ErrGetTemplate.AddDesc(err.Error())
			}

			if resp.Visibility == setting.PUBLICSERVICE {
				serviceNames.Insert(serviceTmpl.ServiceName)
			}
		}

		for _, serviceGroup := range args.Services {
			for _, service := range serviceGroup {
				if !serviceNames.Has(service) {
					return fmt.Errorf("服务 %s 不存在或者已经不是共享服务!", service)
				}
			}
		}
	}

	// 设置新的版本号
	rev, err := commonrepo.NewCounterColl().GetNextSeq("product:" + args.ProductName)
	if err != nil {
		return fmt.Errorf("get next product template revision error: %v", err)
	}

	args.Revision = rev
	return nil
}

// distincProductServices 查询使用到服务模板的产品模板
func distincProductServices(productName string) (map[string][]string, error) {
	serviceMap := make(map[string][]string)
	products, err := templaterepo.NewProductColl().List(productName)
	if err != nil {
		return serviceMap, err
	}

	for _, product := range products {
		for _, group := range product.Services {
			for _, service := range group {
				if _, ok := serviceMap[service]; !ok {
					serviceMap[service] = []string{product.ProductName}
				} else {
					serviceMap[service] = append(serviceMap[service], product.ProductName)
				}
			}
		}
	}

	return serviceMap, nil
}

func CheckServiceUsed(productName, serviceName string) (bool, error) {
	servicesMap, err := distincProductServices("")
	if err != nil {
		errMsg := fmt.Sprintf("failed to get serviceMap, error: %v", err)
		log.Error(errMsg)
		return false, err
	}

	if _, ok := servicesMap[serviceName]; ok {
		for _, svcProductName := range servicesMap[serviceName] {
			if productName != svcProductName {
				return true, nil
			}
		}
	}
	return false, nil
}

func DeleteFreeStylePipelines(productName string, log *xlog.Logger) error {
	collieApiAddress := config.CollieAPIAddress()
	if collieApiAddress == "" {
		return fmt.Errorf("COLLIE_API_ADDRESS is empty")
	}

	client := poetry.NewPoetryServer(collieApiAddress, config.PoetryAPIRootKey())
	_, err := client.Do("/api/collie/api/pipelines/"+productName, "DELETE", nil, client.GetRootTokenHeader())
	if err != nil {
		log.Errorf("call collie delete pipeline err:%+v", err)
		return fmt.Errorf("call collie delete pipeline err:%+v", err)
	}

	return nil
}

func DeleteProductsAsync(userName, productName string, log *xlog.Logger) error {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
	if err != nil {
		return e.ErrListProducts.AddDesc(err.Error())
	}
	errList := new(multierror.Error)
	for _, env := range envs {
		err = commonservice.DeleteProduct(userName, env.EnvName, productName, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Errorf("DeleteProductsAsync err:%v", err)
		return err
	}
	return nil
}

type ProductInfo struct {
	Value       string         `bson:"value"              json:"value"`
	Label       string         `bson:"label"              json:"label"`
	ServiceInfo []*ServiceInfo `bson:"services"           json:"services"`
}

type ServiceInfo struct {
	Value         string           `bson:"value"              json:"value"`
	Label         string           `bson:"label"              json:"label"`
	ContainerInfo []*ContainerInfo `bson:"containers"         json:"containers"`
}

// ContainerInfo ...
type ContainerInfo struct {
	Value string `bson:"value"              json:"value"`
	Label string `bson:"label"              json:"label"`
}

func ListTemplatesHierachy(userName string, userID int, superUser bool, log *xlog.Logger) ([]*ProductInfo, error) {
	var (
		err          error
		resp         = make([]*ProductInfo, 0)
		productTmpls = make([]*template.Product, 0)
	)

	if superUser {
		productTmpls, err = templaterepo.NewProductColl().List("")
		if err != nil {
			log.Errorf("[%s] ProductTmpl.List error: %v", userName, err)
			return nil, e.ErrListProducts.AddDesc(err.Error())
		}
	} else {
		productNameMap, err := poetry.NewPoetryServer("", "").GetUserProject(userID, log)
		if err != nil {
			log.Errorf("ProfuctTmpl.List GetUserProject error: %v", err)
			return resp, e.ErrListProducts.AddDesc(err.Error())
		}
		for productName, _ := range productNameMap {
			product, err := templaterepo.NewProductColl().Find(productName)
			if err != nil {
				log.Errorf("ProfuctTmpl.List error: %v", err)
				return resp, e.ErrListProducts.AddDesc(err.Error())
			}
			productTmpls = append(productTmpls, product)
		}
	}

	for _, productTmpl := range productTmpls {
		pInfo := &ProductInfo{Value: productTmpl.ProductName, Label: productTmpl.ProductName, ServiceInfo: []*ServiceInfo{}}
		for _, servicesGroup := range productTmpl.Services {
			for _, svc := range servicesGroup {

				opt := &commonrepo.ServiceFindOption{
					ServiceName:   svc,
					ExcludeStatus: setting.ProductStatusDeleting,
				}

				svcTmpl, err := commonrepo.NewServiceColl().Find(opt)
				if err != nil {
					log.Errorf("[%s] ServiceTmpl.Find %s/%s error: %v", userName, productTmpl.ProductName, svc, err)
					return nil, e.ErrGetProduct.AddDesc(err.Error())
				}

				sInfo := &ServiceInfo{Value: svcTmpl.ServiceName, Label: svcTmpl.ServiceName, ContainerInfo: make([]*ContainerInfo, 0)}

				for _, c := range svcTmpl.Containers {
					sInfo.ContainerInfo = append(sInfo.ContainerInfo, &ContainerInfo{Value: c.Name, Label: c.Name})
				}

				pInfo.ServiceInfo = append(pInfo.ServiceInfo, sInfo)
			}
		}
		resp = append(resp, pInfo)
	}
	return resp, nil
}
