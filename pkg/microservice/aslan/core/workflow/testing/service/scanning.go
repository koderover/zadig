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
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/sonar"
	"github.com/koderover/zadig/v2/pkg/types"
	jobspec "github.com/koderover/zadig/v2/pkg/types/job"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

func CreateScanningModule(username string, args *Scanning, log *zap.SugaredLogger) error {
	if len(args.Name) == 0 {
		return e.ErrCreateScanningModule.AddDesc("empty Name")
	}

	err := util.CheckDefineResourceParam(args.AdvancedSetting.ResReq, args.AdvancedSetting.ResReqSpec)
	if err != nil {
		return e.ErrCreateScanningModule.AddErr(err)
	}

	err = commonservice.ProcessWebhook(args.AdvancedSetting.HookCtl.Items, nil, webhook.ScannerPrefix+args.Name, log)
	if err != nil {
		return e.ErrCreateScanningModule.AddErr(err)
	}

	scanningModule := ConvertToDBScanningModule(args)
	scanningModule.UpdatedBy = username

	err = commonrepo.NewScanningColl().Create(scanningModule)

	if err != nil {
		log.Errorf("Create scanning module %s error: %s", args.Name, err)
		return e.ErrCreateScanningModule.AddErr(err)
	}

	return nil
}

func UpdateScanningModule(id, username string, args *Scanning, log *zap.SugaredLogger) error {
	if len(args.Name) == 0 {
		return e.ErrUpdateScanningModule.AddDesc("empty Name")
	}

	scanning, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning information to update webhook, err: %s", err)
		return err
	}

	err = util.CheckDefineResourceParam(args.AdvancedSetting.ResReq, args.AdvancedSetting.ResReqSpec)
	if err != nil {
		return e.ErrUpdateScanningModule.AddErr(err)
	}

	if scanning.AdvancedSetting.HookCtl.Enabled {
		err = commonservice.ProcessWebhook(args.AdvancedSetting.HookCtl.Items, scanning.AdvancedSetting.HookCtl.Items, webhook.ScannerPrefix+args.Name, log)
		if err != nil {
			log.Errorf("failed to process webhook for scanning: %s, the error is: %s", args.Name, err)
			return e.ErrUpdateScanningModule.AddErr(err)
		}
	} else {
		err = commonservice.ProcessWebhook(args.AdvancedSetting.HookCtl.Items, nil, webhook.ScannerPrefix+args.Name, log)
		if err != nil {
			log.Errorf("failed to process webhook for scanning: %s, the error is: %s", args.Name, err)
			return e.ErrUpdateScanningModule.AddErr(err)
		}
	}

	scanningModule := ConvertToDBScanningModule(args)
	scanningModule.UpdatedBy = username

	err = commonrepo.NewScanningColl().Update(id, scanningModule)

	if err != nil {
		log.Errorf("update scanning module %s error: %s", args.Name, err)
		return e.ErrUpdateScanningModule.AddErr(err)
	}

	return nil
}

func ListScanningModule(projectName string, log *zap.SugaredLogger) ([]*ListScanningRespItem, int64, error) {
	scanningList, total, err := commonrepo.NewScanningColl().List(&commonrepo.ScanningListOption{ProjectName: projectName}, 0, 0)
	if err != nil {
		log.Errorf("failed to list scanning list from mongodb, the error is: %s", err)
		return nil, 0, err
	}
	resp := make([]*ListScanningRespItem, 0)
	for _, scanning := range scanningList {
		res, err := ListScanningTask(scanning.ID.Hex(), 0, 0, log)
		if err != nil {
			log.Errorf("failed to get scanning task statistics, error is: %s", err)
			return nil, 0, err
		}
		var timesTaken int64
		for _, scanTask := range res.ScanTasks {
			timesTaken += scanTask.RunTime
		}
		var avgRuntime int64
		if len(res.ScanTasks) > 0 {
			avgRuntime = timesTaken / int64(len(res.ScanTasks))
		} else {
			avgRuntime = 0
		}

		item := &ListScanningRespItem{
			ID:          scanning.ID.Hex(),
			Name:        scanning.Name,
			Description: scanning.Description,
			Statistics: &ScanningStatistic{
				TimesRun:       res.TotalTasks,
				AverageRuntime: avgRuntime,
			},
			Repos:     scanning.Repos,
			CreatedAt: scanning.CreatedAt,
			UpdatedAt: scanning.UpdatedAt,
			ClusterID: scanning.AdvancedSetting.ClusterID,
			Envs:      scanning.Envs,
		}

		if scanning.TemplateID != "" {
			tmpl, err := commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{ID: scanning.TemplateID})
			if err != nil {
				// we print error in logger but we don't block the listing
				log.Errorf("failed to find scanning of id: %s, error: %s", scanning.TemplateID, err)
			} else {
				item.Envs = renderKeyVals(scanning.Envs, tmpl.Envs)
			}
		}

		resp = append(resp, item)
	}
	return resp, total, nil
}

