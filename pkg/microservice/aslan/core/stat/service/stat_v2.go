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

package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/util"
)

func CreateStatDashboardConfig(args *StatDashboardConfig, logger *zap.SugaredLogger) error {
	config := &commonmodels.StatDashboardConfig{
		Type:     args.Type,
		ItemKey:  args.ID,
		Name:     args.Name,
		Source:   args.Source,
		Function: args.Function,
		Weight:   args.Weight,
	}

	if args.APIConfig != nil {
		config.APIConfig = &commonmodels.APIConfig{
			ExternalSystemId: args.APIConfig.ExternalSystemId,
			ApiPath:          args.APIConfig.ApiPath,
			Queries:          args.APIConfig.Queries,
		}
	}

	err := commonrepo.NewStatDashboardConfigColl().Create(context.TODO(), config)
	if err != nil {
		logger.Errorf("failed to create config for type: %s, error: %s", args.Type, err)
		return e.ErrCreateStatisticsDashboardConfig.AddDesc(err.Error())
	}
	return nil
}

func ListDashboardConfigs(logger *zap.SugaredLogger) ([]*StatDashboardConfig, error) {
	configs, err := commonrepo.NewStatDashboardConfigColl().List()
	if err != nil {
		logger.Errorf("failed to list dashboard configs, error: %s", err)
		return nil, e.ErrListStatisticsDashboardConfig.AddDesc(err.Error())
	}

	if len(configs) == 0 {
		err := initializeStatDashboardConfig()
		if err != nil {
			logger.Errorf("failed to initialize dashboard configs, error: %s", err)
			return nil, e.ErrListStatisticsDashboardConfig.AddDesc(err.Error())
		}
		configs = createDefaultStatDashboardConfig()
	}

	var result []*StatDashboardConfig
	for _, config := range configs {
		currentResult := &StatDashboardConfig{
			ID:       config.ItemKey,
			Type:     config.Type,
			Name:     config.Name,
			Source:   config.Source,
			Function: config.Function,
			Weight:   config.Weight,
		}
		if config.APIConfig != nil {
			currentResult.APIConfig = &APIConfig{
				ExternalSystemId: config.APIConfig.ExternalSystemId,
				ApiPath:          config.APIConfig.ApiPath,
				Queries:          config.APIConfig.Queries,
			}
		}
		result = append(result, currentResult)
	}
	return result, nil
}

func UpdateStatDashboardConfig(id string, args *StatDashboardConfig, logger *zap.SugaredLogger) error {
	config := &commonmodels.StatDashboardConfig{
		Type:     args.Type,
		ItemKey:  args.ID,
		Name:     args.Name,
		Source:   args.Source,
		Function: args.Function,
		Weight:   args.Weight,
	}

	if args.APIConfig != nil {
		config.APIConfig = &commonmodels.APIConfig{
			ExternalSystemId: args.APIConfig.ExternalSystemId,
			ApiPath:          args.APIConfig.ApiPath,
			Queries:          args.APIConfig.Queries,
		}
	}

	err := commonrepo.NewStatDashboardConfigColl().Update(context.TODO(), id, config)
	if err != nil {
		logger.Errorf("failed to update config for type: %s, error: %s", args.Type, err)
		return e.ErrUpdateStatisticsDashboardConfig.AddDesc(err.Error())
	}
	return nil
}

func DeleteStatDashboardConfig(id string, logger *zap.SugaredLogger) error {
	err := commonrepo.NewStatDashboardConfigColl().Delete(context.TODO(), id)
	if err != nil {
		logger.Errorf("failed to delete config for id: %s, error: %s", id, err)
		e.ErrDeleteStatisticsDashboardConfig.AddDesc(err.Error())
	}
	return nil
}

