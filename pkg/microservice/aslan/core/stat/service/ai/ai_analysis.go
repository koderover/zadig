package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/util"
	"go.uber.org/zap"
	"gorm.io/gorm/utils"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	"github.com/koderover/zadig/pkg/tool/llm"
)

type AiAnalysisResp struct {
	Answer               string `json:"answer"`
	InputData            string `json:"input_data"`
	OverallProjectPrompt string `json:"overall_project_prompt"`
	TokenNum             int    `json:"token_num"`
}

type analysisAnswer struct {
	answer map[string]string
	m      *sync.Mutex
}

func AnalyzeProjectStats(args *AiAnalysisReq, logger *zap.SugaredLogger) (*AiAnalysisResp, error) {
	client, err := service.GetDefaultLLMClient(context.TODO())
	if err != nil {
		logger.Errorf("failed to get llm client, the error is: %+v", err)
		return nil, err
	}

	// parse user prompt to prepare the input data of projects stat
	inputData, err := parseUserPrompt(args, client, logger)
	if err != nil {
		logger.Errorf("failed to parse user prompt, the error is: %+v", err)
		return nil, err
	}
	if len(args.ProjectList) > 0 {
		inputData.ProjectList = args.ProjectList
	}

	// get analysis data from db with parameters projectList, jobList and timeRange
	data, err := GetStatsAnalysisData(inputData, logger)
	if err != nil {
		logger.Errorf("failed to get project stat data, the error is: %+v", err)
		return nil, err
	}

	// if data token over 3500, it will be split into multiple requests
	promptInput, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("failed to marshal data, the error is: %+v", err)
		return nil, err
	}
	prompt := fmt.Sprintf("假设你是资深Devops专家，你需要根据分析要求和目标去分析三重引号分割的项目数据，分析要求和目标：%s, 你的回答需要使用text格式输出, 输出内容不要包含\"三重引号分割的项目数据\"这个名称，也不要复述分析要求中的内容; 项目数据：\"\"\"%s\"\"\";", args.Prompt, string(promptInput))
	start := time.Now()
	tokenNum, err := llm.NumTokensFromPrompt(prompt, "")
	if err != nil {
		logger.Errorf("failed to get token num from prompt, the error is: %+v", err)
		return nil, err
	}
	logger.Infof("=====> Finished Request AI in AnalyzeProjectStats method for token num：%d,  Duration: %.2f seconds\n", tokenNum, time.Since(start).Seconds())

	ans := &analysisAnswer{
		answer: make(map[string]string, 0),
		m:      &sync.Mutex{},
	}
	var overAllInput string
	if tokenNum > 3500 {
		wg := &sync.WaitGroup{}
		// There is a problem: if each project is analyzed separately, the prompt can only be designed by oneself. The last time a user defined prompt is used, it will result in inaccurate results
		for _, project := range data.ProjectList {
			wg.Add(1)
			go AnalyzeProject(args.Prompt, project, client, ans, wg, logger)
		}
		wg.Wait()

		overAllInput, err = combineAiAnswer(data.ProjectList, ans.answer, data.StartTime, data.EndTime)
		if err != nil {
			logger.Errorf("failed to combine ai answer, the error is: %+v", err)
			return nil, err
		}
	}

	// the design of the prompt directly determines the quality of the answer
	if tokenNum > 3500 {
		prompt = fmt.Sprintf("假设你是Devops专家，需要你根据分析要求分析三重引号分割的项目分析结果，分析要求:%s;你的回答需要使用text格式输出,输出内容不要包含\"三重引号分割的项目数据\"这个名称,也不要复述分析要求中的内容; 项目分析结果：\"\"\"%s\"\"\"", args.Prompt, overAllInput)
	}
	start = time.Now()
	answer, err := client.GetCompletion(context.TODO(), util.RemoveExtraSpaces(prompt), llm.WithTemperature(float32(0.2)))
	if err != nil {
		logger.Errorf("failed to get answer from ai: %v, the error is: %+v", client.GetName(), err)
		return nil, err
	}
	logger.Infof("=====> Finished Request AI in AnalyzeProjectStats method,  Duration: %.2f seconds\n", time.Since(start).Seconds())

	return &AiAnalysisResp{
		Answer:               answer,
		InputData:            overAllInput,
		OverallProjectPrompt: prompt,
		TokenNum:             tokenNum,
	}, nil
}

