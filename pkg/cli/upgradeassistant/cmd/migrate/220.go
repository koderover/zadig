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

package migrate

import (
	"context"
	"fmt"
	"strings"

	internalmongodb "github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util/boolptr"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func init() {
	upgradepath.RegisterHandler("2.1.0", "2.2.0", V210ToV220)
	upgradepath.RegisterHandler("2.2.0", "2.1.0", V220ToV210)
}

func V210ToV220() error {
	log.Infof("-------- start migrate predeploy to build --------")
	err := migratePreDeployToBuild()
	if err != nil {
		log.Errorf("migratePreDeployToBuild error: %s", err)
		return err
	}

	log.Infof("-------- start migrate product workflow to custom workflow --------")
	err = migrateProductWorkflowToCustomWorkflow()
	if err != nil {
		log.Errorf("migrate product workflow to custom workflow error: %s", err)
		return err
	}

	return nil
}

func V220ToV210() error {
	return nil
}

func migratePreDeployToBuild() error {
	cursor, err := mongodb.NewBuildColl().ListByCursor(&mongodb.BuildListOption{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var build models.Build
		if err := cursor.Decode(&build); err != nil {
			return err
		}

		if build.PreDeploy == nil {
			build.PreDeploy = &models.PreDeploy{}
			build.PreDeploy.BuildOS = build.PreBuild.BuildOS
			build.PreDeploy.ImageFrom = build.PreBuild.ImageFrom
			build.PreDeploy.ImageID = build.PreBuild.ImageID
			build.PreDeploy.Installs = build.PreBuild.Installs

			build.DeployInfrastructure = build.Infrastructure
			build.DeployVMLabels = build.VMLabels
			build.DeployRepos = build.Repos

			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", build.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"pre_deploy", build.PreDeploy},
							{"deploy_infrastructure", build.DeployInfrastructure},
							{"deploy_vm_labels", build.DeployVMLabels},
							{"deploy_repos", build.DeployRepos},
						}},
					}),
			)
			log.Infof("add build %s", build.Name)
		}

		if len(ms) >= 50 {
			log.Infof("update %d build", len(ms))
			if _, err := mongodb.NewBuildColl().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("update builds error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d build", len(ms))
		if _, err := mongodb.NewBuildColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate builds error: %s", err)
		}
	}

	return nil
}