func GetStatsDashboard(startTime, endTime int64, projectList []string, logger *zap.SugaredLogger) ([]*StatDashboardByProject, error) {
	resp := make([]*StatDashboardByProject, 0)

	configs, err := commonrepo.NewStatDashboardConfigColl().List()
	if err != nil {
		logger.Errorf("failed to list dashboard configs, error: %s", err)
		return nil, e.ErrGetStatisticsDashboard.AddDesc(err.Error())
	}

	if len(configs) == 0 {
		err := initializeStatDashboardConfig()
		if err != nil {
			logger.Errorf("failed to initialize dashboard configs, error: %s", err)
			return nil, e.ErrGetStatisticsDashboard.AddDesc(err.Error())
		}
		configs = createDefaultStatDashboardConfig()
	}

	projects, err := templaterepo.NewProductColl().ListProjectBriefs(projectList)
	if err != nil {
		logger.Errorf("failed to list projects to create dashborad, error: %s", err)
		return nil, e.ErrGetStatisticsDashboard.AddDesc(err.Error())
	}

	for _, project := range projects {
		facts := make([]*StatDashboardItem, 0)
		for _, config := range configs {
			cfg := &StatDashboardConfig{
				ID:       config.ItemKey,
				Type:     config.Type,
				Name:     config.Name,
				Source:   config.Source,
				Function: config.Function,
				Weight:   config.Weight,
			}
			if config.APIConfig != nil {
				cfg.APIConfig = &APIConfig{
					ExternalSystemId: config.APIConfig.ExternalSystemId,
					ApiPath:          config.APIConfig.ApiPath,
					Queries:          config.APIConfig.Queries,
				}
			}
			calculator, err := CreateCalculatorFromConfig(cfg)
			if err != nil {
				logger.Errorf("failed to create calculator for project: %s, fact key: %s, error: %s", project.Name, config.ItemKey, err)
				// if for some reason we failed to create the calculator, we append a fact with value 0, and error along with it
				facts = append(facts, &StatDashboardItem{
					Type:  config.Type,
					ID:    config.ItemKey,
					Data:  0,
					Score: 0,
					Error: err.Error(),
				})
				continue
			}
			fact, exists, err := calculator.GetFact(startTime, endTime, project.Name)
			if err != nil {
				logger.Errorf("failed to get fact for project: %s, fact key: %s, error: %s", project.Name, config.ItemKey, err)
				// if for some reason we failed to get the fact, we append a fact with value 0, and error along with it
				facts = append(facts, &StatDashboardItem{
					Type:     config.Type,
					ID:       config.ItemKey,
					Data:     0,
					Score:    0,
					Error:    err.Error(),
					HasValue: exists,
				})
				continue
			}
			// we round the fact to 2 decimal places
			fact = math.Round(fact*100) / 100
			// otherwise we calculate the score and append the fact
			score, err := calculator.GetWeightedScore(fact)
			if err != nil {
				logger.Errorf("failed to calculate score for project: %s, fact key: %s, error: %s", project.Name, config.ItemKey, err)
				score = 0
			}
			if !exists {
				score = 0
			}

			item := &StatDashboardItem{
				Type:     config.Type,
				ID:       config.ItemKey,
				Data:     fact,
				Score:    math.Round(score*100) / 100,
				HasValue: exists,
			}
			if err != nil {
				item.Error = err.Error()
			}
			facts = append(facts, item)
		}

		// once all configured facts are calculated, we calculate the total score
		totalScore := 0.0
		for _, fact := range facts {
			totalScore += fact.Score
		}

		resp = append(resp, &StatDashboardByProject{
			ProjectKey:  project.Name,
			ProjectName: project.Alias,
			Score:       math.Round(totalScore*100) / 100,
			Facts:       facts,
		})
	}

	return resp, nil
}

