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
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	taskmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func DeleteDeliveryInfos(productName string, log *zap.SugaredLogger) error {
	deliveryVersions, err := commonrepo.NewDeliveryVersionColl().ListDeliveryVersions(productName)
	if err != nil {
		log.Errorf("delete DeleteDeliveryInfo error: %v", err)
		return e.ErrDeleteDeliveryVersion
	}
	errList := new(multierror.Error)
	for _, deliveryVersion := range deliveryVersions {
		err = commonrepo.NewDeliveryVersionColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryVersion delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = commonrepo.NewDeliveryBuildColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryBuild delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = commonrepo.NewDeliveryDeployColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryDeploy delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = commonrepo.NewDeliveryTestColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryTest delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = commonrepo.NewDeliveryDistributeColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryDistribute delete %s error: %v", deliveryVersion.ID.String(), err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func AddDeliveryVersion(taskID int, productName, workflowName string, pipelineTask *taskmodels.Task, logger *zap.SugaredLogger) error {
	deliveryVersionArgs := &commonrepo.DeliveryVersionArgs{
		ProductName:  productName,
		WorkflowName: workflowName,
		TaskID:       taskID,
	}

	// TODO: LOU: if err != "not found", we should return it.
	versionInfo, _ := GetDeliveryVersion(deliveryVersionArgs, logger)

	if versionInfo != nil {
		return e.ErrInvalidParam.AddDesc("该task版本已经执行过交付!")
	}

	deliveryVersion := new(commonmodels.DeliveryVersion)
	deliveryVersion.WorkflowName = workflowName
	deliveryVersion.TaskID = taskID
	deliveryVersion.ProductName = productName

	if pipelineTask.WorkflowArgs.VersionArgs.Version == "" {
		return e.ErrInvalidParam.AddDesc("version can't be empty!")
	}
	deliveryVersion.CreatedBy = pipelineTask.TaskCreator
	deliveryVersion.CreatedAt = time.Now().Unix()
	deliveryVersion.DeletedAt = 0
	deliveryVersion.Version = pipelineTask.WorkflowArgs.VersionArgs.Version
	deliveryVersion.Desc = pipelineTask.WorkflowArgs.VersionArgs.Desc
	deliveryVersion.Labels = pipelineTask.WorkflowArgs.VersionArgs.Labels
	err := InsertDeliveryVersion(deliveryVersion, logger)
	//getReleaseID 获取task数据
	if err == nil {
		go GetSubTaskContent(deliveryVersion, pipelineTask, logger)
	}

	return err
}

func GetDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) (*commonmodels.DeliveryVersion, error) {
	resp, err := commonrepo.NewDeliveryVersionColl().Get(args)
	if err != nil {
		log.Errorf("get deliveryVersion error: %v", err)
		return nil, e.ErrGetDeliveryVersion
	}
	return resp, err
}

func InsertDeliveryVersion(args *commonmodels.DeliveryVersion, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionColl().Insert(args)
	if err != nil {
		log.Errorf("insert deliveryVersion error: %v", err)
		return e.ErrCreateDeliveryVersion
	}
	return nil
}

