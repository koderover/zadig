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

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm/utils"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	service2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
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

	// get analysis data from db with parameters projectList, jobList and timeRange
	data, err := GetStatsAnalysisData(inputData, logger)
	if err != nil {
		logger.Errorf("failed to get project stat data, the error is: %+v", err)
		return nil, err
	}

	promptInput, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("failed to marshal data, the error is: %+v", err)
		return nil, err
	}
	prompt := fmt.Sprintf(ProjectAnalysisPrompt, args.Prompt, string(promptInput))
	tokenNum, err := llm.NumTokensFromPrompt(prompt, "")
	if err != nil {
		logger.Errorf("failed to get token num from prompt, the error is: %+v", err)
		return nil, err
	}

	ans := &analysisAnswer{
		answer: make(map[string]string, 0),
		m:      &sync.Mutex{},
	}
	var overAllInput string
	if tokenNum > AnalysisModelTokenLimit {
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
	if tokenNum > AnalysisModelTokenLimit {
		prompt = fmt.Sprintf("假设你是Devops专家，需要你根据分析要求分析三重引号分割的项目数据，该数据是多个项目各自的初步分析结果，"+
			"分析要求:%s;你的回答需要使用text格式输出,输出内容不要包含\"三重引号分割的项目数据\"这个名称,也不要复述分析要求中的内容,在你的回答中禁止包含 "+
			"\\\"data_description\\\"、\\\"jenkins\\\" 等字段; 项目数据：\"\"\"%s\"\"\"", args.Prompt, overAllInput)
	}
	options := []llm.ParamOption{llm.WithTemperature(float32(0.2))}
	if client.GetModel() != "" {
		options = append(options, llm.WithModel(client.GetModel()))
	} else {
		options = append(options, llm.WithModel(AnalysisModel))
	}
	answer, err := client.GetCompletion(context.TODO(), util.RemoveExtraSpaces(prompt), options...)
	if err != nil {
		logger.Errorf("failed to get answer from ai: %v, the error is: %+v", client.GetName(), err)
		return nil, err
	}

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
	options := []llm.ParamOption{llm.WithTemperature(float32(0.1))}
	if client.GetModel() != "" {
		options = append(options, llm.WithModel(client.GetModel()))
	} else {
		options = append(options, llm.WithModel(AnalysisModel))
	}
	answer, err := client.GetCompletion(context.TODO(), util.RemoveExtraSpaces(prompt), options...)
	if err != nil {
		logger.Errorf("failed to get answer from ai: %v, the error is: %+v", client.GetName(), err)
		return
	}

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
	projectList, err := commonrepo.NewJobInfoColl().GetAllProjectNameByTypeName(0, 0, "")
	if err != nil {
		return input, err
	}

	jobs := []string{
		"build",
		"test",
		"deploy",
		"release",
	}

	prompt := fmt.Sprintf("%s;\"\"\"%s\"\"\"", util.RemoveExtraSpaces(ParseUserPromptPrompt), args.Prompt)
	options := []llm.ParamOption{}
	if aiClient.GetModel() != "" {
		options = append(options, llm.WithModel(aiClient.GetModel()))
	} else {
		options = append(options, llm.WithModel(AnalysisModel))
	}
	resp, err := aiClient.GetCompletion(context.TODO(), prompt, options...)
	if err != nil {
		return input, fmt.Errorf("failed to get completion, error: %v", err)
	}
	resp = strings.TrimPrefix(resp, "```json\n")
	resp = strings.TrimSuffix(resp, "```")

	// parse the user prompt to prepare the input data of projects stat
	err = json.Unmarshal([]byte(resp), &input)
	if err != nil {
		return input, fmt.Errorf("failed to unmarshal response, error: %v", err)
	}

	if err := checkInputData(input, jobs, projectList); err != nil {
		return nil, fmt.Errorf("failed to check input data, error: %v", err)
	}

	err = getTimeParseResult(args.Prompt, input, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get time parse result, error: %v", err)
	}
	return input, nil
}