func GetStatsDashboardGeneralData(startTime, endTime int64, logger *zap.SugaredLogger) (*StatDashboardBasicData, error) {
	totalDeployStats, err := commonrepo.NewJobInfoColl().GetDeployJobsStats(startTime, endTime, []string{}, config.Both)
	if err != nil {
		logger.Errorf("failed to get total deployment count, error: %s", err)
		return nil, err
	}

	var deployTotal, deploySuccess, productionDeployTotal, productionDeploySuccess int64
	for _, deployStat := range totalDeployStats {
		if deployStat.Production {
			productionDeployTotal += int64(deployStat.Count)
			productionDeploySuccess += int64(deployStat.Success)
		} else {
			deployTotal += int64(deployStat.Count)
			deploySuccess += int64(deployStat.Success)
		}
	}

	totalBuildSuccess, totalBuildFailure, err := repo.NewBuildStatColl().GetBuildTotalAndSuccessByTime(startTime, endTime)
	if err != nil {
		logger.Errorf("failed to get total and success build count, error: %s", err)
		return nil, err
	}
	testJobs, err := commonrepo.NewJobInfoColl().GetTestJobs(startTime, endTime, "")
	if err != nil {
		logger.Errorf("failed to get test jobs, error: %s", err)
		return nil, err
	}
	totalTestExecution := 0
	totalTestSuccess := 0
	for _, job := range testJobs {
		totalTestExecution++
		if job.Status == "passed" {
			totalTestSuccess++
		}
	}
	return &StatDashboardBasicData{
		BuildTotal:              totalBuildSuccess + totalBuildFailure,
		BuildSuccess:            totalBuildSuccess,
		TestTotal:               int64(totalTestExecution),
		TestSuccess:             int64(totalTestSuccess),
		DeployTotal:             deployTotal,
		DeploySuccess:           deploySuccess,
		ProductionDeployTotal:   productionDeployTotal,
		ProductionDeploySuccess: productionDeploySuccess,
	}, nil
}

var defaultStatDashboardConfigMap = map[string]*commonmodels.StatDashboardConfig{
	config.DashboardDataTypeBuildAverageDuration: {
		Type:     config.DashboardDataCategoryEfficiency,
		Name:     "构建平均耗时",
		ItemKey:  config.DashboardDataTypeBuildAverageDuration,
		Source:   config.DashboardDataSourceZadig,
		Function: config.DashboardFunctionBuildAverageDuration,
		Weight:   100,
	},
	config.DashboardDataTypeBuildSuccessRate: {
		Type:     config.DashboardDataCategoryEfficiency,
		Name:     "构建成功率",
		ItemKey:  config.DashboardDataTypeBuildSuccessRate,
		Source:   config.DashboardDataSourceZadig,
		Function: config.DashboardFunctionBuildSuccessRate,
		Weight:   0,
	},
	config.DashboardDataTypeDeploySuccessRate: {
		Type:     config.DashboardDataCategoryEfficiency,
		Name:     "部署成功率",
		ItemKey:  config.DashboardDataTypeDeploySuccessRate,
		Source:   config.DashboardDataSourceZadig,
		Function: config.DashboardFunctionDeploySuccessRate,
		Weight:   0,
	},
	config.DashboardDataTypeDeployFrequency: {
		Type:     config.DashboardDataCategoryEfficiency,
		Name:     "部署频次(周）",
		ItemKey:  config.DashboardDataTypeDeployFrequency,
		Source:   config.DashboardDataSourceZadig,
		Function: config.DashboardFunctionDeployFrequency,
		Weight:   0,
	},
	config.DashboardDataTypeTestPassRate: {
		Type:     config.DashboardDataCategoryQuality,
		Name:     "测试通过率",
		ItemKey:  config.DashboardDataTypeTestPassRate,
		Source:   config.DashboardDataSourceZadig,
		Function: config.DashboardFunctionTestPassRate,
		Weight:   0,
	},
	config.DashboardDataTypeTestAverageDuration: {
		Type:     config.DashboardDataCategoryEfficiency,
		Name:     "测试平均耗时",
		ItemKey:  config.DashboardDataTypeTestAverageDuration,
		Source:   config.DashboardDataSourceZadig,
		Function: config.DashboardFunctionTestAverageDuration,
		Weight:   0,
	},
	config.DashboardDataTypeReleaseFrequency: {
		Type:     config.DashboardDataCategoryEfficiency,
		Name:     "发布频次(周）",
		ItemKey:  config.DashboardDataTypeReleaseFrequency,
		Source:   config.DashboardDataSourceZadig,
		Function: config.DashboardFunctionReleaseFrequency,
		Weight:   0,
	},
}

func createDefaultStatDashboardConfig() []*commonmodels.StatDashboardConfig {
	ret := make([]*commonmodels.StatDashboardConfig, 0)
	for _, cfg := range defaultStatDashboardConfigMap {
		ret = append(ret, cfg)
	}
	return ret
}

func initializeStatDashboardConfig() error {
	return commonrepo.NewStatDashboardConfigColl().BulkCreate(context.TODO(), createDefaultStatDashboardConfig())
}