// TODO: LOU rewrite it
func GetSubTaskContent(deliveryVersion *commonmodels.DeliveryVersion, pipelineTask *taskmodels.Task, log *zap.SugaredLogger) {
	//获取产品的服务模板信息和变量信息
	productEnvInfo, serviceNames, err := getProductEnvInfo(pipelineTask, log)
	if err != nil {
		log.Errorf("delivery GetInitProduct failed ! err:%v", err)
		return
	}
	stageArray := pipelineTask.Stages
	for _, subStage := range stageArray {
		taskType := subStage.TaskType
		taskStatus := subStage.Status
		switch taskType {
		case config.TaskBuild:
			if taskStatus != config.StatusPassed {
				continue
			}
			subBuildTaskMap := subStage.SubTasks
			for serviceName, subTask := range subBuildTaskMap {
				deliveryBuild := new(commonmodels.DeliveryBuild)
				deliveryBuild.ReleaseID = deliveryVersion.ID
				deliveryBuild.ServiceName = serviceName
				buildInfo, err := base.ToBuildTask(subTask)
				if err != nil {
					log.Errorf("get buildInfo ToBuildTask failed ! err:%v", err)
					continue
				}
				deliveryBuild.StartTime = buildInfo.StartTime
				deliveryBuild.EndTime = buildInfo.EndTime
				imageName := buildInfo.JobCtx.Image
				deliveryBuild.ImageName = imageName
				deliveryBuild.Commits = buildInfo.JobCtx.Builds
				//packageInfo
				if buildInfo.JobCtx.FileArchiveCtx != nil {
					deliveryPackage := new(commonmodels.DeliveryPackage)
					deliveryPackage.PackageFileLocation = buildInfo.JobCtx.FileArchiveCtx.FileLocation
					deliveryPackage.PackageFileName = buildInfo.JobCtx.FileArchiveCtx.FileName
					storageInfo, _ := s3service.NewS3StorageFromEncryptedURI(pipelineTask.StorageURI)
					deliveryPackage.PackageStorageURI = storageInfo.Endpoint
					deliveryBuild.PackageInfo = deliveryPackage
				}
				//issues
				//找到jira这个stage
				for _, jiraSubStage := range stageArray {
					if jiraSubStage.TaskType == config.TaskJira {
						jiraSubBuildTaskMap := jiraSubStage.SubTasks
						var jira *taskmodels.Jira
						for _, jiraSubTask := range jiraSubBuildTaskMap {
							jira, err = base.ToJiraTask(jiraSubTask)
							if err != nil {
								log.Errorf("ToJiraTask failed ! err:%v", err)
								break
							}
						}
						deliveryBuild.Issues = jira.Issues
						break
					}
				}
				deliveryBuild.CreatedAt = time.Now().Unix()
				deliveryBuild.DeletedAt = 0

				err = insertDeliveryBuild(deliveryBuild, log)
				if err != nil {
					log.Errorf("get buildInfo InsertDeliveryBuild failed ! err:%v", err)
					continue
				}
			}
		case config.TaskDeploy:
			if taskStatus != config.StatusPassed {
				continue
			}
			subDeployTaskMap := subStage.SubTasks
			for _, subTask := range subDeployTaskMap {
				deployInfo, err := base.ToDeployTask(subTask)
				if err != nil {
					log.Errorf("get deployInfo failed, err:%v", err)
					continue
				}

				deliveryDeploy := new(commonmodels.DeliveryDeploy)
				deliveryDeploy.ReleaseID = deliveryVersion.ID
				deliveryDeploy.StartTime = deployInfo.StartTime
				deliveryDeploy.EndTime = deployInfo.EndTime
				deliveryDeploy.ServiceName = deployInfo.ServiceName
				deliveryDeploy.ContainerName = deployInfo.ContainerName
				if pipelineTask.WorkflowArgs != nil {
					deliveryDeploy.RegistryID = pipelineTask.WorkflowArgs.RegistryID
				}
				deliveryDeploy.Image = deployInfo.Image

				containers := make([]*commonmodels.Container, 0)
				container := new(commonmodels.Container)

				container.Name = deployInfo.ContainerName
				container.Image = deployInfo.Image
				containers = append(containers, container)

				yamlContent, err := getServiceRenderYAML(productEnvInfo, containers, deployInfo.ServiceName, setting.K8SDeployType, log)
				if err != nil {
					log.Errorf("get deployInfo ExportYamls failed ! err:%v", err)
					continue
				}
				deliveryDeploy.YamlContents = []string{yamlContent}
				//orderedServices
				deliveryDeploy.OrderedServices = serviceNames
				//envs todo
				deliveryDeploy.CreatedAt = time.Now().Unix()
				deliveryDeploy.DeletedAt = 0
				err = insertDeliveryDeploy(deliveryDeploy, log)
				//更新产品的镜像名称
				updateServiceImage(deployInfo.ServiceName, deployInfo.Image, deployInfo.ContainerName, productEnvInfo)
				if err != nil {
					log.Errorf("get deployInfo InsertDeliveryDeploy failed ! err:%v", err)
					continue
				}
			}
			deliveryVersion.ProductEnvInfo = productEnvInfo
			err = updateDeliveryVersion(deliveryVersion, log)
			if err != nil {
				log.Errorf("delivery UpdateDeliveryVersion failed ! err:%v", err)
				return
			}
		case config.TaskTestingV2:
			if taskStatus != config.StatusPassed {
				continue
			}
			subTestTaskMap := subStage.SubTasks
			deliveryTest := new(commonmodels.DeliveryTest)
			deliveryTest.ReleaseID = deliveryVersion.ID
			testReports := make([]commonmodels.TestReportObject, 0)
			for _, subTask := range subTestTaskMap {
				testInfo, err := base.ToTestingTask(subTask)
				if err != nil {
					log.Errorf("get testInfo ToTestingTask failed ! err:%v", err)
					continue
				}
				var TestReportObject commonmodels.TestReportObject
				TestReportObject.TestResultPath = testInfo.JobCtx.TestResultPath
				TestReportObject.TestName = testInfo.TestModuleName
				TestReportObject.WorkflowName = pipelineTask.PipelineName
				TestReportObject.TaskID = pipelineTask.TaskID
				TestReportObject.StartTime = testInfo.StartTime
				TestReportObject.EndTime = testInfo.EndTime
				testName := fmt.Sprintf("%s-%d-%s", pipelineTask.PipelineName, deliveryVersion.TaskID, testInfo.TestName)
				if testInfo.JobCtx.TestType == setting.PerformanceTest { //兼容老数据
					testReport, err := GetLocalTestSuite(pipelineTask.PipelineName, testInfo.TestModuleName, setting.PerformanceTest, int64(deliveryVersion.TaskID), testName, config.WorkflowType, log)
					if err != nil {
						log.Errorf("get testInfo GetLocalTestSuite PerformanceTest failed ! err:%v", err)
						continue
					}
					TestReportObject.PerformanceTestSuites = testReport.PerformanceTestSuites
				} else {
					functionTestReport, err := GetLocalTestSuite(pipelineTask.PipelineName, testInfo.TestModuleName, setting.FunctionTest, int64(deliveryVersion.TaskID), testName, config.WorkflowType, log)
					if err != nil {
						log.Errorf("get testInfo GetLocalTestSuite functionTest failed ! err:%v", err)
						continue
					}
					functionTestReport.FunctionTestSuite.TestCases = nil
					TestReportObject.FunctionTestSuite = functionTestReport.FunctionTestSuite
				}
				testReports = append(testReports, TestReportObject)
			}
			deliveryTest.TestReports = testReports
			deliveryTest.CreatedAt = time.Now().Unix()
			deliveryTest.DeletedAt = 0

			err = InsertDeliveryTest(deliveryTest, log)
			if err != nil {
				log.Errorf("get testInfo InsertDeliveryTest failed ! err:%v", err)
				continue
			}
		case config.TaskReleaseImage:
			if taskStatus != config.StatusPassed {
				continue
			}
			subDistributeTaskMap := subStage.SubTasks
			for serviceName, subTask := range subDistributeTaskMap {
				deliveryDistribute := new(commonmodels.DeliveryDistribute)
				deliveryDistribute.ReleaseID = deliveryVersion.ID
				deliveryDistribute.ServiceName = serviceName
				deliveryDistribute.DistributeType = config.Image
				releaseImageInfo, err := base.ToReleaseImageTask(subTask)
				if err != nil {
					log.Errorf("get releaseImage ToReleaseImageTask failed ! err:%v", err)
					continue
				}
				deliveryDistribute.RegistryName = releaseImageInfo.ImageRelease
				deliveryDistribute.Namespace = releaseImageInfo.ImageRepo
				deliveryDistribute.StartTime = releaseImageInfo.StartTime
				deliveryDistribute.EndTime = releaseImageInfo.EndTime
				deliveryDistribute.CreatedAt = time.Now().Unix()
				deliveryDistribute.DeletedAt = 0

				err = insertDeliveryDistribute(deliveryDistribute, log)
				if err != nil {
					log.Errorf("get releaseImage InsertDeliveryDistribute failed ! err:%v", err)
					continue
				}
			}
		case config.TaskDistributeToS3:
			if taskStatus != config.StatusPassed {
				continue
			}
			subDistributeFileTaskMap := subStage.SubTasks
			for serviceName, subTask := range subDistributeFileTaskMap {
				deliveryDistributeFile := new(commonmodels.DeliveryDistribute)
				deliveryDistributeFile.ReleaseID = deliveryVersion.ID
				deliveryDistributeFile.ServiceName = serviceName
				deliveryDistributeFile.DistributeType = config.File
				releaseFileInfo, err := base.ToDistributeToS3Task(subTask)
				if err != nil {
					log.Errorf("get releasefile ToDistributeToS3Task failed ! err:%v", err)
					continue
				}
				deliveryDistributeFile.PackageFile = releaseFileInfo.PackageFile
				deliveryDistributeFile.RemoteFileKey = releaseFileInfo.RemoteFileKey
				storageInfo, _ := s3service.NewS3StorageFromEncryptedURI(releaseFileInfo.DestStorageURL)
				deliveryDistributeFile.DestStorageURL = storageInfo.Endpoint
				storageInfo, _ = s3service.NewS3StorageFromEncryptedURI(pipelineTask.StorageURI)
				deliveryDistributeFile.SrcStorageURL = storageInfo.Endpoint
				deliveryDistributeFile.StartTime = releaseFileInfo.StartTime
				deliveryDistributeFile.EndTime = releaseFileInfo.EndTime
				deliveryDistributeFile.CreatedAt = time.Now().Unix()
				deliveryDistributeFile.DeletedAt = 0

				err = insertDeliveryDistribute(deliveryDistributeFile, log)
				if err != nil {
					log.Errorf("get releaseFile InsertDeliveryDistribute failed ! err:%v", err)
					continue
				}
			}
		}
	}
}

