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

package job

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	codehostdb "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type VMDeployJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ZadigVMDeployJobSpec
}

func (j *VMDeployJob) Instantiate() error {
	j.spec = &commonmodels.ZadigVMDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *VMDeployJob) SetPreset() error {
	j.spec = &commonmodels.ZadigVMDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	var err error
	_, err = templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}
	// if quoted job quote another job, then use the service and artifact of the quoted job
	if j.spec.Source == config.SourceFromJob {
		j.spec.OriginJobName = j.spec.JobName
		j.spec.JobName = getOriginJobName(j.workflow, j.spec.JobName)
	} else if j.spec.Source == config.SourceRuntime {
		envName := strings.ReplaceAll(j.spec.Env, setting.FixedValueMark, "")
		_, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: envName})
		if err != nil {
			log.Errorf("can't find product %s in env %s, error: %w", j.workflow.Project, envName, err)
			return nil
		}
	}

	for _, serviceAndVMDeploy := range j.spec.ServiceAndVMDeploys {
		templateSvc, err := commonrepo.NewServiceColl().Find(
			&commonrepo.ServiceFindOption{
				ServiceName: serviceAndVMDeploy.ServiceName,
				ProductName: j.workflow.Project,
			},
		)
		if err != nil {
			err = fmt.Errorf("can't find service %s in project %s, error: %v", serviceAndVMDeploy.ServiceName, j.workflow.Project, err)
			log.Error(err)
			return fmt.Errorf(err.Error())
		}
		if templateSvc.BuildName == "" {
			err = fmt.Errorf("service %s in project %s has no deploy", serviceAndVMDeploy.ServiceName, j.workflow.Project)
			log.Error(err)
			return fmt.Errorf(err.Error())
		}
		build, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: templateSvc.BuildName, ProductName: j.workflow.Project})
		if err != nil {
			err = fmt.Errorf("can't find build %s in project %s, error: %v", templateSvc.BuildName, j.workflow.Project, err)
			log.Error(err)
			return fmt.Errorf(err.Error())
		}
		serviceAndVMDeploy.Repos = mergeRepos(serviceAndVMDeploy.Repos, build.DeployRepos)
	}

	return nil
}

func (j *VMDeployJob) GetRepos() ([]*types.Repository, error) {
	resp := []*types.Repository{}
	j.spec = &commonmodels.ZadigVMDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	for _, serviceAndVMDeploy := range j.spec.ServiceAndVMDeploys {
		templateSvc, err := commonrepo.NewServiceColl().Find(
			&commonrepo.ServiceFindOption{
				ServiceName: serviceAndVMDeploy.ServiceName,
				ProductName: j.workflow.Project,
			},
		)
		if err != nil {
			err = fmt.Errorf("can't find service %s in project %s, error: %v", serviceAndVMDeploy.ServiceName, j.workflow.Project, err)
			log.Error(err)
			return nil, fmt.Errorf(err.Error())
		}
		if templateSvc.BuildName == "" {
			err = fmt.Errorf("service %s in project %s has no deploy", serviceAndVMDeploy.ServiceName, j.workflow.Project)
			log.Error(err)
			return nil, fmt.Errorf(err.Error())
		}
		build, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: templateSvc.BuildName, ProductName: j.workflow.Project})
		if err != nil {
			err = fmt.Errorf("can't find build %s in project %s, error: %v", templateSvc.BuildName, j.workflow.Project, err)
			log.Error(err)
			return nil, fmt.Errorf(err.Error())
		}
		repos := mergeRepos(serviceAndVMDeploy.Repos, build.DeployRepos)
		resp = append(resp, repos...)
	}
	return resp, nil
}

