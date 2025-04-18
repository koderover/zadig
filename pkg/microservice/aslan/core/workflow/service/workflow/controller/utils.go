package controller

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	"go.uber.org/zap"

	"strings"
)

func renderMultiLineString(body string, inputs []*commonmodels.Param) string {
	for _, input := range inputs {
		var inputValue string
		if input.ParamsType == string(commonmodels.MultiSelectType) {
			inputValue = strings.Join(input.ChoiceValue, ",")
		} else {
			inputValue = input.Value
		}
		inputValue = strings.ReplaceAll(inputValue, "\n", "\\n")
		body = strings.ReplaceAll(body, fmt.Sprintf(setting.RenderValueTemplate, input.Name), inputValue)
	}
	return body
}

type RepoIndex struct {
	JobName       string `json:"job_name"`
	ServiceName   string `json:"service_name"`
	ServiceModule string `json:"service_module"`
	RepoIndex     int    `json:"repo_index"`
}

// GetWorkflowRepoIndex TODO: FIX THIS LATER THIS IS WEIRD AF
func GetWorkflowRepoIndex(workflow *commonmodels.WorkflowV4, currentJobName string, log *zap.SugaredLogger) []*RepoIndex {
	resp := make([]*RepoIndex, 0)
	buildService := commonservice.NewBuildService()
	jobRankMap := job.GetJobRankMap(workflow.Stages)
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			// we only need to get the outputs from job runs before the current job
			if jobRankMap[job.Name] >= jobRankMap[currentJobName] {
				return resp
			}
			if job.JobType == config.JobZadigBuild {
				jobSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, jobSpec); err != nil {
					log.Errorf("get job spec failed, err: %v", err)
					continue
				}
				for _, build := range jobSpec.ServiceAndBuildsOptions {
					buildInfo, err := buildService.GetBuild(build.BuildName, build.ServiceName, build.ServiceModule)
					if err != nil {
						log.Errorf("get build info failed, err: %v", err)
						continue
					}
					for _, target := range buildInfo.Targets {
						if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
							repos := applyRepos(buildInfo.Repos, build.Repos)
							for index := range repos {
								resp = append(resp, &RepoIndex{
									JobName:       job.Name,
									ServiceName:   build.ServiceName,
									ServiceModule: build.ServiceModule,
									RepoIndex:     index,
								})
							}
							break
						}
					}
				}
			}
		}
	}
	return resp
}

func applyRepos(base, input []*types.Repository) []*types.Repository {
	resp := make([]*types.Repository, 0)
	customRepoMap := make(map[string]*types.Repository)
	for _, repo := range input {
		if repo.RepoNamespace == "" {
			repo.RepoNamespace = repo.RepoOwner
		}
		customRepoMap[repo.GetKey()] = repo
	}
	for _, repo := range base {
		item := new(types.Repository)
		_ = util.DeepCopy(item, repo)
		if item.RepoNamespace == "" {
			item.RepoNamespace = item.RepoOwner
		}
		// user can only set default branch in custom workflow.
		if cv, ok := customRepoMap[repo.GetKey()]; ok {
			item.Branch = cv.Branch
			item.Tag = cv.Tag
			item.PR = cv.PR
			item.PRs = cv.PRs
			item.FilterRegexp = cv.FilterRegexp
		}

		resp = append(resp, item)
	}
	return resp
}