func getProductEnvInfo(pipelineTask *taskmodels.Task, log *zap.SugaredLogger) (*commonmodels.Product, [][]string, error) {
	product := new(commonmodels.Product)
	product.ProductName = pipelineTask.WorkflowArgs.ProductTmplName
	product.EnvName = pipelineTask.WorkflowArgs.Namespace

	if productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    product.ProductName,
		EnvName: product.EnvName,
	}); err != nil {
		product.Namespace = product.GetNamespace()
	} else {
		product.Namespace = productInfo.Namespace
	}

	//if pipelineTask.Render != nil {
	//	if renderSet, err := GetRenderSet(product.Namespace, pipelineTask.Render.Revision, false, product.EnvName, log); err == nil {
	//		product.Vars = renderSet.KVs
	//	} else {
	//		log.Warnf("GetProductEnvInfo GetRenderSet namespace:%s pipelineTask.Render.Revision:%d err:%v", product.GetNamespace(), pipelineTask.Render.Revision, err)
	//	}
	//}

	//返回中的ProductName即产品模板的名称
	product.Render = pipelineTask.Render
	product.Services = pipelineTask.Services

	return product, product.GetGroupServiceNames(), nil
}

func insertDeliveryBuild(args *commonmodels.DeliveryBuild, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryBuildColl().Insert(args)
	if err != nil {
		log.Errorf("insert deliveryBuild error: %v", err)
		return e.ErrCreateDeliveryBuild
	}
	return nil
}