func migrateProductWorkflowToCustomWorkflow() error {
	logger := log.SugaredLogger()
	productWorkflows, err := mongodb.NewWorkflowColl().List(&mongodb.ListWorkflowOption{})
	if err != nil {
		logger.Errorf("failed to list product workflow from db, error: %s", err)
		return err
	}

	for _, wf := range productWorkflows {
		logger.Infof("migrating product workflow: %s to custom workflow.......", wf.Name)
		newWorkflow, err := generateCustomWorkflowFromProductWorkflow(wf)
		if err != nil {
			logger.Errorf("failed to generate custom workflow for product workflow: %s, error: %s", wf.Name, err)
			continue
		}

		err = workflow.CreateWorkflowV4("system", newWorkflow, log.SugaredLogger())
		if err != nil {
			logger.Errorf("failed to create custom workflow for product workflow: %s, error: %s", wf.DisplayName, err)
			return err
		}

		if wf.HookCtl != nil && len(wf.HookCtl.Items) > 0 {
			presetInfo, err := workflow.GetWebhookForWorkflowV4Preset(newWorkflow.Name, "", "", logger)
			if err != nil {
				logger.Errorf("failed to generate workflow preset for custom workflow: %s, error: %s", newWorkflow.Name, err)
				return err
			}

			for i, hook := range wf.HookCtl.Items {
				for _, stage := range presetInfo.WorkflowArg.Stages {
					if stage.Name == "构建" {
						// if the workflow has a build stage in it
						buildJobSpec := new(models.ZadigBuildJobSpec)
						err = models.IToi(stage.Jobs[0].Spec, buildJobSpec)
						if err != nil {
							logger.Errorf("failed to decode workflow spec, error: %s", err)
							return err
						}
						targets := make([]*models.ServiceAndBuild, 0)
						for _, buildTarget := range hook.WorkflowArgs.Target {
							for _, svc := range buildJobSpec.ServiceAndBuilds {
								if svc.ServiceName == buildTarget.ServiceName && svc.ServiceModule == buildTarget.Name {
									// set repos
									targets = append(targets, svc)
								}
							}
						}
						buildJobSpec.ServiceAndBuilds = targets

						stage.Jobs[0].Spec = buildJobSpec
					}

					if stage.Name == "部署" {
						// if the workflow has a deploy stage in it
						switch stage.Jobs[0].JobType {
						case config.JobZadigVMDeploy:
							buildJobSpec := new(models.ZadigVMDeployJobSpec)
							err = models.IToi(stage.Jobs[0].Spec, buildJobSpec)
							if err != nil {
								logger.Errorf("failed to decode workflow spec, error: %s", err)
								return err
							}

							if hook.WorkflowArgs.EnvUpdatePolicy == "all" {
								buildJobSpec.Env = hook.WorkflowArgs.Namespace
							} else {
								envs := strings.Split(hook.WorkflowArgs.Namespace, ",")
								buildJobSpec.Env = envs[0]
							}

							stage.Jobs[0].Spec = buildJobSpec
						case config.JobZadigDeploy:
							buildJobSpec := new(models.ZadigDeployJobSpec)
							err = models.IToi(stage.Jobs[0].Spec, buildJobSpec)
							if err != nil {
								logger.Errorf("failed to decode workflow spec, error: %s", err)
								return err
							}

							if hook.WorkflowArgs.EnvUpdatePolicy == "all" {
								buildJobSpec.Env = hook.WorkflowArgs.Namespace
							} else {
								envs := strings.Split(hook.WorkflowArgs.Namespace, ",")
								buildJobSpec.Env = envs[0]
							}
							stage.Jobs[0].Spec = buildJobSpec
						}

					}
				}

				newWebhook := &models.WorkflowV4Hook{
					Name:        fmt.Sprintf("hook-%d", i),
					AutoCancel:  hook.AutoCancel,
					Enabled:     wf.HookCtl.Enabled,
					MainRepo:    hook.MainRepo,
					Description: "",
					Repos:       nil,
					WorkflowArg: presetInfo.WorkflowArg,
				}

				err = workflow.CreateWebhookForWorkflowV4(newWorkflow.Name, newWebhook, logger)
				if err != nil {
					logger.Errorf("failed to create workflow for workflow: %s, error: %s", newWorkflow.Name, err)
					return err
				}
			}
		}

		if wf.ScheduleEnabled {
			crons, err := mongodb.NewCronjobColl().List(&mongodb.ListCronjobParam{
				ParentName: wf.Name,
				ParentType: setting.WorkflowCronjob,
			})
			if err != nil {
				logger.Errorf("failed to find job for workflow: %s, error: %s", wf.Name, err)
				return err
			}
			for i, cron := range crons {
				logger.Infof("creating cron for workflow: %s", wf.Name)
				cronJobPreset, err := workflow.GetCronForWorkflowV4Preset(newWorkflow.Name, "", "", logger)
				if err != nil {
					logger.Errorf("failed to generate workflow preset for custom workflow %s, error: %s")
					return err
				}
				for _, stage := range cronJobPreset.WorkflowV4Args.Stages {
					if stage.Name == "构建" {
						// if the workflow has a build stage in it
						buildJobSpec := new(models.ZadigBuildJobSpec)
						err = models.IToi(stage.Jobs[0].Spec, buildJobSpec)
						if err != nil {
							logger.Errorf("failed to decode workflow spec, error: %s", err)
							return err
						}
						targets := make([]*models.ServiceAndBuild, 0)
						for _, buildTarget := range cron.WorkflowArgs.Target {
							for _, svc := range buildJobSpec.ServiceAndBuilds {
								if svc.ServiceName == buildTarget.ServiceName && svc.ServiceModule == buildTarget.Name {
									// set repos
									svc.Repos = buildTarget.Build.Repos
									targets = append(targets, svc)
								}
							}
						}
						buildJobSpec.ServiceAndBuilds = targets

						stage.Jobs[0].Spec = buildJobSpec
					}

					if stage.Name == "部署" {
						// if the workflow has a deploy stage in it
						switch stage.Jobs[0].JobType {
						case config.JobZadigVMDeploy:
							buildJobSpec := new(models.ZadigVMDeployJobSpec)
							err = models.IToi(stage.Jobs[0].Spec, buildJobSpec)
							if err != nil {
								logger.Errorf("failed to decode workflow spec, error: %s", err)
								return err
							}

							buildJobSpec.Env = cron.WorkflowArgs.Namespace
							stage.Jobs[0].Spec = buildJobSpec
						case config.JobZadigDeploy:
							buildJobSpec := new(models.ZadigDeployJobSpec)
							err = models.IToi(stage.Jobs[0].Spec, buildJobSpec)
							if err != nil {
								logger.Errorf("failed to decode workflow spec, error: %s", err)
								return err
							}

							buildJobSpec.Env = cron.WorkflowArgs.Namespace
							stage.Jobs[0].Spec = buildJobSpec
						}

					}

					if stage.Name == "测试" {
						testJobSpec := new(models.ZadigTestingJobSpec)
						err = models.IToi(stage.Jobs[0].Spec, testJobSpec)
						if err != nil {
							logger.Errorf("failed to decode workflow spec, error: %s", err)
							return err
						}

						for _, test := range cron.WorkflowArgs.Tests {
							for _, testing := range testJobSpec.TestModules {
								if test.Namespace == testing.ProjectName && test.TestModuleName == testing.Name {
									testing.Repos = test.Builds
								}
							}
						}

						stage.Jobs[0].Spec = testJobSpec
					}
				}

				newCron := &models.Cronjob{
					Name:           fmt.Sprintf("cron-%d", i),
					Type:           "workflow_v4",
					Number:         cron.Number,
					Frequency:      cron.Frequency,
					Time:           cron.Time,
					Cron:           cron.Cron,
					ProductName:    wf.ProductTmplName,
					MaxFailure:     cron.MaxFailure,
					WorkflowV4Args: cronJobPreset.WorkflowV4Args,
					JobType:        cron.JobType,
					Enabled:        true,
				}

				err = workflow.CreateCronForWorkflowV4(newWorkflow.Name, newCron, logger)
				if err != nil {
					logger.Errorf("failed to create cron for workflow: %s, error: %s", newWorkflow.Name, err)
					return err
				}
			}
		}

		// when all the creation process are done, remove all the timer and cron in the product workflow.
		if wf.HookCtl != nil && wf.HookCtl.Enabled {
			wf.HookCtl.Enabled = false
		}
		if wf.ScheduleEnabled {
			wf.ScheduleEnabled = false
		}

		err = workflow.UpdateWorkflow(wf, logger)
		if err != nil {
			logger.Errorf("failed to disable product workflow [%s]'s cron scheduler and webhooks, error: %s", wf.Name, err)
			return err
		}

		logger.Infof("product workflow: %s migration done.", wf.Name)
	}

	return nil
}