func (j *VMDeployJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.ZadigVMDeployJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.ZadigVMDeployJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}

		newDeploys := []*commonmodels.ServiceAndVMDeploy{}
		for _, deploy := range j.spec.ServiceAndVMDeploys {
			for _, argsDeploy := range argsSpec.ServiceAndVMDeploys {
				if deploy.ServiceName == argsDeploy.ServiceName && deploy.ServiceModule == argsDeploy.ServiceModule {
					deploy.Repos = mergeRepos(deploy.Repos, argsDeploy.Repos)
					newDeploys = append(newDeploys, deploy)
					break
				}
			}
		}
		j.spec.ServiceAndVMDeploys = newDeploys

		j.job.Spec = j.spec
	}
	return nil
}

func (j *VMDeployJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.ZadigVMDeployJobSpec{}

	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	envName := strings.ReplaceAll(j.spec.Env, setting.FixedValueMark, "")
	_, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: envName})
	if err != nil {
		return resp, fmt.Errorf("env %s not exists", envName)
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	vms, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		return resp, fmt.Errorf("list private keys error: %v", err)
	}

	services, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("list project %s's services error: %v", j.workflow.Project, err)
	}
	serviceMap := map[string]*commonmodels.Service{}
	for _, service := range services {
		serviceMap[service.ServiceName] = service
	}

	var s3Storage *commonmodels.S3Storage
	// get deploy info from previous build job
	if j.spec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.spec.OriginJobName != "" {
			j.spec.JobName = j.spec.OriginJobName
		}
		targets, err := j.getOriginReferedJobTargets(j.spec.JobName, int(taskID))
		if err != nil {
			return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", j.spec.JobName, err)
		}

		s3Storage, err = commonrepo.NewS3StorageColl().FindDefault()
		if err != nil {
			return resp, fmt.Errorf("find default s3 storage error: %v", err)
		}
		// clear service and image list to prevent old data from remaining
		j.spec.ServiceAndVMDeploys = targets
		j.spec.S3StorageID = s3Storage.ID.Hex()
	} else {
		s3Storage, err = commonrepo.NewS3StorageColl().Find(j.spec.S3StorageID)
		if err != nil {
			return resp, fmt.Errorf("find s3 storage id: %s, error: %v", j.spec.S3StorageID, err)
		}
	}

	for _, vmDeployInfo := range j.spec.ServiceAndVMDeploys {
		service, ok := serviceMap[vmDeployInfo.ServiceName]
		if !ok {
			return resp, fmt.Errorf("service %s not found", vmDeployInfo.ServiceName)
		}
		buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: service.BuildName, ProductName: j.workflow.Project})
		if err != nil {
			return resp, fmt.Errorf("find build: %s error: %v", service.BuildName, err)
		}
		// it only fills build detail created from template
		if err := fillBuildDetail(buildInfo, vmDeployInfo.ServiceName, vmDeployInfo.ServiceName); err != nil {
			return resp, err
		}
		basicImage, err := commonrepo.NewBasicImageColl().Find(buildInfo.PreDeploy.ImageID)
		if err != nil {
			return resp, fmt.Errorf("find base image: %s error: %v", buildInfo.PreBuild.ImageID, err)
		}

		jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
		jobTask := &commonmodels.JobTask{
			Name: jobNameFormat(vmDeployInfo.ServiceName + "-" + vmDeployInfo.ServiceModule + "-" + j.job.Name),
			JobInfo: map[string]string{
				"service_name":   vmDeployInfo.ServiceName,
				"service_module": vmDeployInfo.ServiceModule,
				JobNameKey:       j.job.Name,
			},
			Key:            strings.Join([]string{j.job.Name, vmDeployInfo.ServiceName, vmDeployInfo.ServiceModule}, "."),
			JobType:        string(config.JobZadigVMDeploy),
			Spec:           jobTaskSpec,
			Timeout:        int64(buildInfo.Timeout),
			Infrastructure: buildInfo.DeployInfrastructure,
			VMLabels:       buildInfo.DeployVMLabels,
		}
		jobTaskSpec.Properties = commonmodels.JobProperties{
			Timeout:         int64(timeout),
			ResourceRequest: buildInfo.PreBuild.ResReq,
			ResReqSpec:      buildInfo.PreBuild.ResReqSpec,
			CustomEnvs:      buildInfo.PreBuild.Envs,
			ClusterID:       buildInfo.PreBuild.ClusterID,
			StrategyID:      buildInfo.PreBuild.StrategyID,
			BuildOS:         basicImage.Value,
			ImageFrom:       buildInfo.PreDeploy.ImageFrom,
		}

		initShellScripts := []string{}
		vmDeployVars := []*commonmodels.KeyVal{}
		tmpVmDeployVars := getVMDeployJobVariables(vmDeployInfo, buildInfo, taskID, j.spec.Env, j.workflow.Project, j.workflow.Name, j.workflow.DisplayName, jobTask.Infrastructure, vms, services, log.SugaredLogger())
		for _, kv := range tmpVmDeployVars {
			if strings.HasSuffix(kv.Key, "_PK_CONTENT") {
				name := strings.TrimSuffix(kv.Key, "_PK_CONTENT")
				vmDeployVars = append(vmDeployVars, &commonmodels.KeyVal{Key: name + "_PK", Value: "/tmp/" + name + "_PK", IsCredential: false})

				initShellScripts = append(initShellScripts, "echo \""+kv.Value+"\" > /tmp/"+name+"_PK")
				initShellScripts = append(initShellScripts, "chmod 600 /tmp/"+name+"_PK")
			} else {
				vmDeployVars = append(vmDeployVars, kv)
			}
		}

		jobTaskSpec.Properties.Envs = append(jobTaskSpec.Properties.CustomEnvs, vmDeployVars...)
		jobTaskSpec.Properties.UseHostDockerDaemon = buildInfo.PreBuild.UseHostDockerDaemon

		if jobTask.Infrastructure == setting.JobVMInfrastructure {
			jobTaskSpec.Properties.CacheEnable = buildInfo.CacheEnable
			jobTaskSpec.Properties.CacheDirType = buildInfo.CacheDirType
			jobTaskSpec.Properties.CacheUserDir = buildInfo.CacheUserDir
		} else {
			clusterInfo, err := commonrepo.NewK8SClusterColl().Get(buildInfo.PreBuild.ClusterID)
			if err != nil {
				return resp, fmt.Errorf("find cluster: %s error: %v", buildInfo.PreBuild.ClusterID, err)
			}

			if clusterInfo.Cache.MediumType == "" {
				jobTaskSpec.Properties.CacheEnable = false
			} else {
				jobTaskSpec.Properties.Cache = clusterInfo.Cache
				jobTaskSpec.Properties.CacheEnable = buildInfo.CacheEnable
				jobTaskSpec.Properties.CacheDirType = buildInfo.CacheDirType
				jobTaskSpec.Properties.CacheUserDir = buildInfo.CacheUserDir
			}

			if jobTaskSpec.Properties.CacheEnable && jobTaskSpec.Properties.Cache.MediumType == types.NFSMedium {
				jobTaskSpec.Properties.CacheUserDir = renderEnv(jobTaskSpec.Properties.CacheUserDir, jobTaskSpec.Properties.Envs)
				jobTaskSpec.Properties.Cache.NFSProperties.Subpath = renderEnv(jobTaskSpec.Properties.Cache.NFSProperties.Subpath, jobTaskSpec.Properties.Envs)
			}
		}

		// init tools install step
		tools := []*step.Tool{}
		for _, tool := range buildInfo.PreDeploy.Installs {
			tools = append(tools, &step.Tool{
				Name:    tool.Name,
				Version: tool.Version,
			})
		}
		toolInstallStep := &commonmodels.StepTask{
			Name:     fmt.Sprintf("%s-%s", vmDeployInfo.ServiceName, "tool-install"),
			JobName:  jobTask.Name,
			StepType: config.StepTools,
			Spec:     step.StepToolInstallSpec{Installs: tools},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, toolInstallStep)
		// init git clone step
		gitStep := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-git",
			JobName:  jobTask.Name,
			StepType: config.StepGit,
			Spec:     step.StepGitSpec{Repos: vmRenderRepos(buildInfo.DeployRepos, jobTaskSpec.Properties.Envs)},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, gitStep)

		if vmDeployInfo.WorkflowType == config.WorkflowType {
			if s3Storage.Subfolder != "" {
				s3Storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", s3Storage.Subfolder, vmDeployInfo.WorkflowName, vmDeployInfo.TaskID, "file")
			} else {
				s3Storage.Subfolder = fmt.Sprintf("%s/%d/%s", vmDeployInfo.WorkflowName, vmDeployInfo.TaskID, "file")
			}
		} else if vmDeployInfo.WorkflowType == config.WorkflowTypeV4 {
			log.Debugf("s3Storage.Subfolder: %s", s3Storage.Subfolder)
			if s3Storage.Subfolder != "" {
				log.Debugf("1")
				s3Storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s/%s", s3Storage.Subfolder, vmDeployInfo.WorkflowName, vmDeployInfo.TaskID, vmDeployInfo.JobTaskName, "archive")
			} else {
				log.Debugf("2")
				s3Storage.Subfolder = fmt.Sprintf("%s/%d/%s/%s", vmDeployInfo.WorkflowName, vmDeployInfo.TaskID, vmDeployInfo.JobTaskName, "archive")
			}
			log.Debugf("after s3Storage.Subfolder: %s", s3Storage.Subfolder)
		} else {
			return resp, fmt.Errorf("unknown workflow type %s", vmDeployInfo.WorkflowType)
		}
		log.Debugf("vmDeployInfo.FileName: %s", vmDeployInfo.FileName)
		// init download artifact step
		downloadArtifactStep := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-download-artifact",
			JobName:  jobTask.Name,
			StepType: config.StepDownloadArtifact,
			Spec: step.StepDownloadArtifactSpec{
				Artifact: vmDeployInfo.FileName,
				S3:       modelS3toS3(s3Storage),
			},
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, downloadArtifactStep)
		// init debug before step
		debugBeforeStep := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-debug_before",
			JobName:  jobTask.Name,
			StepType: config.StepDebugBefore,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugBeforeStep)
		// init shell step
		scripts := append([]string{}, initShellScripts...)
		scripts = append(scripts, strings.Split(replaceWrapLine(buildInfo.PMDeployScripts), "\n")...)
		scriptStep := &commonmodels.StepTask{
			JobName: jobTask.Name,
		}
		if buildInfo.ScriptType == types.ScriptTypeShell || buildInfo.ScriptType == "" {
			scriptStep.Name = vmDeployInfo.ServiceName + "-shell"
			scriptStep.StepType = config.StepShell
			scriptStep.Spec = &step.StepShellSpec{
				Scripts: scripts,
			}
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, scriptStep)
		// init debug after step
		debugAfterStep := &commonmodels.StepTask{
			Name:     vmDeployInfo.ServiceName + "-debug_after",
			JobName:  jobTask.Name,
			StepType: config.StepDebugAfter,
		}
		jobTaskSpec.Steps = append(jobTaskSpec.Steps, debugAfterStep)

		resp = append(resp, jobTask)
	}

	j.job.Spec = j.spec
	return resp, nil
}