func insertDeliveryDeploy(args *commonmodels.DeliveryDeploy, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryDeployColl().Insert(args)
	if err != nil {
		log.Errorf("insert deliveryDeploy error: %v", err)
		return e.ErrCreateDeliveryDeploy
	}
	return nil
}

func InsertDeliveryTest(args *commonmodels.DeliveryTest, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryTestColl().Insert(args)
	if err != nil {
		log.Errorf("insert deliveryTest error: %v", err)
		return e.ErrCreateDeliveryTest
	}
	return nil
}

func insertDeliveryDistribute(args *commonmodels.DeliveryDistribute, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryDistributeColl().Insert(args)
	if err != nil {
		log.Errorf("insert deliveryDistribute error: %v", err)
		return e.ErrCreateDeliveryDistribute
	}
	return nil
}

func updateDeliveryVersion(args *commonmodels.DeliveryVersion, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionColl().Update(args)
	if err != nil {
		log.Errorf("update deliveryVersion error: %v", err)
		return e.ErrUpdateDeliveryVersion
	}
	return nil
}

func updateServiceImage(serviceName, image, containerName string, product *commonmodels.Product) {
	for _, services := range product.Services {
		for _, serviceInfo := range services {
			if serviceInfo.ServiceName == serviceName {
				for _, container := range serviceInfo.Containers {
					if container.Name == containerName {
						container.Image = image
						break
					}
				}
				break
			}
		}
	}
}

func getServiceRenderYAML(productInfo *commonmodels.Product, containers []*commonmodels.Container, serviceName, deployType string, log *zap.SugaredLogger) (string, error) {
	if deployType == setting.K8SDeployType {
		opt := &commonrepo.RenderSetFindOption{
			Name:        productInfo.Render.Name,
			Revision:    productInfo.Render.Revision,
			EnvName:     productInfo.EnvName,
			ProductTmpl: productInfo.ProductName,
		}
		newRender, err := commonrepo.NewRenderSetColl().Find(opt)
		if err != nil {
			log.Errorf("[%s][P:%s]renderset Find error: %v", productInfo.EnvName, productInfo.ProductName, err)
			return "", fmt.Errorf("get pure yaml %s error: %v", serviceName, err)
		}

		serviceInfo := productInfo.GetServiceMap()[serviceName]
		if serviceInfo == nil {
			return "", fmt.Errorf("service %s not found", serviceName)
		}
		// 获取服务模板
		serviceFindOption := &commonrepo.ServiceFindOption{
			ServiceName: serviceName,
			ProductName: serviceInfo.ProductName,
			Type:        setting.K8SDeployType,
			Revision:    serviceInfo.Revision,
		}
		svcTmpl, err := commonrepo.NewServiceColl().Find(serviceFindOption)
		if err != nil {
			return "", fmt.Errorf("service template %s error: %v", serviceName, err)
		}

		parsedYaml, err := kube.RenderServiceYaml(svcTmpl.Yaml, productInfo.ProductName, svcTmpl.ServiceName, newRender, svcTmpl.ServiceVars, svcTmpl.VariableYaml)
		if err != nil {
			log.Errorf("RenderServiceYaml failed, err: %s", err)
			return "", err
		}
		// 渲染系统变量键值
		parsedYaml = kube.ParseSysKeys(productInfo.Namespace, productInfo.EnvName, productInfo.ProductName, serviceName, parsedYaml)
		// 替换服务模板容器镜像为用户指定镜像
		parsedYaml = kube.ReplaceContainerImages(parsedYaml, svcTmpl.Containers, containers)

		return parsedYaml, nil
	}
	return "", nil
}