func GetScanningModuleByID(id string, log *zap.SugaredLogger) (*Scanning, error) {
	scanning, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return nil, err
	}

	if scanning.AdvancedSetting != nil {
		if scanning.AdvancedSetting.StrategyID == "" {
			clusterID := scanning.AdvancedSetting.ClusterID
			if clusterID == "" {
				clusterID = setting.LocalClusterID
			}
			cluster, err := commonrepo.NewK8SClusterColl().FindByID(clusterID)
			if err != nil {
				if err != mongo.ErrNoDocuments {
					return nil, fmt.Errorf("failed to find cluster %s, error: %v", scanning.AdvancedSetting.ClusterID, err)
				}
			} else if cluster.AdvancedConfig != nil {
				strategies := cluster.AdvancedConfig.ScheduleStrategy
				for _, strategy := range strategies {
					if strategy.Default {
						scanning.AdvancedSetting.StrategyID = strategy.StrategyID
						break
					}
				}
			}
		}

		if scanning.AdvancedSetting.Cache == nil {
			scanning.AdvancedSetting.Cache = &commonmodels.ScanningCacheSetting{
				CacheEnable:  false,
				CacheDirType: "",
				CacheUserDir: "",
			}
		}

		if scanning.AdvancedSetting.ConcurrencyLimit == 0 {
			scanning.AdvancedSetting.ConcurrencyLimit = -1
		}
	}

	for _, notify := range scanning.AdvancedSetting.NotifyCtls {
		err := notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
	}

	if scanning.TemplateID != "" {
		tmpl, err := commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{ID: scanning.TemplateID})
		if err != nil {
			// we print error in logger but we don't block the listing
			log.Errorf("failed to find scanning of id: %s, error: %s", scanning.TemplateID, err)
			return nil, err
		}
		scanning.Envs = renderKeyVals(scanning.Envs, tmpl.Envs)
	}

	return ConvertDBScanningModule(scanning), nil
}

func DeleteScanningModuleByID(id string, log *zap.SugaredLogger) error {
	scanning, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return err
	}

	err = commonservice.ProcessWebhook(nil, scanning.AdvancedSetting.HookCtl.Items, webhook.ScannerPrefix+scanning.Name, log)
	if err != nil {
		log.Errorf("failed to process webhook for scanning module: %s, the error is: %s", id, err)
		return err
	}

	err = commonrepo.NewScanningColl().DeleteByID(id)
	if err != nil {
		log.Errorf("failed to delete scanning from mongodb, the error is: %s", err)
	}
	return err
}