// TODO: check the input data
func checkInputData(input *UserPromptParseInput, jobs, projects []string) error {
	if len(input.ProjectList) > 0 {
		for _, project := range input.ProjectList {
			if !utils.Contains(projects, project) {
				return fmt.Errorf("the project %s is not exist", project)
			}
		}
	}
	if len(input.JobList) > 0 {
		for _, job := range input.JobList {
			if !utils.Contains(jobs, job) {
				return fmt.Errorf("the job %s is not exist", job)
			}
		}
	}

	if len(input.ProjectList) == 0 && len(projects) > 0 {
		input.ProjectList = projects
	}
	if len(input.JobList) == 0 && len(jobs) > 0 {
		input.JobList = jobs
	}

	return nil
}

type UserPromptTimeParseResult struct {
	Prompt      string   `json:"prompt"`
	ErrMessage  string   `json:"err_message"`
	ParseResult []string `json:"parse_result"`
}

func getTimeParseResult(prompt string, input *UserPromptParseInput, logger *zap.SugaredLogger) error {
	// parse the time in user prompt by rest api
	userPrompt := struct {
		Prompt string `json:"prompt"`
	}{
		Prompt: prompt,
	}
	body, err := json.Marshal(userPrompt)
	if err != nil {
		return err
	}

	host := setting.Services[setting.TimeNlp]
	url := fmt.Sprintf("http://%s:%d/%s", host.Name, host.Port, "api/prompt/time")
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logger.Infof("finished Request NLP in getTimeParseResult method, the response is: \n%s", string(responseBody))

	result := &UserPromptTimeParseResult{}
	err = json.Unmarshal(responseBody, result)
	if err != nil {
		return err
	}

	// check the parse result
	if len(result.ParseResult) != 2 {
		return fmt.Errorf("the parse result is not correct, the parse result is: %s", result.ParseResult)
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", result.ParseResult[0], time.Local)
	if err != nil {
		return err
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", result.ParseResult[1], time.Local)
	if err != nil {
		return err
	}
	input.StartTime = start.Unix()
	input.EndTime = end.Unix()
	now := time.Now().Unix()
	if end.Unix() > now {
		input.EndTime = now
	}
	return nil
}

func GetStatsAnalysisData(args *UserPromptParseInput, logger *zap.SugaredLogger) (*AiReqData, error) {
	reqData := &AiReqData{
		StartTime:   args.StartTime,
		EndTime:     args.EndTime,
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
				build, err := getBuildData(project, args.StartTime, args.EndTime, logger)
				if err != nil {
					logger.Errorf("failed to get build data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.BuildInfo = build
			case "test":
				test, err := getTestData(project, args.StartTime, args.EndTime, logger)
				if err != nil {
					logger.Errorf("failed to get test data from project %s, the error is: %+v", project, err)
				}
				data.ProjectDataDetail.TestInfo = test
			case "deploy":
				deploy, err := getDeployData(project, args.StartTime, args.EndTime, logger)
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

func getReleaseData(project string, startTime, endTime int64) (*ReleaseData, error) {
	// get release data from mongo
	releaseJobList, err := service2.GetProjectReleaseStat(startTime, endTime, project)
	if err != nil {
		return nil, err
	}

	detail := &ReleaseDetails{
		ReleaseTotal:         releaseJobList.Total,
		ReleaseSuccessTotal:  releaseJobList.Success,
		ReleaseFailureTotal:  releaseJobList.Failure,
		ReleaseTotalDuration: int64(releaseJobList.Duration),
	}
	return &ReleaseData{
		Description: fmt.Sprintf("%s项目在%s到%s期间发布相关数据，包括发布总次数，发布成功次数，发布失败次数, 发布周趋势数据，发布每日数据", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02")),
		Details:     detail,
	}, nil
}

type SystemEvaluation struct {
	ProjectName      string `json:"project_name"`
	EvaluationResult string `json:"evaluation_result"`
	Description      string `json:"data_description"`
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

	data := &SystemEvaluation{
		ProjectName:      project,
		EvaluationResult: string(jsonResult),
		Description:      fmt.Sprintf("%s项目在%s到%s期间系统评估结果,此评估结果由第三方api产生的数据和zadig系统内部的数据利用管理员设置的数学模型来计算获取的。", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02")),
	}
	jsonStr, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(jsonStr), nil
}

type ExamplePrompt struct {
	Prompts []string `json:"prompts"`
}

// GetAiPrompts TODO: need to optimize,consider that whether to save prompt examples in db
func GetAiPrompts(logger *zap.SugaredLogger) (*ExamplePrompt, error) {
	list := make([]string, 0)
	projects, err := commonrepo.NewJobInfoColl().GetAllProjectNameByTypeName(time.Now().AddDate(0, -1, 0).Unix(), time.Now().Unix(), "")
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, nil
	}
	rand.Seed(time.Now().UnixNano())
	list = append(list, fmt.Sprintf("分析 %s 项目近一个月的质量和效率，并给出优化建议", projects[rand.Intn(len(projects))]))
	list = append(list, "分析所有项目近一个月的整体表现，并给出优化建议")
	list = append(list, fmt.Sprintf("分析 %s 项目近一个月的质量数据，预测未来的趋势和潜在问题", projects[rand.Intn(len(projects))]))
	list = append(list, fmt.Sprintf("通过历史数据，请用简洁的文字总结%s项目最近一个月的整体表现。", projects[rand.Intn(len(projects))]))
	list = append(list, fmt.Sprintf("请根据项目%s的构建、部署、测试和发布等数据，分析项目最近一个月的现状，并基于历史数据，分析未来的趋势和潜在问题，并提出改进建议。", projects[rand.Intn(len(projects))]))

	return &ExamplePrompt{
		Prompts: list,
	}, nil
}

type AIAttentionResp struct {
	Answer []AttentionAnswer `json:"answer"`
}

type AttentionAnswer struct {
	Project      string `json:"project"`
	ProjectAlias string `json:"project_alias"`
	Result       string `json:"result"`
	Name         string `json:"name"`
	CurrentMonth string `json:"current_month"`
	LastMonth    string `json:"last_month"`
}

func AnalyzeMonthAttention(start, end int64, data []*service2.MonthAttention, logger *zap.SugaredLogger) (*AIAttentionResp, error) {
	client, err := service.GetDefaultLLMClient(context.TODO())
	if err != nil {
		logger.Errorf("failed to get llm client, the error is: %+v", err)
		return nil, err
	}

	jsonStr, err := json.Marshal(data)
	if err != nil {
		log.Errorf("failed to marshal MonthAttentionData, the error is: %+v", err)
		return nil, err
	}
	input := string(jsonStr)
	prompt := fmt.Sprintf("你是DevOps专家，你需要根据三重引号分割的输入数据，该数据分别包括了%d月和%d月每个月各自的构建成功率，测试成功率，部署成功率，发布成功率，发布频次，"+
		"以及需求研发周期等数据;%s;输入数据\"\"\"%s\"\"\"", time.Now().AddDate(0, -1, 0).Month(), time.Now().Month(), AttentionPrompt, input)

	retryTime := 0
	answer := ""
	for retryTime < 3 {
		options := []llm.ParamOption{llm.WithTemperature(float32(0.2))}
		if client.GetModel() != "" {
			options = append(options, llm.WithModel(client.GetModel()))
		} else {
			options = append(options, llm.WithModel(AnalysisModel))
		}
		answer, err = client.GetCompletion(context.TODO(), util.RemoveExtraSpaces(prompt), options...)
		if err != nil {
			retryTime++
			if strings.Contains(err.Error(), "create chat completion failed") && retryTime < 3 {
				continue
			}
			logger.Errorf("failed to get completion analyze answer, the error is: %+v", err)
			return nil, err
		} else {
			break
		}
	}

	answer = strings.TrimPrefix(answer, "```json\n")
	answer = strings.TrimSuffix(answer, "```")
	resp := &AIAttentionResp{}
	err = json.Unmarshal([]byte(answer), resp)
	if err != nil {
		logger.Errorf("failed to unmarshal answer, ai may return the  the error is wrong: %+v", err)
		return nil, ReturnAnswerWrongFormat
	}

	return resp, nil
}
