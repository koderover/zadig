package workflow

import (
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	jobctl "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow/job"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// CreateCustomWorkflowTask creates a task for custom workflow with user-friendly inputs, this is currently
// used for openAPI
func CreateCustomWorkflowTask(username string, args *OpenAPICreateCustomWorkflowTaskArgs, log *zap.SugaredLogger) (*CreateTaskV4Resp, error) {
	// first we generate a detailed workflow.
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(args.WorkflowName)
	if err != nil {
		log.Errorf("cannot find workflow %s, the error is: %v", args.WorkflowName, err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}

	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if err := jobctl.SetPreset(job, workflow); err != nil {
				log.Errorf("cannot get workflow %s preset, the error is: %v", args.WorkflowName, err)
				return nil, e.ErrFindWorkflow.AddDesc(err.Error())
			}
		}
	}

	if err := fillWorkflowV4(workflow, log); err != nil {
		return nil, err
	}

	inputMap := make(map[string]interface{})
	for _, input := range args.Inputs {
		inputMap[input.JobName] = input.Parameters
	}

	for _, stage := range workflow.Stages {
		jobList := make([]*commonmodels.Job, 0)
		for _, job := range stage.Jobs {
			// if a job is found, add it to the job creation list
			if inputParam, ok := inputMap[job.Name]; ok {
				updater, err := getInputUpdater(job, inputParam)
				if err != nil {
					return nil, err
				}
				newJob, err := updater.UpdateJobSpec(job)
				if err != nil {
					log.Errorf("Failed to update jobspec for job: %s, error: %s", job.Name, err)
					return nil, errors.New("failed to update jobspec")
				}
				jobList = append(jobList, newJob)
			}
		}
		stage.Jobs = jobList
	}

	return CreateWorkflowTaskV4(&CreateWorkflowTaskV4Args{
		Name: username,
	}, workflow, log)
}

func CreateWorkflowViewOpenAPI(name, projectName string, workflowList []*commonmodels.WorkflowViewDetail, username string, logger *zap.SugaredLogger) error {
	// the list we got in openAPI is slightly different from the normal version, adding the missing field for workflowList
	for _, workflowInfo := range workflowList {
		workflowInfo.Enabled = true
	}

	return CreateWorkflowView(name, projectName, workflowList, username, logger)
}

func fillWorkflowV4(workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobZadigBuild {
				spec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				for _, build := range spec.ServiceAndBuilds {
					buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
					if err != nil {
						logger.Errorf(err.Error())
						return e.ErrFindWorkflow.AddErr(err)
					}
					kvs := buildInfo.PreBuild.Envs
					if buildInfo.TemplateID != "" {
						templateEnvs := []*commonmodels.KeyVal{}
						buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
							ID: buildInfo.TemplateID,
						})
						// if template not found, envs are empty, but do not block user.
						if err != nil {
							logger.Error("build job: %s, template not found", buildInfo.Name)
						} else {
							templateEnvs = buildTemplate.PreBuild.Envs
						}

						for _, target := range buildInfo.Targets {
							if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
								kvs = target.Envs
							}
						}
						// if build template update any keyvals, merge it.
						kvs = commonservice.MergeBuildEnvs(templateEnvs, kvs)
					}
					build.KeyVals = commonservice.MergeBuildEnvs(kvs, build.KeyVals)
				}
				job.Spec = spec
			}
			if job.JobType == config.JobFreestyle {
				spec := &commonmodels.FreestyleJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				job.Spec = spec
			}
			if job.JobType == config.JobPlugin {
				spec := &commonmodels.PluginJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				job.Spec = spec
			}
		}
	}
	return nil
}