// CreateScanningTask uses notificationID if the task is triggered by webhook, otherwise it should be empty
func CreateScanningTask(id string, req []*ScanningRepoInfo, notificationID, username string, log *zap.SugaredLogger) (int64, error) {
	scanningInfo, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return 0, err
	}

	// if a scanning uses template, we use the information in the template
	if len(scanningInfo.TemplateID) != 0 {
		templateInfo, err := commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{
			ID: scanningInfo.TemplateID,
		})
		if err != nil {
			log.Errorf("failed to get scanning template from mongodb, the error is: %s", err)
			return 0, err
		}

		scanningInfo.ScannerType = templateInfo.ScannerType
		scanningInfo.EnableScanner = templateInfo.EnableScanner
		scanningInfo.ImageID = templateInfo.ImageID
		scanningInfo.SonarID = templateInfo.SonarID
		scanningInfo.Installs = templateInfo.Installs
		scanningInfo.Parameter = templateInfo.Parameter
		scanningInfo.Envs = templateInfo.Envs
		scanningInfo.Script = templateInfo.Script
		scanningInfo.AdvancedSetting = templateInfo.AdvancedSetting
		scanningInfo.CheckQualityGate = templateInfo.CheckQualityGate
	}

	scanningName := fmt.Sprintf("%s-%s-%s", scanningInfo.Name, id, "scanning-job")

	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.ScanningTaskFmt, scanningName))
	if err != nil {
		log.Errorf("failed to generated task id for scanning task, error: %s", err)
		return 0, e.ErrGetCounter.AddDesc(err.Error())
	}

	imageInfo, err := commonrepo.NewBasicImageColl().Find(scanningInfo.ImageID)
	if err != nil {
		log.Errorf("failed to get image information to create scanning task, error: %s", err)
		return 0, err
	}

	scanningImage := imageInfo.Value

	if imageInfo.ImageFrom == commonmodels.ImageFromKoderover {
		scanningImage = strings.ReplaceAll(config.ReaperImage(), "${BuildOS}", imageInfo.Value)
	}

	registries, err := commonservice.ListRegistryNamespaces("", true, log)
	if err != nil {
		log.Errorf("ListRegistryNamespaces err:%v", err)
		return 0, err
	}

	clusterInfo, err := commonrepo.NewK8SClusterColl().Get(scanningInfo.AdvancedSetting.ClusterID)
	if err != nil {
		return 0, e.ErrConvertSubTasks.AddErr(fmt.Errorf("failed to get cluster: %s, err: %s", scanningInfo.AdvancedSetting.ClusterID, err))
	}

	repos := make([]*types.Repository, 0)
	for _, arg := range req {
		rep, err := systemconfig.New().GetCodeHost(arg.CodehostID)
		if err != nil {
			log.Errorf("failed to get codehost info from mongodb, the error is: %s", err)
			return 0, err
		}

		repoInfo := &types.Repository{
			Source:             rep.Type,
			RepoOwner:          arg.RepoOwner,
			RepoName:           arg.RepoName,
			Branch:             arg.Branch,
			PR:                 arg.PR,
			PRs:                arg.PRs,
			CodehostID:         arg.CodehostID,
			OauthToken:         rep.AccessToken,
			Address:            rep.Address,
			Username:           rep.Username,
			Password:           rep.Password,
			EnableProxy:        rep.EnableProxy,
			RepoNamespace:      arg.RepoNamespace,
			Tag:                arg.Tag,
			AuthType:           rep.AuthType,
			SSHKey:             rep.SSHKey,
			PrivateAccessToken: rep.PrivateAccessToken,
		}

		for _, repo := range scanningInfo.Repos {
			// make sure we are using the same repo's configuration
			if repo.CodehostID == arg.CodehostID && repo.RepoName == arg.RepoName {
				repoInfo.SubModules = repo.SubModules
				repoInfo.RemoteName = repo.RemoteName
				repoInfo.CheckoutPath = repo.CheckoutPath
				break
			}
		}
		if repoInfo.PR > 0 && len(repoInfo.PRs) == 0 {
			repoInfo.PRs = []int{repoInfo.PR}
		}
		repoInfo.RepoNamespace = repoInfo.GetRepoNamespace()
		repos = append(repos, repoInfo)
	}

	// compatibility code
	cacheEnabled := false
	cacheDirType := types.WorkspaceCacheDir
	userCacheDir := ""
	if scanningInfo.AdvancedSetting.Cache != nil {
		cacheEnabled = scanningInfo.AdvancedSetting.Cache.CacheEnable
		cacheDirType = scanningInfo.AdvancedSetting.Cache.CacheDirType
		userCacheDir = scanningInfo.AdvancedSetting.Cache.CacheUserDir
	}

	scanningTask := &task.Scanning{
		TaskType:      config.TaskScanning,
		Status:        config.StatusCreated,
		ScanningID:    scanningInfo.ID.Hex(),
		Name:          scanningInfo.Name,
		EnableScanner: scanningInfo.EnableScanner,
		ImageInfo:     scanningImage,
		ResReq:        scanningInfo.AdvancedSetting.ResReq,
		ResReqSpec:    scanningInfo.AdvancedSetting.ResReqSpec,
		Registries:    registries,
		Parameter:     scanningInfo.Parameter,
		Envs:          scanningInfo.Envs,
		Script:        scanningInfo.Script,
		// the timeout we save is measured in minute
		Timeout:          scanningInfo.AdvancedSetting.Timeout * 60,
		ClusterID:        scanningInfo.AdvancedSetting.ClusterID,
		StrategyID:       scanningInfo.AdvancedSetting.StrategyID,
		Cache:            clusterInfo.Cache,
		CacheEnable:      cacheEnabled,
		CacheDirType:     cacheDirType,
		CacheUserDir:     userCacheDir,
		Repos:            repos,
		InstallItems:     scanningInfo.Installs,
		CheckQualityGate: scanningInfo.CheckQualityGate,
	}

	scanningTask.InstallCtx, err = workflowservice.BuildInstallCtx(scanningTask.InstallItems)
	if err != nil {
		log.Errorf("buildInstallCtx for scanning task error: %v", err)
		return 0, err
	}

	if scanningInfo.ScannerType == "sonarQube" {
		sonarInfo, err := commonrepo.NewSonarIntegrationColl().GetByID(context.TODO(), scanningInfo.SonarID)
		if err != nil {
			log.Errorf("failed to get sonar integration information to create scanning task, error: %s", err)
			return 0, err
		}

		scanningTask.SonarInfo = &commonmodels.SonarInfo{
			Token:         sonarInfo.Token,
			ServerAddress: sonarInfo.ServerAddress,
		}
	}

	proxies, err := commonrepo.NewProxyColl().List(&commonrepo.ProxyArgs{})
	if err != nil {
		log.Errorf("failed to get proxy info to create scanning task, error: %s", err)
		return 0, err
	}
	if len(proxies) != 0 {
		scanningTask.Proxy = proxies[0]
	}

	scanningSubtask, err := scanningTask.ToSubTask()
	if err != nil {
		log.Errorf("failed to convert scanning subtask, error: %s", err)
		return 0, e.ErrCreateTask.AddDesc(err.Error())
	}

	stages := make([]*commonmodels.Stage, 0)
	workflowservice.AddSubtaskToStage(&stages, scanningSubtask, scanningInfo.Name)
	sort.Sort(workflowservice.ByStageKind(stages))

	configPayload := commonservice.GetConfigPayload(0)

	defaultS3, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("cannot find the default s3 to store the logs, error: %s", err)
		return 0, e.ErrFindDefaultS3Storage.AddDesc("default storage is required by distribute task")
	}

	defaultURL, err := defaultS3.GetEncrypted()
	if err != nil {
		log.Errorf("cannot convert the s3 config to an encrypted URI, error: %s", err)
		return 0, e.ErrS3Storage.AddErr(err)
	}

	finalTask := &task.Task{
		TaskID:        nextTaskID,
		ProductName:   scanningInfo.ProjectName,
		PipelineName:  scanningName,
		Type:          config.ScanningType,
		Status:        config.StatusCreated,
		TaskCreator:   username,
		CreateTime:    time.Now().Unix(),
		Stages:        stages,
		ConfigPayload: configPayload,
		StorageURI:    defaultURL,
		ScanningArgs: &commonmodels.ScanningArgs{
			ScanningName:   scanningInfo.Name,
			ScanningID:     scanningInfo.ID.Hex(),
			NotificationID: notificationID,
		},
	}

	if len(finalTask.Stages) <= 0 {
		return 0, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	if err := workflowservice.CreateTask(finalTask); err != nil {
		log.Error(err)
		return 0, e.ErrCreateTask
	}

	// Updating the comment in the git repository, this will not cause the function to return error if this function call fails
	err = scmnotify.NewService().UpdateWebhookCommentForScanning(finalTask, log)
	if err != nil {
		log.Warnf("Failed to update comment for scanning: %s, the error is: %s", scanningInfo.ID.Hex(), err)
	}

	return nextTaskID, nil
}