func (j *VMDeployJob) LintJob() error {
	j.spec = &commonmodels.ZadigVMDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	if j.spec.Source != config.SourceFromJob {
		return nil
	}
	jobRankMap := getJobRankMap(j.workflow.Stages)
	buildJobRank, ok := jobRankMap[j.spec.JobName]
	if !ok || buildJobRank >= jobRankMap[j.job.Name] {
		return fmt.Errorf("can not quote job %s in job %s", j.spec.JobName, j.job.Name)
	}
	return nil
}

func (j *VMDeployJob) GetOutPuts(log *zap.SugaredLogger) []string {
	return getOutputKey(j.job.Name, ensureDeployInOutputs())
}

func (j *VMDeployJob) getOriginReferedJobTargets(jobName string, taskID int) ([]*commonmodels.ServiceAndVMDeploy, error) {
	serviceAndVMDeploys := []*commonmodels.ServiceAndVMDeploy{}
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != j.spec.JobName {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return serviceAndVMDeploys, err
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					serviceAndVMDeploys = append(serviceAndVMDeploys, &commonmodels.ServiceAndVMDeploy{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
						FileName:      build.Package,
						TaskID:        taskID,
						WorkflowName:  j.workflow.Name,
						WorkflowType:  config.WorkflowTypeV4,
						JobTaskName:   jobNameFormat(build.ServiceName + "-" + build.ServiceModule + "-" + jobName),
					})
					log.Infof("DeployJob ToJobs getOriginReferedJobTargets: workflow %s service %s, module %s, image %s",
						j.workflow.Name, build.ServiceName, build.ServiceModule, build.Image)
				}
				return serviceAndVMDeploys, nil
			}
		}
	}
	return nil, fmt.Errorf("build job %s not found", jobName)
}

