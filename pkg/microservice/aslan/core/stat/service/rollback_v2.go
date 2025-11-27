/*
Copyright 2024 The KodeRover Authors.

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
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/util"
)

// CreateWeeklyDeployStat creates deploy stats for both testing and production envs. Note that this function MUST be
// called in Monday otherwise it will CREATE STATS FOR INVALID DATE MAKING THE WHOLE STATS UNUSABLE.
func CreateWeeklyRollbackStat(log *zap.SugaredLogger) error {
	log.Info("start creating weekly rollback stats..")

	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		err = fmt.Errorf("failed to list project list to create rollback stats, error: %s", err)
		log.Error(err)
		return err
	}

	failedProjects := make([]string, 0)
	for _, project := range projects {
		testRollbackStat, productionRollbackStat, err := generateWeeklyRollbackStatByProduct(project.ProductName, log)
		if err != nil {
			log.Errorf("failed to generate weekly rollback stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			failedProjects = append(failedProjects, project.ProductName)
			continue
		}

		inserted := false
		err = mongodb.NewWeeklyRollbackStatColl().Upsert(testRollbackStat)
		if err != nil {
			log.Errorf("failed to insert weekly rollback stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			failedProjects = append(failedProjects, project.ProductName)
			inserted = true
		}

		err = mongodb.NewWeeklyRollbackStatColl().Upsert(productionRollbackStat)
		if err != nil {
			log.Errorf("failed to insert weekly production deploy stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			if !inserted {
				failedProjects = append(failedProjects, project.ProductName)
			}
		}
	}

	if len(failedProjects) > 0 {
		err = fmt.Errorf("failed to do full rollback stats, failed projects: %s", strings.Join(failedProjects, ", "))
		log.Error(err)
		return err
	}

	return nil
}

func GetRollbackTotalStat(startTime, endTime int64, projects []string, production config.ProductionType, log *zap.SugaredLogger) (*RollbackTotalStat, error) {
	_, total, err := commonrepo.NewEnvInfoColl().List(context.Background(), &commonrepo.ListEnvInfoOption{
		ProjectNames: projects,
		Operation:    config.EnvOperationRollback,
		StartTime:    startTime,
		EndTime:      endTime,
		Production:   production.ToBool(),
	})
	if err != nil {
		err = fmt.Errorf("failed to list env info for projects: %v, error: %s", projects, err)
		log.Error(err)
		return nil, err
	}

	resp := &RollbackTotalStat{
		Total: total,
	}

	return resp, nil
}

func GetTopRollbackedProject(startTime, endTime int64, top int, production config.ProductionType, projects []string, log *zap.SugaredLogger) ([]*commonrepo.RollbackServiceCount, error) {
	rollbackProjects, err := commonrepo.NewEnvInfoColl().GetTopRollbackedService(context.Background(), startTime, endTime, production, projects, top)
	if err != nil {
		err = fmt.Errorf("failed to get top rollbacked projects: %s, error: %s", production, err)
		log.Error(err)
		return nil, err
	}

	return rollbackProjects, nil
}

type RollbackImageTag struct {
	ImageName string `json:"image_name"`
	ImageTag  string `json:"image_tag"`
}

type RollbackStat struct {
	ProjectName            string              `json:"project_name"`
	ServiceName            string              `json:"service_name"`
	EnvName                string              `json:"env_name"`
	RollbackTime           int64               `json:"rollback_time"`
	RollbackDetail         string              `json:"rollback_detail"`
	RollbackBy             string              `json:"rollback_by"`
	RollbackBeforeImageTag []*RollbackImageTag `json:"rollback_before_image_tag"`
	RollbackAfterImageTag  []*RollbackImageTag `json:"rollback_after_image_tag"`
}

type GetRollbackStatResponse struct {
	Total int64           `json:"total"`
	Data  []*RollbackStat `json:"data"`
}

func GetRollbackStat(startTime, endTime int64, production config.ProductionType, projects []string, log *zap.SugaredLogger) (*GetRollbackStatResponse, error) {
	envInfos, total, err := commonrepo.NewEnvInfoColl().List(context.Background(), &commonrepo.ListEnvInfoOption{
		ProjectNames: projects,
		Operation:    config.EnvOperationRollback,
		StartTime:    startTime,
		EndTime:      endTime,
		Production:   production.ToBool(),
	})
	if err != nil {
		err = fmt.Errorf("failed to list env info for projects: %v, error: %s", projects, err)
		log.Error(err)
		return nil, err
	}

	rollbackStats := make([]*RollbackStat, 0)
	for _, envInfo := range envInfos {
		// 提取回滚前的镜像信息
		rollbackBeforeImageTags := extractImageTags(envInfo.OriginService)

		// 提取回滚后的镜像信息
		rollbackAfterImageTags := extractImageTags(envInfo.UpdateService)

		// 获取回滚操作者
		rollbackBy := ""
		if envInfo.CreatedBy.Name != "" {
			rollbackBy = envInfo.CreatedBy.Name
		} else if envInfo.CreatedBy.Account != "" {
			rollbackBy = envInfo.CreatedBy.Account
		}

		rollbackStats = append(rollbackStats, &RollbackStat{
			ProjectName:            envInfo.ProjectName,
			ServiceName:            envInfo.ServiceName,
			EnvName:                envInfo.EnvName,
			RollbackTime:           envInfo.CreatTime,
			RollbackDetail:         envInfo.Detail,
			RollbackBy:             rollbackBy,
			RollbackBeforeImageTag: rollbackBeforeImageTags,
			RollbackAfterImageTag:  rollbackAfterImageTags,
		})
	}

	return &GetRollbackStatResponse{
		Total: total,
		Data:  rollbackStats,
	}, nil
}

// extractImageTags 从服务中提取所有容器的镜像信息
func extractImageTags(service *commonmodels.ProductService) []*RollbackImageTag {
	if service == nil || len(service.Containers) == 0 {
		return nil
	}

	imageTags := make([]*RollbackImageTag, 0)
	for _, container := range service.Containers {
		if container.Image == "" {
			continue
		}

		imageTag := commonutil.ExtractImageTag(container.Image)
		imageTags = append(imageTags, &RollbackImageTag{
			ImageName: container.ImageName,
			ImageTag:  imageTag,
		})
	}

	return imageTags
}

func GetRollbackWeeklyTrend(startTime, endTime int64, projects []string, production config.ProductionType, log *zap.SugaredLogger) ([]*models.WeeklyRollbackStat, error) {
	// first get weekly stats
	weeklystats, err := mongodb.NewWeeklyRollbackStatColl().CalculateStat(startTime, endTime, projects, production)
	if err != nil {
		err = fmt.Errorf("failed to get weekly rollback trend, error: %s", err)
		log.Error(err)
		return nil, err
	}

	// then calculate the start time of this week, append it to the end of the array
	firstDayOfWeek := util.GetMonday(time.Now())
	thisWeekRollbackStat, _, err := commonrepo.NewEnvInfoColl().List(context.Background(), &commonrepo.ListEnvInfoOption{
		ProjectNames: projects,
		Operation:    config.EnvOperationRollback,
		StartTime:    firstDayOfWeek.Unix(),
		EndTime:      time.Now().Unix(),
		Production:   production.ToBool(),
	})
	if err != nil {
		err = fmt.Errorf("failed to list rollback stats for weekly trend, error: %s", err)
		log.Error(err)
		return nil, err
	}

	date := firstDayOfWeek.Format(config.Date)
	rollbackCount := len(thisWeekRollbackStat)
	weeklystats = append(weeklystats, &models.WeeklyRollbackStat{
		Rollback: rollbackCount,
		Date:     date,
	})

	return weeklystats, nil
}

// generateWeeklyRollbackStatByProduct generates the rollback stats for the week before the calling time.
// (this should be called on monday)
func generateWeeklyRollbackStatByProduct(projectKey string, log *zap.SugaredLogger) (testRollbackStat, productionRollbackStat *models.WeeklyRollbackStat, retErr error) {
	startTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -7)
	endTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, time.Local).AddDate(0, 0, -1)

	allRollbacks, _, err := commonrepo.NewEnvInfoColl().List(context.Background(), &commonrepo.ListEnvInfoOption{
		ProjectNames: []string{projectKey},
		Operation:    config.EnvOperationRollback,
		StartTime:    startTime.Unix(),
		EndTime:      endTime.Unix(),
	})
	if err != nil {
		err = fmt.Errorf("failed to list rollback stat for product: %s, error: %s", projectKey, err)
		log.Error(err)
		return nil, nil, err
	}

	var (
		testRollback       = 0
		productionRollback = 0
	)

	// count the data for both production job
	for _, rollbackJob := range allRollbacks {
		if rollbackJob.Production {
			productionRollback++
		} else {
			testRollback++
		}
	}

	date := startTime.Format(config.Date)

	testRollbackStat = &models.WeeklyRollbackStat{
		ProjectKey: projectKey,
		Production: false,
		Rollback:   testRollback,
		Date:       date,
		CreateTime: time.Now().Unix(),
	}

	productionRollbackStat = &models.WeeklyRollbackStat{
		ProjectKey: projectKey,
		Production: true,
		Rollback:   productionRollback,
		Date:       date,
		CreateTime: time.Now().Unix(),
	}

	return
}