// CreateScanningTaskV2 uses notificationID if the task is triggered by webhook, otherwise it should be empty
func CreateScanningTaskV2(id, username, account, userID string, req *CreateScanningTaskReq, notificationID string, log *zap.SugaredLogger) (int64, error) {
	scanningInfo, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return 0, err
	}

	scanningWorkflow, err := generateCustomWorkflowFromScanningModule(scanningInfo, req, notificationID, log)
	if err != nil {
		log.Errorf("failed to getenerate custom workflow from mongodb, the error is: %s", err)
		return 0, err
	}
	scanningWorkflow.HookPayload = req.HookPayload

	createResp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
		Name:    username,
		Account: account,
		UserID:  userID,
		Type:    config.WorkflowTaskTypeScanning,
	}, scanningWorkflow, log)

	if createResp != nil {
		return createResp.TaskID, err
	}

	return 0, err
}

func ListScanningTask(id string, pageNum, pageSize int, log *zap.SugaredLogger) (*ListScanningTaskResp, error) {
	scanningInfo, err := commonrepo.NewScanningColl().GetByID(id)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return nil, err
	}

	workflowName := commonutil.GenScanningWorkflowName(scanningInfo.ID.Hex())
	workflowTasks, total, err := commonrepo.NewworkflowTaskv4Coll().List(&commonrepo.ListWorkflowTaskV4Option{
		WorkflowName: workflowName,
		ProjectName:  scanningInfo.ProjectName,
		Skip:         (pageNum - 1) * pageSize,
		Limit:        pageSize,
	})

	if err != nil {
		log.Errorf("failed to find scanning module task of scanning: %s (common workflow name: %s), error: %s", scanningInfo.Name, workflowName, err)
		return nil, err
	}

	respList := make([]*ScanningTaskResp, 0)

	for _, workflowTask := range workflowTasks {
		taskInfo := &ScanningTaskResp{
			ScanID:    workflowTask.TaskID,
			Status:    string(workflowTask.Status),
			Creator:   workflowTask.TaskCreator,
			CreatedAt: workflowTask.CreateTime,
		}
		if workflowTask.Status == config.StatusPassed || workflowTask.Status == config.StatusCancelled || workflowTask.Status == config.StatusFailed {
			taskInfo.RunTime = workflowTask.EndTime - workflowTask.StartTime
		}
		respList = append(respList, taskInfo)
	}

	return &ListScanningTaskResp{
		ScanInfo: &ScanningInfo{
			Editor:    scanningInfo.UpdatedBy,
			UpdatedAt: scanningInfo.UpdatedAt,
		},
		ScanTasks:  respList,
		TotalTasks: total,
	}, nil
}