func getVMDeployJobVariables(vmDeploy *commonmodels.ServiceAndVMDeploy, buildInfo *commonmodels.Build, taskID int64, envName, project, workflowName, workflowDisplayName, infrastructure string, vms []*commonmodels.PrivateKey, services []*commonmodels.Service, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	// basic envs
	ret = append(ret, PrepareDefaultWorkflowTaskEnvs(project, workflowName, workflowDisplayName, infrastructure, taskID)...)

	// repo envs
	ret = append(ret, getReposVariables(buildInfo.Repos)...)

	// vm deploy specific envs
	ret = append(ret, &commonmodels.KeyVal{Key: "ENV_NAME", Value: envName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE", Value: vmDeploy.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_NAME", Value: vmDeploy.ServiceName, IsCredential: false})
	ret = append(ret, &commonmodels.KeyVal{Key: "SERVICE_MODULE", Value: vmDeploy.ServiceModule, IsCredential: false})

	privateKeys := sets.String{}
	IDvmMap := map[string]*commonmodels.PrivateKey{}
	labelVMsMap := map[string][]*commonmodels.PrivateKey{}
	for _, vm := range vms {
		privateKeys.Insert(vm.Name)
		IDvmMap[vm.ID.Hex()] = vm
		labelVMsMap[vm.Label] = append(labelVMsMap[vm.Label], vm)
	}
	ret = append(ret, &commonmodels.KeyVal{Key: "AGENTS", Value: strings.Join(privateKeys.List(), ","), IsCredential: false})

	agentVMIDs := sets.String{}
	if len(buildInfo.SSHs) > 0 {
		// privateKeys := make([]*taskmodels.SSH, 0)
		for _, sshID := range buildInfo.SSHs {
			//私钥信息可能被更新，而构建中存储的信息是旧的，需要根据id获取最新的私钥信息
			latestKeyInfo, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{ID: sshID})
			if err != nil || latestKeyInfo == nil {
				log.Errorf("PrivateKey.Find failed, id:%s, err:%s", sshID, err)
				continue
			}
			agentName := latestKeyInfo.Name
			userName := latestKeyInfo.UserName
			ip := latestKeyInfo.IP
			port := latestKeyInfo.Port
			if port == 0 {
				port = setting.PMHostDefaultPort
			}
			privateKey, err := base64.StdEncoding.DecodeString(latestKeyInfo.PrivateKey)
			if err != nil {
				log.Errorf("base64 decode failed ip:%s, error:%s", ip, err)
				continue
			}
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_PK_CONTENT", Value: string(privateKey), IsCredential: false})
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_USERNAME", Value: userName, IsCredential: false})
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_IP", Value: ip, IsCredential: false})
			ret = append(ret, &commonmodels.KeyVal{Key: agentName + "_PORT", Value: strconv.Itoa(int(port)), IsCredential: false})
			agentVMIDs.Insert(sshID)
		}
	}

	envHostNamesMap := map[string][]string{}
	envHostIPsMap := map[string][]string{}
	addedHostIDs := sets.String{}
	for _, svc := range services {
		if svc.ServiceName != vmDeploy.ServiceName {
			continue
		}
		for _, envConfig := range svc.EnvConfigs {
			for _, hostID := range envConfig.HostIDs {
				if agentVMIDs.Has(hostID) || addedHostIDs.Has(hostID) {
					continue
				}
				if vm, ok := IDvmMap[hostID]; ok {
					addedHostIDs.Insert(hostID)
					envHostNamesMap[envConfig.EnvName] = append(envHostNamesMap[envConfig.EnvName], vm.Name)
					envHostIPsMap[envConfig.EnvName] = append(envHostIPsMap[envConfig.EnvName], vm.IP)

					hostName := vm.Name
					userName := vm.UserName
					ip := vm.IP
					port := vm.Port
					if port == 0 {
						port = setting.PMHostDefaultPort
					}
					privateKey, err := base64.StdEncoding.DecodeString(vm.PrivateKey)
					if err != nil {
						log.Errorf("base64 decode failed ip:%s, error:%s", ip, err)
						continue
					}
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PK_CONTENT", Value: string(privateKey), IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_USERNAME", Value: userName, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_IP", Value: ip, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PORT", Value: strconv.Itoa(int(port)), IsCredential: false})
				}
			}
			for _, label := range envConfig.Labels {
				for _, vm := range labelVMsMap[label] {
					if agentVMIDs.Has(vm.ID.Hex()) || addedHostIDs.Has(vm.ID.Hex()) {
						continue
					}
					addedHostIDs.Insert(vm.ID.Hex())
					envHostNamesMap[envConfig.EnvName] = append(envHostNamesMap[envConfig.EnvName], vm.Name)
					envHostIPsMap[envConfig.EnvName] = append(envHostIPsMap[envConfig.EnvName], vm.IP)

					hostName := vm.Name
					userName := vm.UserName
					ip := vm.IP
					port := vm.Port
					if port == 0 {
						port = setting.PMHostDefaultPort
					}
					privateKey := vm.PrivateKey
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PK_CONTENT", Value: privateKey, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_USERNAME", Value: userName, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_IP", Value: ip, IsCredential: false})
					ret = append(ret, &commonmodels.KeyVal{Key: hostName + "_PORT", Value: strconv.Itoa(int(port)), IsCredential: false})
				}
			}
		}
	}
	// env host ips
	for envName, HostIPs := range envHostIPsMap {
		ret = append(ret, &commonmodels.KeyVal{Key: envName + "_HOST_IPs", Value: strings.Join(HostIPs, ","), IsCredential: false})
	}
	// env host names
	for envName, names := range envHostNamesMap {
		ret = append(ret, &commonmodels.KeyVal{Key: envName + "_HOST_NAMEs", Value: strings.Join(names, ","), IsCredential: false})
	}

	if infrastructure != setting.JobVMInfrastructure {
		ret = append(ret, &commonmodels.KeyVal{Key: "ARTIFACT", Value: "/workspace/artifact/" + vmDeploy.FileName, IsCredential: false})
	} else {
		ret = append(ret, &commonmodels.KeyVal{Key: "ARTIFACT", Value: "$WORKSPACE/artifact/" + vmDeploy.FileName, IsCredential: false})
	}
	ret = append(ret, &commonmodels.KeyVal{Key: "PKG_FILE", Value: vmDeploy.FileName, IsCredential: false})
	return ret
}

func vmRenderRepos(repos []*types.Repository, kvs []*commonmodels.KeyVal) []*types.Repository {
	for _, inputRepo := range repos {
		inputRepo.CheckoutPath = renderEnv(inputRepo.CheckoutPath, kvs)
		if inputRepo.RemoteName == "" {
			inputRepo.RemoteName = "origin"
		}
		if inputRepo.Source == types.ProviderOther {
			codeHostInfo, err := codehostdb.NewCodehostColl().GetCodeHostByID(inputRepo.CodehostID, false)
			if err == nil {
				inputRepo.PrivateAccessToken = codeHostInfo.PrivateAccessToken
				inputRepo.SSHKey = codeHostInfo.SSHKey
			}
		}
	}
	return repos
}
