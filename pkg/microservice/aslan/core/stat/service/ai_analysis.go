package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/pkg/tool/llm"
	"go.uber.org/zap"
)

func AiAnalysisProjectStats(args *AiAnalysisReq, logger *zap.SugaredLogger) (string, error) {
	ctx := context.Background()
	client, err := service.GetDefaultLLMClient(ctx)
	if err != nil {
		logger.Errorf("failed to get llm client, the error is: %+v", err)
		return "", err
	}

	// parse user prompt to prepare the input data of projects stat
	inputData, err := parseUserPrompt(args.Prompt, client)
	if err != nil {
		logger.Errorf("failed to parse user prompt, the error is: %+v", err)
		return "", err
	}

	// get analysis data from db with parameters projectList, jobList and timeRange
	data, err := GetStatsAnalysisData(inputData, logger)
	if err != nil {
		logger.Errorf("failed to get project stat data, the error is: %+v", err)
		return "", err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("failed to marshal data, the error is: %+v", err)
		return "", err
	}

	// the design of the prompt directly determines the quality of the answer
	prompt := fmt.Sprintf("%s \n %s", args.Prompt, string(dataBytes))
	answer, err := client.GetCompletion(ctx, prompt)
	if err != nil {
		logger.Errorf("failed to get answer from ai: %v, the error is: %+v", client.GetName(), err)
		return "", err
	}
	return answer, nil
}

// parseUserPrompt parse the user prompt to prepare the input data of projects stat
func parseUserPrompt(prompt string, aiClient llm.ILLM, logger *zap.SugaredLogger) (UserPromptParseInput, error) {
	input := UserPromptParseInput{}
	// get all project name from db
	projectList, err := template.NewProductColl().ListAllName()
	if err != nil {
		return input, err
	}

	jobs := []string{
		"build",
		"test",
		"deploy",
		"release",
	}

	prompt = fmt.Sprintf(fmt.Sprintf("%s \n %s", prompt, PROJECT_STAT_PROMPT), projectList, jobs)
	logger.Infof("the prompt is: %s", prompt)

	retry := 3
	// consider that change basic prompt to get the better parse result when the parse result is not valid
	for retry > 0 && !checkInputData(input) {
		resp, err := aiClient.GetCompletion(context.TODO(), prompt)
		logger.Infof("ai parse user prompt and the response is: %s", resp)
		if err != nil {
			return input, err
		}

		// parse the user prompt to prepare the input data of projects stat
		err = json.Unmarshal([]byte(resp), &input)
		if err != nil {
			return input, err
		}
		retry--
	}

	if !checkInputData(input) {
		return input, fmt.Errorf("failed to parse user prompt to get the valid input data for ai analysis")
	}

	if len(input.ProjectList) == 0 {
		input.ProjectList = projectList
	}
	if len(input.JobList) == 0 {
		input.JobList = jobs
	}
	return input, nil
}

// TODO: check the input data
func checkInputData(input UserPromptParseInput) bool {
	if input.StartTime == 0 || input.EndTime == 0 {
		return false
	}
	if input.StartTime > input.EndTime {
		return false
	}
	return true
}