func OpenAPICreateProductWorkflowTask(username string, args *OpenAPICreateProductWorkflowTaskArgs, logger *zap.SugaredLogger) (*CreateTaskResp, error) {
	// first get the preset info from the workflow itself
	createArgs, err := PresetWorkflowArgs(args.Input.TargetEnv, args.WorkflowName, logger)
	if err != nil {
		return nil, err
	}

	// if build is enabled, we change the information in the target section
	if args.Input.BuildArgs.Enabled {
		targetList := make([]*commonmodels.TargetArgs, 0)
		for _, targetInfo := range createArgs.Target {
			for _, inputTarget := range args.Input.BuildArgs.ServiceList {
				// if the 2 are the same
				if targetInfo.Name == inputTarget.ServiceModule && targetInfo.ServiceName == inputTarget.ServiceName {
					// update build repo info with input build info
					for _, inputRepo := range inputTarget.RepoInfo {
						repoInfo, err := mongodb.NewCodehostColl().GetCodeHostByAlias(inputRepo.CodeHostName)
						if err != nil {
							return nil, errors.New("failed to find code host with name:" + inputRepo.CodeHostName)
						}

						for _, buildRepo := range targetInfo.Build.Repos {
							if buildRepo.CodehostID == repoInfo.ID {
								if buildRepo.RepoNamespace == inputRepo.RepoNamespace && buildRepo.RepoName == inputRepo.RepoName {
									buildRepo.Branch = inputRepo.Branch
									buildRepo.PR = inputRepo.PR
									buildRepo.PRs = inputRepo.PRs
								}
							}
						}
					}

					// update the build kv
					kvMap := make(map[string]string)
					for _, kv := range inputTarget.Inputs {
						kvMap[kv.Key] = kv.Value
					}

					for _, buildParam := range targetInfo.Envs {
						if val, ok := kvMap[buildParam.Key]; ok {
							buildParam.Value = val
						}
					}

					targetList = append(targetList, targetInfo)
				}
			}
		}

		// if it has a build stage and does not have a deployment stage, we simply remove all the deployment info in the parameter
		if !args.Input.DeployArgs.Enabled {
			for _, target := range targetList {
				target.Deploy = make([]commonmodels.DeployEnv, 0)
			}
		} else if args.Input.DeployArgs.Source != "zadig" {
			return nil, fmt.Errorf("deploy source must be zadig when there is a build stage")
		}

		createArgs.Target = targetList
	} else if args.Input.DeployArgs.Enabled {
		// otherwise if only deploy is enabled
		targetList := make([]*commonmodels.TargetArgs, 0)
		deployList := make([]*commonmodels.ArtifactArgs, 0)
		for _, target := range createArgs.Target {
			for _, deployTarget := range args.Input.DeployArgs.ServiceList {
				if deployTarget.ServiceModule == target.Name && deployTarget.ServiceName == target.ServiceName {
					deployList = append(deployList, &commonmodels.ArtifactArgs{
						Name:        deployTarget.ServiceModule,
						ImageName:   deployTarget.ServiceModule,
						ServiceName: deployTarget.ServiceName,
						Image:       deployTarget.ImageName,
						Deploy:      target.Deploy,
					})
				}
			}
		}
		createArgs.Target = targetList
		createArgs.Artifact = deployList
	}

	return CreateWorkflowTask(createArgs, username, logger)
}

func getInputUpdater(job *commonmodels.Job, input interface{}) (CustomJobInput, error) {
	switch job.JobType {
	case config.JobPlugin:
		updater := new(PluginJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobFreestyle:
		updater := new(FreestyleJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigBuild:
		updater := new(ZadigBuildJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigDeploy:
		updater := new(ZadigDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sBlueGreenDeploy:
		updater := new(BlueGreenDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sCanaryDeploy:
		updater := new(CanaryDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobCustomDeploy:
		updater := new(CustomDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sBlueGreenRelease, config.JobK8sCanaryRelease:
		updater := new(EmptyInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigTesting:
		updater := new(ZadigTestingJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sGrayRelease:
		updater := new(GrayReleaseJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sGrayRollback:
		updater := new(GrayRollbackJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sPatch:
		updater := new(K8sPatchJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigScanning:
		updater := new(ZadigScanningJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	default:
		return nil, errors.New("undefined job type of type:" + string(job.JobType))
	}
}