type DailyJobInfo struct {
	Name  string             `json:"name"`
	Datas []DailyJobInfoData `json:"datas"`
}

type DailyJobInfoData struct {
	Timestamp int64 `json:"timestamp"`
	Count     int   `json:"count"`
}

type project30DayOverview struct {
	name string
	data []*currently30DayOverview
}

type currently30DayOverview struct {
	day   int64
	count int
}

func GetProjectsOverview(start, end int64, logger *zap.SugaredLogger) ([]*DailyJobInfo, error) {
	result, err := commonrepo.NewJobInfoColl().GetJobInfos(start, end, nil)
	if err != nil {
		err = fmt.Errorf("failed to get coarse grained data from job_info collection, error: %s", err)
		logger.Error(err)
		return nil, err
	}

	// because the mongodb version 3.4 does not support convert timestamp to date(no $toDate,$convert), we have to do the join in the code
	buildJobs := &project30DayOverview{
		name: "构建",
		data: make([]*currently30DayOverview, 0),
	}
	testJobs := &project30DayOverview{
		name: "测试",
		data: make([]*currently30DayOverview, 0),
	}
	deployJobs := &project30DayOverview{
		name: "部署",
		data: make([]*currently30DayOverview, 0),
	}

	currently30DayOverviewMap := map[int64]*struct {
		buildDayData  *currently30DayOverview
		testDayData   *currently30DayOverview
		deployDayData *currently30DayOverview
	}{}
	for i := 0; i < len(result); i++ {
		weekdayTimeStamp := util.GetEndOfWeekDayTimeStamp(time.Unix(result[i].StartTime, 0))
		start := weekdayTimeStamp - 24*60*60*6
		end := weekdayTimeStamp + 24*60*60

		var (
			buildDayData  *currently30DayOverview
			testDayData   *currently30DayOverview
			deployDayData *currently30DayOverview
		)

		if _, ok := currently30DayOverviewMap[weekdayTimeStamp]; !ok {
			buildDayData = &currently30DayOverview{
				day:   weekdayTimeStamp,
				count: 0,
			}
			testDayData = &currently30DayOverview{
				day:   weekdayTimeStamp,
				count: 0,
			}
			deployDayData = &currently30DayOverview{
				day:   weekdayTimeStamp,
				count: 0,
			}
		} else {
			buildDayData = currently30DayOverviewMap[weekdayTimeStamp].buildDayData
			testDayData = currently30DayOverviewMap[weekdayTimeStamp].testDayData
			deployDayData = currently30DayOverviewMap[weekdayTimeStamp].deployDayData
		}

		for j := i; j < len(result); j++ {
			if result[j].StartTime >= start && result[j].StartTime <= end {
				switch result[j].Type {
				case string(config.JobZadigBuild):
					buildDayData.count++
				case string(config.JobZadigTesting):
					testDayData.count++
				case string(config.JobZadigDeploy):
					deployDayData.count++
				}
			} else {
				buildJobs.data = append(buildJobs.data, buildDayData)
				testJobs.data = append(testJobs.data, testDayData)
				deployJobs.data = append(deployJobs.data, deployDayData)
				i = j - 1
				break
			}
		}
	}
	resp := make([]*DailyJobInfo, 0)
	resp = append(resp, reBuildData(start, end, buildJobs), reBuildData(start, end, testJobs), reBuildData(start, end, deployJobs))
	return resp, nil
}

func reBuildData(start, end int64, data *project30DayOverview) *DailyJobInfo {
	resp := &DailyJobInfo{
		Name:  data.name,
		Datas: make([]DailyJobInfoData, 0),
	}

	sort.Slice(data.data, func(i, j int) bool {
		return data.data[i].day < data.data[j].day
	})

	for _, d := range data.data {
		jobInfodata := DailyJobInfoData{
			Timestamp: d.day,
			Count:     d.count,
		}
		resp.Datas = append(resp.Datas, jobInfodata)
	}
	return resp
}

type Currently30DayBuildTrend struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
	Data  []int  `json:"data"`
}

type project30DayBuildData struct {
	Name  string                     `json:"name"`
	Alias string                     `json:"alias"`
	Data  []*currently30DayBuildData `json:"data"`
}