func AnalyzeProject(userPrompt string, project *ProjectData, client llm.ILLM, ans *analysisAnswer, wg *sync.WaitGroup, logger *zap.SugaredLogger) {
	defer wg.Done()

	pData, err := json.Marshal(project)
	if err != nil {
		// there is no need to return error, just log it
		logger.Errorf("failed to marshal project data, the error is: %+v", err)
		return
	}

	prompt := fmt.Sprintf("假设你是资深Devops专家，我需要你根据以下分析要求来分析用三重引号分割的项目数据，最后根据你的分析来生成分析报告，分析要求：%s； 项目数据：\"\"\"%s\"\"\";你的回答不能超过400个汉字，同时回答内容要符合text格式，不要存在换行和空行;", util.RemoveExtraSpaces(EveryProjectAnalysisPrompt), string(pData))
	start := time.Now()
	answer, err := client.GetCompletion(context.TODO(), util.RemoveExtraSpaces(prompt), llm.WithTemperature(float32(0.1)))
	if err != nil {
		logger.Errorf("failed to get answer from ai: %v, the error is: %+v", client.GetName(), err)
		return
	}
	logger.Infof("=====> Finished Request AI in AnalyzeProject method,  Duration: %.2f seconds\n; the answer is: \n%s\n", time.Since(start).Seconds(), answer)

	ans.m.Lock()
	ans.answer[project.ProjectName] = answer
	ans.m.Unlock()
}

type Answer2input struct {
	ProjectName            string `json:"project_name"`
	ProjectDataStartTime   int64  `json:"project_data_start_time"`
	ProjectDataEndTime     int64  `json:"project_data_end_time"`
	AIAnalyzeProjectResult string `json:"ai_analyze_project_result"`
}