func GetScanningTaskInfo(scanningID string, taskID int64, log *zap.SugaredLogger) (*ScanningTaskDetail, error) {
	scanningInfo, err := commonrepo.NewScanningColl().GetByID(scanningID)
	if err != nil {
		log.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return nil, err
	}

	workflowName := commonutil.GenScanningWorkflowName(scanningInfo.ID.Hex())
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		log.Errorf("failed to find workflow task %d for scanning: %s, error: %s", taskID, scanningID, err)
		return nil, err
	}

	resultAddr := ""

	if len(workflowTask.Stages) != 1 || len(workflowTask.Stages[0].Jobs) != 1 {
		errMsg := fmt.Sprintf("invalid test task!")
		log.Errorf(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	spec := new(commonmodels.ZadigScanningJobSpec)
	err = commonmodels.IToi(workflowTask.WorkflowArgs.Stages[0].Jobs[0].Spec, spec)
	if err != nil {
		log.Errorf("failed to decode testing job spec, err: %s", err)
		return nil, err
	}

	if len(spec.Scannings) != 1 {
		log.Errorf("invalid scanning custom workflow scan list length: expect 1")
		return nil, fmt.Errorf("invalid scanning custom workflow scan list length: expect 1")
	}

	jobTaskSpec := new(commonmodels.JobTaskFreestyleSpec)
	err = commonmodels.IToi(workflowTask.Stages[0].Jobs[0].Spec, jobTaskSpec)
	if err != nil {
		log.Errorf("failed to decode scanning job spec, err: %s", err)
		return nil, err
	}

	jobName := ""
	isHasArtifact := false
	for _, step := range jobTaskSpec.Steps {
		if step.Name == config.TestJobArchiveResultStepName {
			if step.StepType != config.StepTarArchive {
				return nil, fmt.Errorf("step: %s was not a junit report step", step.Name)
			}
			if workflowTask.Stages[0].Jobs[0].Status == config.StatusPassed || workflowTask.Stages[0].Jobs[0].Status == config.StatusFailed {
				isHasArtifact = true
			}
			jobName = step.JobName
		}
	}

	sonarMetrics := &stepspec.SonarMetrics{}
	if scanningInfo.ScannerType == "sonarQube" {
		sonarInfo, err := commonrepo.NewSonarIntegrationColl().GetByID(context.TODO(), scanningInfo.SonarID)
		if err != nil {
			log.Errorf("failed to get sonar integration info, error: %s", err)
			return nil, err
		}

		sonarURL := sonarInfo.ServerAddress
		jobKey := workflowTask.Stages[0].Jobs[0].Key

		projectScanningOutputKey := jobspec.GetJobOutputKey(jobKey, setting.WorkflowScanningJobOutputKeyProject)
		projectScanningOutputKey = workflowcontroller.GetContextKey(projectScanningOutputKey)
		projectKey := workflowTask.GlobalContext[projectScanningOutputKey]

		branchScanningOutputKey := jobspec.GetJobOutputKey(jobKey, setting.WorkflowScanningJobOutputKeyBranch)
		branchScanningOutputKey = workflowcontroller.GetContextKey(branchScanningOutputKey)
		branch := workflowTask.GlobalContext[branchScanningOutputKey]

		resultAddr, err = sonar.GetSonarAddress(sonarURL, projectKey, branch)
		if err != nil {
			resultAddr = sonarURL
			log.Errorf("failed to get sonar address, project key: %s, branch: %s, error: %v", projectKey, branch, err)
		}

		for _, step := range jobTaskSpec.Steps {
			if step.StepType == config.StepSonarGetMetrics {
				stepSpec := &stepspec.StepSonarGetMetricsSpec{}
				commonmodels.IToi(step.Spec, &stepSpec)
				sonarMetrics = stepSpec.SonarMetrics
				break
			}
		}
	} else {
		sonarMetrics = nil
	}

	repoInfo := spec.Scannings[0].Repos
	// for security reasons, we set all sensitive information to empty
	for _, repo := range repoInfo {
		repo.OauthToken = ""
		repo.Password = ""
		repo.Username = ""
	}

	return &ScanningTaskDetail{
		Creator:       workflowTask.TaskCreator,
		Status:        string(workflowTask.Status),
		CreateTime:    workflowTask.CreateTime,
		EndTime:       workflowTask.EndTime,
		RepoInfo:      repoInfo,
		SonarMetrics:  sonarMetrics,
		ResultLink:    resultAddr,
		JobName:       jobName,
		IsHasArtifact: isHasArtifact,
	}, nil
}

func generateCustomWorkflowFromScanningModule(scanInfo *commonmodels.Scanning, args *CreateScanningTaskReq, notificationID string, log *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	concurrencyLimit := 1
	if scanInfo.AdvancedSetting != nil {
		concurrencyLimit = scanInfo.AdvancedSetting.ConcurrencyLimit
	}
	// compatibility code
	if concurrencyLimit == 0 {
		concurrencyLimit = -1
	}

	for _, notify := range scanInfo.AdvancedSetting.NotifyCtls {
		err := notify.GenerateNewNotifyConfigWithOldData()
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
	}

	resp := &commonmodels.WorkflowV4{
		Name:             commonutil.GenScanningWorkflowName(scanInfo.ID.Hex()),
		DisplayName:      scanInfo.Name,
		Stages:           nil,
		Project:          scanInfo.ProjectName,
		CreatedBy:        "system",
		ConcurrencyLimit: concurrencyLimit,
		NotificationID:   notificationID,
		NotifyCtls:       scanInfo.AdvancedSetting.NotifyCtls,
	}

	stage := make([]*commonmodels.WorkflowStage, 0)

	repos := make([]*types.Repository, 0)
	scanInfoRepoMap := make(map[string]*types.Repository)
	for _, repo := range scanInfo.Repos {
		scanInfoRepoMap[repo.GetKey()] = repo
	}

	for _, arg := range args.Repos {
		scanInfoRepo, ok := scanInfoRepoMap[arg.GetKey()]
		if !ok {
			log.Errorf("failed to find scanning repo info for codehost: %d", arg.CodehostID)
			return nil, fmt.Errorf("failed to find scanning repo info for codehost: %d", arg.CodehostID)
		}

		rep, err := systemconfig.New().GetCodeHost(arg.CodehostID)
		if err != nil {
			log.Errorf("failed to get codehost info from mongodb, the error is: %s", err)
			return nil, err
		}

		repos = append(repos, &types.Repository{
			Source:             rep.Type,
			RepoOwner:          arg.RepoOwner,
			RepoName:           arg.RepoName,
			Branch:             arg.Branch,
			PR:                 arg.PR,
			PRs:                arg.PRs,
			CodehostID:         arg.CodehostID,
			OauthToken:         rep.AccessToken,
			Address:            rep.Address,
			Username:           rep.Username,
			Password:           rep.Password,
			EnableProxy:        rep.EnableProxy,
			RepoNamespace:      arg.RepoNamespace,
			Tag:                arg.Tag,
			AuthType:           rep.AuthType,
			SSHKey:             rep.SSHKey,
			PrivateAccessToken: rep.PrivateAccessToken,
			DepotType:          arg.DepotType,
			Stream:             arg.Stream,
			ViewMapping:        arg.ViewMapping,
			ChangeListID:       arg.ChangeListID,
			ShelveID:           arg.ShelveID,
			RemoteName:         scanInfoRepo.RemoteName,
			CheckoutPath:       scanInfoRepo.CheckoutPath,
			SubModules:         scanInfoRepo.SubModules,
		})
	}

	kvs := args.KeyVals

	if scanInfo.TemplateID != "" {
		template, err := commonrepo.NewScanningTemplateColl().Find(&commonrepo.ScanningTemplateQueryOption{ID: scanInfo.TemplateID})
		if err != nil {
			log.Errorf("failed to get scanning template, id: %s, error: %s", scanInfo.TemplateID, err)
			return nil, err
		}

		kvs = commonservice.MergeBuildEnvs(template.Envs.ToRuntimeList(), scanInfo.Envs.ToRuntimeList()).ToKVList()
	}

	scan := &commonmodels.ScanningModule{
		Name:        scanInfo.Name,
		ProjectName: scanInfo.ProjectName,
		Repos:       repos,
		KeyVals:     renderKeyVals(args.KeyVals, kvs).ToRuntimeList(),
	}

	job := make([]*commonmodels.Job, 0)
	name := scanInfo.Name
	if len(name) >= 32 {
		name = strings.TrimSuffix(scanInfo.Name[:31], "-")
	}
	job = append(job, &commonmodels.Job{
		Name:    name,
		JobType: config.JobZadigScanning,
		Skipped: false,
		Spec: &commonmodels.ZadigScanningJobSpec{
			Scannings: []*commonmodels.ScanningModule{scan},
		},
	})

	stage = append(stage, &commonmodels.WorkflowStage{
		Name:     "scan",
		Parallel: false,
		Jobs:     job,
	})

	resp.Stages = stage

	return resp, nil
}

func renderKeyVals(input, origin commonmodels.KeyValList) commonmodels.KeyValList {
	for i, originKV := range origin {
		for _, inputKV := range input {
			if originKV.Key == inputKV.Key {
				// always use origin credential config.
				isCredential := originKV.IsCredential
				origin[i] = inputKV
				origin[i].IsCredential = isCredential
			}
		}
	}
	return origin
}