type currently30DayBuildData struct {
	Day   int64 `json:"day"`
	Count int   `json:"count"`
}

func GetCurrently30DayBuildTrend(startTime, endTime int64, projects []string, logger *zap.SugaredLogger) ([]*Currently30DayBuildTrend, error) {
	result, err := commonrepo.NewJobInfoColl().GetBuildTrend(startTime, endTime, projects)
	if err != nil {
		logger.Errorf("failed to get coarse grained data from job_info collection, error: %s", err)
		return nil, err
	}
	// bstr, _ := json.Marshal(result)
	// logger.Infof("start:%d, end:%d, get coarse grained data: %s", startTime, endTime, string(bstr))

	projects, err = commonrepo.NewJobInfoColl().GetAllProjectNameByTypeName(startTime, endTime, string(config.JobZadigBuild))
	if err != nil {
		logger.Errorf("failed to get all project name from job_info collection, error: %s", err)
		return nil, err
	}

	projectInfos, err := templaterepo.NewProductColl().ListProjectBriefs(projects)
	if err != nil {
		err = fmt.Errorf("failed to get project infos for %v, error: %s", projects, err)
		logger.Error(err)
		return nil, err
	}
	projectAliasMap := map[string]string{}
	for _, projectInfo := range projectInfos {
		projectAliasMap[projectInfo.Name] = projectInfo.Alias
	}

	logger.Infof("start:%d, end:%d, get all project name: %v", startTime, endTime, projects)
	resp := make([]*project30DayBuildData, 0)
	for _, project := range projects {
		trend := &project30DayBuildData{
			Name:  project,
			Alias: projectAliasMap[project],
			Data:  make([]*currently30DayBuildData, 0),
		}

		for i := 0; i < len(result); i++ {
			if result[i].ProductName != project {
				continue
			}
			start := util.GetMidnightTimestamp(result[i].StartTime)
			end := time.Unix(start, 0).Add(time.Hour * 24).Unix()
			data := &currently30DayBuildData{
				Day:   start,
				Count: 0,
			}
			for j := i; j < len(result); j++ {
				if result[j].ProductName == project {
					if result[j].StartTime >= start && result[j].StartTime < end {
						data.Count++
					} else {
						trend.Data = append(trend.Data, data)
						i = j - 1
						break
					}
				} else {
					if result[j].StartTime >= end {
						trend.Data = append(trend.Data, data)
						i = j
						break
					}
					continue
				}
			}
		}
		resp = append(resp, trend)
	}
	// jstr, _ := json.Marshal(resp)
	// logger.Infof("start:%d, end:%d, get resp: %s", startTime, endTime, string(jstr))
	return clearData(RebuildCurrently30DayBuildData(startTime, endTime, resp)), nil
}

func clearData(data []*Currently30DayBuildTrend) []*Currently30DayBuildTrend {
	resp := make([]*Currently30DayBuildTrend, 0)
	for _, d := range data {
		if len(d.Data) > 0 {
			resp = append(resp, d)
		}
	}
	return resp
}

func RebuildCurrently30DayBuildData(start, end int64, data []*project30DayBuildData) []*Currently30DayBuildTrend {
	start = util.GetMidnightTimestamp(start)
	resp := make([]*Currently30DayBuildTrend, 0)
	for _, project := range data {
		index := 0
		buildTrend := &Currently30DayBuildTrend{
			Name:  project.Name,
			Alias: project.Alias,
			Data:  make([]int, 0),
		}
		for day := start; day <= end; day = time.Unix(day, 0).Add(time.Hour * 24).Unix() {
			sort.Slice(project.Data, func(i, j int) bool {
				return project.Data[i].Day < project.Data[j].Day
			})

			if index < len(project.Data) && day == project.Data[index].Day {
				buildTrend.Data = append(buildTrend.Data, project.Data[index].Count)
				index++
			} else {
				buildTrend.Data = append(buildTrend.Data, 0)
			}
		}
		resp = append(resp, buildTrend)
	}
	return resp
}