func combineAiAnswer(projects []*ProjectData, ans map[string]string, startTime, endTime int64) (string, error) {
	data := make([]*Answer2input, 0)
	for _, project := range projects {
		data = append(data, &Answer2input{
			ProjectName:            project.ProjectName,
			ProjectDataStartTime:   startTime,
			ProjectDataEndTime:     endTime,
			AIAnalyzeProjectResult: ans[project.ProjectName],
		})
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(dataBytes), nil
}

// parseUserPrompt parse the user prompt to prepare the input data of projects stat
func parseUserPrompt(args *AiAnalysisReq, aiClient llm.ILLM, logger *zap.SugaredLogger) (*UserPromptParseInput, error) {
	input := &UserPromptParseInput{}
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

	if len(args.ProjectList) > 0 && args.StartTime != 0 && args.EndTime != 0 {
		input.ProjectList = args.ProjectList
		input.start = args.StartTime
		input.end = args.EndTime
		input.JobList = jobs
		return input, nil
	}
	prompt := fmt.Sprintf("%s {%s}", util.RemoveExtraSpaces(ProjectStatPrompt), args.Prompt)
	if args.TestPrompt != "" {
		prompt = fmt.Sprintf("%s {%s}", args.TestPrompt, args.Prompt)
	}

	retry := 1
	// consider that change basic prompt to get the better parse result when the parse result is not valid
	for retry > 0 {
		start := time.Now()
		resp, err := aiClient.GetCompletion(context.TODO(), prompt)
		logger.Infof("=====> Finished Request AI in parseUserPrompt method,  Duration: %.2f seconds\n; the response is: \n%s\n", time.Since(start).Seconds(), resp)
		if err != nil {
			return input, err
		}

		// parse the user prompt to prepare the input data of projects stat
		err = json.Unmarshal([]byte(resp), &input)
		if err != nil {
			return input, err
		}

		if ok, in := checkInputData(input); ok {
			input = in
			break
		}
		retry--
	}

	if ok, _ := checkInputData(input); !ok {
		if args.StartTime > 0 && args.EndTime > 0 && args.EndTime > args.StartTime {
			input.start = args.StartTime
			input.end = args.EndTime
		} else {
			return input, fmt.Errorf("failed to parse user prompt to get the valid input data for ai analysis")
		}
	}

	if len(input.ProjectList) == 0 {
		input.ProjectList = projectList
	}
	if args.StartTime > 0 && args.EndTime > 0 && args.EndTime > args.StartTime {
		input.start = args.StartTime
		input.end = args.EndTime
	}

	jobList := make([]string, 0)
	if len(input.JobList) > 0 {
		for _, job := range input.JobList {
			if utils.Contains(jobs, job) {
				jobList = append(jobList, job)
			}
		}
	}
	input.JobList = jobs
	if len(jobList) != 0 {
		input.JobList = jobList
	}
	return input, nil
}

// TODO: check the input data
func checkInputData(input *UserPromptParseInput) (bool, *UserPromptParseInput) {
	if len(input.StartTime) == 10 && len(input.EndTime) == 10 {
		start, err := strconv.ParseInt(input.StartTime, 10, 64)
		if err != nil {
			return false, nil
		}
		end, err := strconv.ParseInt(input.EndTime, 10, 64)
		if err != nil {
			return false, nil
		}
		if end > start {
			input.start, input.end = start, end
			return true, input
		}
		return false, nil
	}
	return false, nil
}

func GetStatsAnalysisData(args *UserPromptParseInput, logger *zap.SugaredLogger) (*AiReqData, error) {
	reqData := &AiReqData{
		StartTime:   args.start,
		EndTime:     args.end,
		ProjectList: make([]*ProjectData, 0),
	}
	for _, project := range args.ProjectList {
		data := &ProjectData{
			ProjectName:       project,
			ProjectDataDetail: &DataDetail{},
		}

		for _, job := range args.JobList {
			switch job {
			case "build":
				build, err := getBuildData(project, args.start, args.end, logger)
				if err != nil {
					logger.Errorf("failed to get build data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.BuildInfo = build
			case "test":
				test, err := getTestData(project, args.start, args.end, logger)
				if err != nil {
					logger.Errorf("failed to get test data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.TestInfo = test
			case "deploy":
				deploy, err := getDeployData(project, args.start, args.end, logger)
				if err != nil {
					logger.Errorf("failed to get deploy data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.DeployInfo = deploy
			case "release":
				release, err := getReleaseData(project, args.start, args.end)
				if err != nil {
					logger.Errorf("failed to get release data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.ReleaseInfo = release
			}
		}

		// get system evaluation data
		system, err := getSystemEvaluationData(project, args.start, args.end, logger)
		if err != nil {
			logger.Errorf("failed to get system evaluation data from project %s, the error is: %+v", project, err)
		}
		data.SystemInternalEvaluationResult = system

		reqData.ProjectList = append(reqData.ProjectList, data)
	}
	return reqData, nil
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

	detail := &ReleaseDetails{
		ReleaseTotal:         totalCounter,
		ReleaseSuccessTotal:  passCounter,
		ReleaseFailureTotal:  totalCounter - passCounter,
		ReleaseTotalDuration: totalTimesTaken / int64(totalCounter),
	}
	return &ReleaseData{
		Description: "部署数据",
		Details:     detail,
	}, nil
}

func getSystemEvaluationData(project string, startTime, endTime int64, logger *zap.SugaredLogger) (string, error) {
	result, err := service2.GetStatsDashboard(startTime, endTime, []string{project}, logger)
	if err != nil {
		return "", err
	}
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(jsonResult), nil
}

type ExamplePrompt struct {
	Prompts []string `json:"prompts"`
}

// GetAiPrompts TODO: need to optimize,consider that whether to save prompt examples in db
func GetAiPrompts(logger *zap.SugaredLogger) (*ExamplePrompt, error) {
	return &ExamplePrompt{
		Prompts: PromptExamples,
	}, nil
}