func GetStatsAnalysisData(args UserPromptParseInput, logger *zap.SugaredLogger) (*AiReqData, error) {
	reqData := &AiReqData{
		StartTime:   args.StartTime,
		EndTime:     args.EndTime,
		ProjectList: make([]*ProjectData, 0),
	}
	for _, project := range args.ProjectList {
		logger.Infof("start to get ai analysis data from project %s", project)
		data := &ProjectData{
			ProjectName:       project,
			ProjectDataDetail: &DataDetail{},
		}

		for _, job := range args.JobList {
			switch job {
			case "build":
				build, err := getBuildData(project, args.StartTime, args.EndTime)
				if err != nil {
					logger.Errorf("failed to get build data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.BuildInfo = build
			case "test":
				test, err := getTestData(project, args.StartTime, args.EndTime)
				if err != nil {
					logger.Errorf("failed to get test data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.TestInfo = test
			case "deploy":
				deploy, err := getDeployData(project, args.StartTime, args.EndTime)
				if err != nil {
					logger.Errorf("failed to get deploy data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.DeployInfo = deploy
			case "release":
				release, err := getReleaseData(project, args.StartTime, args.EndTime)
				if err != nil {
					logger.Errorf("failed to get release data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.ReleaseInfo = release
			}
		}

		// get system evaluation data
		system, err := getSystemEvaluationData(project, args.StartTime, args.EndTime, logger)
		if err != nil {
			logger.Errorf("failed to get system evaluation data from project %s, the error is: %+v", project, err)
		}
		data.SystemInternalEvaluationResult = system

		reqData.ProjectList = append(reqData.ProjectList, data)
	}
	return reqData, nil
}

func getBuildData(project string, startTime, endTime int64) (*BuildData, error) {
	// get build data from mongo
	buildJobList, err := commonrepo.NewJobInfoColl().GetBuildJobs(startTime, endTime, project)
	if err != nil {
		return &BuildData{}, err
	}
	totalCounter := len(buildJobList)
	if totalCounter == 0 {
		return &BuildData{}, err
	}
	passCounter := 0
	for _, job := range buildJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	var totalTimesTaken int64 = 0
	for _, job := range buildJobList {
		totalTimesTaken += job.Duration
	}
	return &BuildData{
		BuildTotal:         totalCounter,
		BuildSuccessTotal:  passCounter,
		BuildFailureTotal:  totalCounter - passCounter,
		BuildTotalDuration: totalTimesTaken / int64(totalCounter),
	}, err
}

func getTestData(project string, startTime, endTime int64) (*TestData, error) {
	// get test data from mongo
	testJobList, err := commonrepo.NewJobInfoColl().GetTestJobs(startTime, endTime, project)
	if err != nil {
		return &TestData{}, err
	}
	totalCounter := len(testJobList)
	if totalCounter == 0 {
		return &TestData{}, err
	}
	passCounter := 0
	for _, job := range testJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	var totalTimesTaken int64 = 0
	for _, job := range testJobList {
		totalTimesTaken += job.Duration
	}
	return &TestData{
		TestTotal:         totalCounter,
		TestPass:          passCounter,
		TestFail:          totalCounter - passCounter,
		TestTotalDuration: totalTimesTaken / int64(totalCounter),
	}, nil
}

func getDeployData(project string, startTime, endTime int64) (*DeployData, error) {
	// get deploy data from mongo
	deployJobList, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime, endTime, project)
	if err != nil {
		return &DeployData{}, err
	}
	totalCounter := len(deployJobList)
	if totalCounter == 0 {
		return &DeployData{}, err
	}
	passCounter := 0
	for _, job := range deployJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	var totalTimesTaken int64 = 0
	for _, job := range deployJobList {
		totalTimesTaken += job.Duration
	}
	return &DeployData{
		DeployTotal:         totalCounter,
		DeploySuccessTotal:  passCounter,
		DeployFailureTotal:  totalCounter - passCounter,
		DeployTotalDuration: totalTimesTaken / int64(totalCounter),
	}, nil
}

func getReleaseData(project string, startTime, endTime int64) (*ReleaseData, error) {
	// get release data from mongo
	releaseJobList, err := commonrepo.NewJobInfoColl().GetProductionDeployJobs(startTime, endTime, project)
	if err != nil {
		return &ReleaseData{}, err
	}
	totalCounter := len(releaseJobList)
	if totalCounter == 0 {
		return &ReleaseData{}, err
	}
	passCounter := 0
	for _, job := range releaseJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	var totalTimesTaken int64 = 0
	for _, job := range releaseJobList {
		totalTimesTaken += job.Duration
	}
	return &ReleaseData{
		ReleaseTotal:         totalCounter,
		ReleaseSuccessTotal:  passCounter,
		ReleaseFailureTotal:  totalCounter - passCounter,
		ReleaseTotalDuration: totalTimesTaken / int64(totalCounter),
	}, nil
}

func getSystemEvaluationData(project string, startTime, endTime int64, logger *zap.SugaredLogger) (string, error) {
	result, err := GetStatsDashboard(startTime, endTime, []string{project}, logger)
	if err != nil {
		return "", err
	}
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(jsonResult), nil
}