type EfficiencyRadarData struct {
	Name                           string  `json:"name"`
	Alias                          string  `json:"alias"`
	TestSuccessRate                float64 `json:"test_success_rate"`
	ReleaseFrequency               float64 `json:"release_frequency"`
	BuildFrequency                 float64 `json:"build_frequency"`
	ReleaseSuccessRate             float64 `json:"release_success_rate"`
	RequirementDevelopmentLeadTime float64 `json:"requirement_development_lead_time"`
}

// GetEfficiencyRadar Return test pass rate, release frequency, R&D lead time within the past 30 days (if configured)
func GetEfficiencyRadar(startTime, endTime int64, projects []string, logger *zap.SugaredLogger) ([]*EfficiencyRadarData, error) {
	projects, err := commonrepo.NewJobInfoColl().GetAllProjectNameByTypeName(startTime, endTime, "")
	if err != nil {
		logger.Errorf("failed to get all project name from job_info collection, error: %s", err)
		return nil, err
	}
	projectInfos, err := templaterepo.NewProductColl().ListProjectBriefs(projects)
	if err != nil {
		err = fmt.Errorf("failed to get project infos for %v, error: %s", projects, err)
		logger.Error(err)
		return nil, err
	}
	projectAliasMap := map[string]string{}
	for _, projectInfo := range projectInfos {
		projectAliasMap[projectInfo.Name] = projectInfo.Alias
	}

	resp := make([]*EfficiencyRadarData, 0)
	for _, project := range projects {
		radarData := &EfficiencyRadarData{
			Name:  project,
			Alias: projectAliasMap[project],
		}
		// test pass rate
		testStat, err := GetProjectTestStat(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get test stat, error: %s", err)
			return nil, err
		}
		if testStat.Total != 0 {
			radarData.TestSuccessRate = getDestFloat(float64(testStat.Success) / float64(testStat.Total) * 100)
		}

		// release success rate
		releaseStat, err := GetProjectReleaseStat(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get release stat, error: %s", err)
			return nil, err
		}
		if releaseStat.Total != 0 {
			radarData.ReleaseSuccessRate = getDestFloat(float64(releaseStat.Success) / float64(releaseStat.Total) * 100)
		}

		// release frequency
		// get release_frequency
		releaseFrequencyCalculator := &ReleaseFrequencyCalculator{}
		releaseFrequency, _, err := releaseFrequencyCalculator.GetFact(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get release frequency, error: %s", err)
			return nil, err
		}
		radarData.ReleaseFrequency = getDestFloat(releaseFrequency)

		// build frequency
		buildFrequencyCalculator := &BuildFrequencyCalculator{}
		buildFrequency, _, err := buildFrequencyCalculator.GetFact(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get build frequency, error: %s", err)
			return nil, err
		}
		radarData.BuildFrequency = getDestFloat(buildFrequency)

		// R&D lead time
		leadTime, err := GetRequirementDevelopmentLeadTime(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get lead time stat, error: %s", err)
			// TODO: the api is not stable, so we ignore the error
			//return nil, err
		}
		radarData.RequirementDevelopmentLeadTime = leadTime

		resp = append(resp, radarData)
	}
	return resp, nil
}

type MonthAttention struct {
	ProjectName  string                `json:"project_name"`
	ProjectAlias string                `json:"project_alias"`
	Facts        []*MonthAttentionData `json:"facts"`
}

type MonthAttentionData struct {
	Name         string `json:"name"`
	CurrentMonth string `json:"current_month"`
	LastMonth    string `json:"last_month"`
}