const (
	customWorkflowNamingConvention = "z-wf-%s"
	customWorkflowDescription      = "原工作流： 产品工作流 [%s], 显示名称 [%s]"
)

func generateCustomWorkflowFromProductWorkflow(productWorkflow *models.Workflow) (*models.WorkflowV4, error) {
	if productWorkflow == nil {
		return nil, fmt.Errorf("empty workflow...")
	}
	// there should be no concurrency limit on product workflow
	concurrencyLimit := 1
	if productWorkflow.IsParallel {
		concurrencyLimit = -1
	}

	project, err := templaterepo.NewProductColl().Find(productWorkflow.ProductTmplName)
	if err != nil {
		return nil, fmt.Errorf("failed to find project %s, err: %v", productWorkflow.ProductTmplName, err)
	}

	resp := &models.WorkflowV4{
		Name:             fmt.Sprintf(customWorkflowNamingConvention, productWorkflow.Name),
		DisplayName:      productWorkflow.DisplayName,
		Project:          productWorkflow.ProductTmplName,
		CreatedBy:        "system",
		ConcurrencyLimit: concurrencyLimit,
		NotifyCtls:       productWorkflow.NotifyCtls,
		Description:      fmt.Sprintf(customWorkflowDescription, productWorkflow.Name, productWorkflow.DisplayName),
	}

	stages := make([]*models.WorkflowStage, 0)

	defaultObjectStorage, err := mongodb.NewS3StorageColl().FindDefault()
	if err != nil {
		return nil, err
	}

	// create the stages based on their priority, first would be the build, which comes with a default deploy stage
	if productWorkflow.BuildStage != nil && productWorkflow.BuildStage.Enabled {
		buildStage := &models.WorkflowStage{
			Name:     "构建",
			Parallel: true,
		}

		envName := productWorkflow.EnvName
		if envName == "" {
			envs, err := service.ListProducts("system", productWorkflow.ProductTmplName, []string{}, false, log.SugaredLogger())
			if err != nil {
				return nil, err
			}
			if len(envs) > 0 {
				envName = envs[0].Name
			}
		}

		env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
			Name:       productWorkflow.ProductTmplName,
			EnvName:    envName,
			Production: boolptr.False(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to find env %s/%s, err: %v", productWorkflow.ProductTmplName, envName, err)
		}

		// use env registry
		registry, err := mongodb.NewRegistryNamespaceColl().Find(&mongodb.FindRegOps{IsDefault: true})
		if err != nil {
			return nil, err
		}
		if env.RegistryID != "" {
			registry, err = mongodb.NewRegistryNamespaceColl().Find(&mongodb.FindRegOps{ID: env.RegistryID})
			if err != nil {
				return nil, err
			}
		}

		serviceAndBuilds := make([]*models.ServiceAndBuild, 0)
		for _, module := range productWorkflow.BuildStage.Modules {
			// if a module is hidden, we don't add it to the service and modules
			if !module.HideServiceModule {
				repos := make([]*types.Repository, 0)
				for _, item := range module.BranchFilter {
					ch, err := internalmongodb.NewCodehostColl().GetByID(item.CodehostID)
					if err != nil {
						log.Errorf("failed to get codehost of id: %d, error: %s", item.CodehostID, err)
						return nil, err
					}
					repos = append(repos, &types.Repository{
						Source:        ch.Type,
						CodehostID:    item.CodehostID,
						Branch:        item.DefaultBranch,
						RepoName:      item.RepoName,
						RepoNamespace: item.RepoNamespace,
						RepoOwner:     item.RepoOwner,
					})
				}
				serviceAndBuilds = append(serviceAndBuilds, &models.ServiceAndBuild{
					ServiceName:   module.Target.ServiceName,
					ServiceModule: module.Target.ServiceModule,
					BuildName:     module.Target.BuildName,
					ImageName:     module.Target.ServiceModule,
					KeyVals:       module.Target.Envs.ToRuntimeList(),
					Repos:         repos,
				})
			}
		}

		jobs := make([]*models.Job, 0)
		jobs = append(jobs, &models.Job{
			Name:    "build",
			JobType: config.JobZadigBuild,
			Skipped: false,
			Spec: &models.ZadigBuildJobSpec{
				DockerRegistryID: registry.ID.Hex(),
				ServiceAndBuilds: serviceAndBuilds,
			},
		})

		buildStage.Jobs = jobs
		stages = append(stages, buildStage)

		deployStage := &models.WorkflowStage{
			Name:     "部署",
			Parallel: true,
		}

		deployJobs := make([]*models.Job, 0)
		if project.ProductFeature == nil {
			return nil, fmt.Errorf("product feature cannot be nil")
		}
		if project.ProductFeature.BasicFacility == "kubernetes" {
			spec := &models.ZadigDeployJobSpec{
				Env:    envName,
				Source: config.SourceFromJob,
				DeployContents: []config.DeployContent{
					config.DeployImage,
				},
				JobName:    "build",
				DeployType: project.ProductFeature.DeployType,
			}

			deployJobs = append(deployJobs, &models.Job{
				Name:    "deploy",
				JobType: config.JobZadigDeploy,
				Spec:    spec,
			})
		} else if project.ProductFeature.BasicFacility == "cloud_host" {
			spec := &models.ZadigVMDeployJobSpec{
				Env:         envName,
				S3StorageID: defaultObjectStorage.ID.Hex(),
				Source:      config.SourceFromJob,
				JobName:     "build",
			}

			deployJobs = append(deployJobs, &models.Job{
				Name:    "deploy",
				JobType: config.JobZadigVMDeploy,
				Spec:    spec,
			})
		}

		deployStage.Jobs = deployJobs
		stages = append(stages, deployStage)

		if productWorkflow.DistributeStage != nil && productWorkflow.DistributeStage.Enabled {
			distributeJobs := make([]*models.Job, 0)
			deployAfterDistributeJobs := make([]*models.Job, 0)

			// the distribute stage will only be supported after build & deploy stage
			for i, distribute := range productWorkflow.DistributeStage.Releases {
				distributeSpec := &models.ZadigDistributeImageJobSpec{
					Source:                   config.SourceFromJob,
					JobName:                  "build",
					TargetRegistryID:         distribute.RepoID,
					Timeout:                  10,
					EnableTargetImageTagRule: false,
				}

				distributeJobs = append(distributeJobs, &models.Job{
					Name:    fmt.Sprintf("distribute-%d", i),
					JobType: config.JobZadigDistributeImage,
					Spec:    distributeSpec,
				})

				if distribute.DeployEnabled {
					deploySpec := &models.ZadigDeployJobSpec{
						Env:    distribute.DeployEnv,
						Source: config.SourceFromJob,
						DeployContents: []config.DeployContent{
							config.DeployImage,
						},
						JobName: fmt.Sprintf("distribute-%d", i),
					}

					if project.ProductFeature != nil {
						deploySpec.DeployType = project.ProductFeature.DeployType
					}
					deployAfterDistributeJobs = append(deployAfterDistributeJobs, &models.Job{
						Name:    fmt.Sprintf("deploy-%d", i),
						JobType: config.JobZadigDeploy,
						Spec:    deploySpec,
					})
				}
			}

			// after all the distribute stage is taken care of, add them to the workflow if there actually exists some work to do.
			if len(distributeJobs) > 0 {
				stages = append(stages, &models.WorkflowStage{
					Name:     "分发",
					Parallel: true,
					Jobs:     distributeJobs,
				})
			}

			if len(deployAfterDistributeJobs) > 0 {
				stages = append(stages, &models.WorkflowStage{
					Name:     "分发部署",
					Parallel: true,
					Jobs:     deployAfterDistributeJobs,
				})
			}
		}
	} else if productWorkflow.ArtifactStage != nil && productWorkflow.ArtifactStage.Enabled {
		// build and artifact stage is mutually exclusive
		deployStage := &models.WorkflowStage{
			Name:     "部署",
			Parallel: true,
		}

		deployJobs := make([]*models.Job, 0)
		if project.ProductFeature == nil {
			return nil, fmt.Errorf("product feature cannot be nil")
		}
		if project.ProductFeature.BasicFacility == "kubernetes" {
			serviceAndImages := make([]*models.ServiceAndImage, 0)

			for _, module := range productWorkflow.ArtifactStage.Modules {
				if !module.HideServiceModule {
					serviceAndImages = append(serviceAndImages, &models.ServiceAndImage{
						ServiceName:   module.Target.ServiceName,
						ServiceModule: module.Target.ServiceModule,
					})
				}
			}

			spec := &models.ZadigDeployJobSpec{
				Env:    productWorkflow.EnvName,
				Source: config.SourceRuntime,
				DeployContents: []config.DeployContent{
					config.DeployImage,
				},
				DefaultServices: serviceAndImages,
				DeployType:      project.ProductFeature.DeployType,
			}

			if spec.Env == "" {
				envs, err := service.ListProducts("system", productWorkflow.ProductTmplName, []string{}, false, log.SugaredLogger())
				if err != nil {
					return nil, err
				}
				if len(envs) > 0 {
					spec.Env = envs[0].Name
				}
			}

			deployJobs = append(deployJobs, &models.Job{
				Name:    "deploy",
				JobType: config.JobZadigDeploy,
				Spec:    spec,
			})
		} else if project.ProductFeature.BasicFacility == "cloud_host" {
			serviceAndImages := make([]*models.ServiceAndVMDeploy, 0)

			for _, module := range productWorkflow.ArtifactStage.Modules {
				if !module.HideServiceModule {
					serviceAndImages = append(serviceAndImages, &models.ServiceAndVMDeploy{
						ServiceName:   module.Target.ServiceName,
						ServiceModule: module.Target.ServiceModule,
					})
				}
			}

			spec := &models.ZadigVMDeployJobSpec{
				Env:                 productWorkflow.EnvName,
				S3StorageID:         defaultObjectStorage.ID.Hex(),
				Source:              config.SourceRuntime,
				JobName:             "build",
				ServiceAndVMDeploys: serviceAndImages,
			}

			if spec.Env == "" {
				envs, err := service.ListProducts("system", productWorkflow.ProductTmplName, []string{}, false, log.SugaredLogger())
				if err != nil {
					return nil, err
				}
				if len(envs) > 0 {
					spec.Env = envs[0].Name
				}
			}

			deployJobs = append(deployJobs, &models.Job{
				Name:    "deploy",
				JobType: config.JobZadigVMDeploy,
				Spec:    spec,
			})
		}

		deployStage.Jobs = deployJobs
		stages = append(stages, deployStage)
	}

	if productWorkflow.TestStage != nil && productWorkflow.TestStage.Enabled {
		testStage := &models.WorkflowStage{
			Name:     "测试",
			Parallel: true,
		}

		testJobs := make([]*models.Job, 0)
		testModules := make([]*models.TestModule, 0)

		for _, test := range productWorkflow.TestStage.Tests {
			testModules = append(testModules, &models.TestModule{
				Name:        test.Name,
				ProjectName: test.Project,
				KeyVals:     test.Envs.ToRuntimeList(),
			})
		}
		spec := &models.ZadigTestingJobSpec{
			TestType:      "",
			Source:        config.SourceRuntime,
			JobName:       "",
			OriginJobName: "",
			TestModules:   testModules,
		}

		testJobs = append(testJobs, &models.Job{
			Name:    "test",
			JobType: config.JobZadigTesting,
			Spec:    spec,
		})

		testStage.Jobs = testJobs
		stages = append(stages, testStage)
	}

	resp.Stages = stages
	resp.NotifyCtls = productWorkflow.NotifyCtls

	return resp, nil
}