func GetMonthAttention(startTime, endTime int64, projects []string, logger *zap.SugaredLogger) ([]*MonthAttention, error) {
	CurrentMonthStart := startTime
	CurrentMonthEnd := endTime
	LastMonthStart := time.Unix(CurrentMonthStart, 0).AddDate(0, -1, 0).Unix()
	LastMonthEnd := CurrentMonthStart - 1

	projects, err := commonrepo.NewJobInfoColl().GetAllProjectNameByTypeName(startTime, endTime, "")
	if err != nil {
		logger.Errorf("failed to get all project name from job_info collection, error: %s", err)
		return nil, err
	}
	projectInfos, err := templaterepo.NewProductColl().ListProjectBriefs(projects)
	if err != nil {
		err = fmt.Errorf("failed to get project infos for %v, error: %s", projects, err)
		logger.Error(err)
		return nil, err
	}
	projectAliasMap := map[string]string{}
	for _, projectInfo := range projectInfos {
		projectAliasMap[projectInfo.Name] = projectInfo.Alias
	}

	resp := make([]*MonthAttention, 0)
	for _, project := range projects {
		monthAttention := &MonthAttention{
			ProjectName:  project,
			ProjectAlias: projectAliasMap[project],
			Facts:        []*MonthAttentionData{},
		}
		// get build_success_rate
		currentBuild, err := GetProjectBuildStat(CurrentMonthStart, CurrentMonthEnd, project)
		if err != nil {
			logger.Errorf("failed to get build stat, error: %s", err)
			return nil, err
		}
		LastBuild, err := GetProjectBuildStat(LastMonthStart, LastMonthEnd, project)
		if err != nil {
			logger.Errorf("failed to get build stat, error: %s", err)
			return nil, err
		}
		curRate, lastRate := 0.0, 0.0
		if currentBuild.Total != 0 {
			curRate = float64(currentBuild.Success) / float64(currentBuild.Total) * 100
		}
		if LastBuild.Total != 0 {
			lastRate = float64(LastBuild.Success) / float64(LastBuild.Total) * 100
		}
		buildSuccessRate := &MonthAttentionData{
			Name:         "build_success_rate",
			CurrentMonth: fmt.Sprintf("%.2f", curRate),
			LastMonth:    fmt.Sprintf("%.2f", lastRate),
		}
		monthAttention.Facts = append(monthAttention.Facts, buildSuccessRate)

		// get test_success_rate
		currentTest, err := GetProjectTestStat(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get test stat, error: %s", err)
			return nil, err
		}
		LastTest, err := GetProjectTestStat(LastMonthStart, LastMonthEnd, project)
		if err != nil {
			logger.Errorf("failed to get test stat, error: %s", err)
			return nil, err
		}
		curRate, lastRate = 0.0, 0.0
		if currentTest.Total != 0 {
			curRate = float64(currentTest.Success) / float64(currentTest.Total) * 100
		}
		if LastTest.Total != 0 {
			lastRate = float64(LastTest.Success) / float64(LastTest.Total) * 100
		}
		testSuccessRate := &MonthAttentionData{
			Name:         "test_success_rate",
			CurrentMonth: fmt.Sprintf("%.2f", curRate),
			LastMonth:    fmt.Sprintf("%.2f", lastRate),
		}
		monthAttention.Facts = append(monthAttention.Facts, testSuccessRate)

		// get deploy_success_rate
		currentDeploy, err := GetProjectDeployStat(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get deploy stat, error: %s", err)
			return nil, err
		}
		LastDeploy, err := GetProjectDeployStat(LastMonthStart, LastMonthEnd, project)
		if err != nil {
			logger.Errorf("failed to get deploy stat, error: %s", err)
			return nil, err
		}
		curRate, lastRate = 0, 0
		if currentDeploy.Total != 0 {
			curRate = float64(currentDeploy.Success) / float64(currentDeploy.Total) * 100
		}
		if LastDeploy.Total != 0 {
			lastRate = float64(LastDeploy.Success) / float64(LastDeploy.Total) * 100
		}
		deploySuccessRate := &MonthAttentionData{
			Name:         "deploy_success_rate",
			CurrentMonth: fmt.Sprintf("%.2f", curRate),
			LastMonth:    fmt.Sprintf("%.2f", lastRate),
		}
		monthAttention.Facts = append(monthAttention.Facts, deploySuccessRate)

		// get release_success_rate
		currentRelease, err := GetProjectReleaseStat(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get release stat, error: %s", err)
			return nil, err
		}
		LastRelease, err := GetProjectReleaseStat(LastMonthStart, LastMonthEnd, project)
		if err != nil {
			logger.Errorf("failed to get release stat, error: %s", err)
			return nil, err
		}
		curRate, lastRate = 0, 0
		if currentRelease.Total != 0 {
			curRate = float64(currentRelease.Success) / float64(currentRelease.Total) * 100
		}
		if LastRelease.Total != 0 {
			lastRate = float64(LastRelease.Success) / float64(LastRelease.Total) * 100
		}
		releaseSuccessRate := &MonthAttentionData{
			Name:         "release_success_rate",
			CurrentMonth: fmt.Sprintf("%.2f", curRate),
			LastMonth:    fmt.Sprintf("%.2f", lastRate),
		}
		monthAttention.Facts = append(monthAttention.Facts, releaseSuccessRate)

		// get release_frequency
		releaseFrequencyCalculator := &ReleaseFrequencyCalculator{}
		currentReleaseFrequency, _, err := releaseFrequencyCalculator.GetFact(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get release frequency, error: %s", err)
			return nil, err
		}
		LastReleaseFrequency, _, err := releaseFrequencyCalculator.GetFact(LastMonthStart, LastMonthEnd, project)
		if err != nil {
			logger.Errorf("failed to get release frequency, error: %s", err)
			return nil, err
		}
		releaseFrequency := &MonthAttentionData{
			Name:         "release_frequency",
			CurrentMonth: strconv.Itoa(int(currentReleaseFrequency)),
			LastMonth:    strconv.Itoa(int(LastReleaseFrequency)),
		}
		monthAttention.Facts = append(monthAttention.Facts, releaseFrequency)

		// get requirement_development_lead_time type:schedule item_key:requirement_development_lead_time
		currentFact, err := GetRequirementDevelopmentLeadTime(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get requirement development lead time, error: %s", err)
			// TODO: the api is not stable, so we ignore the error
			//return nil, err
		}
		// get requirement_development_lead_time
		LastFact, err := GetRequirementDevelopmentLeadTime(startTime, endTime, project)
		if err != nil {
			logger.Errorf("failed to get requirement development lead time, error: %s", err)
			// TODO: the api is not stable, so we ignore the error
			//return nil, err
		}
		leadTime := &MonthAttentionData{
			Name:         "requirement_development_lead_time",
			CurrentMonth: strconv.Itoa(int(currentFact)),
			LastMonth:    strconv.Itoa(int(LastFact)),
		}
		monthAttention.Facts = append(monthAttention.Facts, leadTime)

		resp = append(resp, monthAttention)
	}
	return resp, nil
}

func getAllProjectNames(projectList []string) ([]string, error) {
	var projects []*templaterepo.ProjectInfo
	var err error
	if len(projectList) != 0 {
		projects, err = templaterepo.NewProductColl().ListProjectBriefs(projectList)
	} else {
		projects, err = templaterepo.NewProductColl().ListNonPMProject()
		if err != nil {
			return nil, e.ErrGetStatisticsDashboard.AddDesc(err.Error())
		}
	}

	resp := make([]string, 0)
	for _, project := range projects {
		resp = append(resp, project.Name)
	}
	return resp, nil
}

func getDestFloat(f float64) float64 {
	return math.Round(f*100) / 100
}

type DevDelPeriod struct {
	RequirementDeliveryLeadTime    float64 `json:"requirement_delivery_lead_time"`
	RequirementDevelopmentLeadTime float64 `json:"requirement_development_lead_time"`
}

func GetRequirementDevDelPeriod(start, end int64, projects []string, logger *zap.SugaredLogger) (*DevDelPeriod, error) {
	projects, err := getAllProjectNames(projects)
	if err != nil {
		logger.Errorf("failed to get all project names, error: %s", err)
		return nil, err
	}

	developSum, devCount, deliverySum, delCount := 0.0, 0, 0.0, 0
	for _, project := range projects {
		// get requirement_delivery_lead_time
		delivery, err := GetRequirementDeliveryLeadTime(start, end, project)
		if err != nil {
			logger.Errorf("failed to get requirement delivery lead time, error: %s", err)
		} else {
			deliverySum += delivery
			delCount++
		}

		// get requirement_development_lead_time
		development, err := GetRequirementDevelopmentLeadTime(start, end, project)
		if err != nil {
			logger.Errorf("failed to get requirement development lead time, error: %s", err)
		} else {
			developSum += development
			devCount++
		}
	}

	resp := &DevDelPeriod{}
	if devCount != 0 {
		resp.RequirementDevelopmentLeadTime = getDestFloat(developSum / float64(devCount))
	}
	if delCount != 0 {
		resp.RequirementDeliveryLeadTime = getDestFloat(deliverySum / float64(delCount))
	}
	return resp, nil
}
