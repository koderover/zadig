package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statorm "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/orm"
)

var (
	ErrAIAssistantStatsUnavailable = errors.New("AI 助手统计数据暂时不可用")
	ErrAIAssistantStatsRange       = errors.New("统计日期范围无效")
	getAIAssistantStatsStore       = statorm.GetAIAssistantStatsStore
)

func reportRange(from, to string, now time.Time) (time.Time, time.Time, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end, start := today, today.AddDate(0, 0, -29)
	var err error
	if from != "" {
		start, err = time.ParseInLocation("2006-01-02", from, now.Location())
		if err != nil {
			return start, end, fmt.Errorf("from 必须是 YYYY-MM-DD")
		}
	}

	if to != "" {
		end, err = time.ParseInLocation("2006-01-02", to, now.Location())
		if err != nil {
			return start, end, fmt.Errorf("to 必须是 YYYY-MM-DD")
		}
	}

	if start.Before(today.AddDate(0, 0, -364)) || end.After(today) || start.After(end) {
		return start, end, fmt.Errorf("报告范围必须在保留的三百六十五天内，且开始日期不晚于结束日期")
	}

	return start, end, nil
}

func estimatedSavedMinutes(items []models.AIAssistantUsageReportRow) float64 {
	var minutes float64
	for _, item := range items {
		baseline := 0.0
		switch item.Key {
		case "workflow_diagnosis", "service_diagnosis":
			baseline = 30
		case "project_onboarding", "service_onboarding":
			baseline = 40
		case "environment_inspection", "efficiency_diagnosis":
			baseline = 15
		}

		saved := baseline*float64(item.Completed) - float64(item.CompletedElapsedMS)/60000
		if saved > 0 {
			minutes += saved
		}
	}

	return minutes
}

type aiAssistantStatsData struct {
	period       models.AIAssistantStatsPeriod
	rows         []models.AIAssistantUsageReportRow
	savedMinutes float64
}

func queryAIAssistantStats(ctx context.Context, kind, fromDate, toDate string) (*aiAssistantStatsData, error) {
	from, to, err := reportRange(fromDate, toDate, time.Now().In(time.Local))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAIAssistantStatsRange, err.Error())
	}
	// 等待连接和查询共用预算，页面并行加载四类报告时由数据库连接池控制并发。
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	store, err := getAIAssistantStatsStore()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIAssistantStatsUnavailable, err)
	}
	rows, err := store.Report(ctx, kind, from, to)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIAssistantStatsUnavailable, err)
	}
	result := &aiAssistantStatsData{
		period: models.AIAssistantStatsPeriod{From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), TimeZone: time.Local.String()},
		rows:   rows,
	}
	if kind == "overview" {
		skills, err := store.Report(ctx, "skill_details", from, to)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrAIAssistantStatsUnavailable, err)
		}
		result.savedMinutes = estimatedSavedMinutes(skills)
	}
	return result, nil
}

func GetAIAssistantStatsOverview(ctx context.Context, from, to string) (*models.AIAssistantStatsOverviewResponse, error) {
	report, err := queryAIAssistantStats(ctx, "overview", from, to)
	if err != nil {
		return nil, err
	}
	result := &models.AIAssistantStatsOverviewResponse{
		AIAssistantStatsPeriod: report.period,
		Estimation:             true,
		EstimatedSavedMinutes:  report.savedMinutes,
	}
	if len(report.rows) > 0 {
		result.Metrics = report.rows[0].AIAssistantStatsMetrics
	}
	if result.Metrics.AdoptionRate != nil {
		days := result.EstimatedSavedMinutes * *result.Metrics.AdoptionRate / 60 / 8
		result.EstimatedSavedPersonDays = &days
	}
	return result, nil
}

func GetAIAssistantStatsTrends(ctx context.Context, from, to string) (*models.AIAssistantStatsTrendsResponse, error) {
	report, err := queryAIAssistantStats(ctx, "trends", from, to)
	if err != nil {
		return nil, err
	}
	items := make([]models.AIAssistantStatsTrendItem, 0, len(report.rows))
	for _, row := range report.rows {
		items = append(items, models.AIAssistantStatsTrendItem{Date: row.Key, AIAssistantStatsMetrics: row.AIAssistantStatsMetrics})
	}
	return &models.AIAssistantStatsTrendsResponse{AIAssistantStatsPeriod: report.period, Items: items}, nil
}

func GetAIAssistantStatsSkills(ctx context.Context, from, to string) (*models.AIAssistantStatsSkillsResponse, error) {
	report, err := queryAIAssistantStats(ctx, "skills", from, to)
	if err != nil {
		return nil, err
	}
	items := []models.AIAssistantStatsSkillItem{
		{Category: "发布辅助"},
		{Category: "故障排查"},
		{Category: "实施辅助"},
		{Category: "巡检诊断"},
	}
	for i := range items {
		for _, row := range report.rows {
			if row.Key == items[i].Category {
				items[i].AIAssistantStatsMetrics = row.AIAssistantStatsMetrics
				break
			}
		}
	}
	return &models.AIAssistantStatsSkillsResponse{AIAssistantStatsPeriod: report.period, Items: items}, nil
}

func GetAIAssistantStatsRankings(ctx context.Context, from, to string) (*models.AIAssistantStatsRankingsResponse, error) {
	report, err := queryAIAssistantStats(ctx, "rankings", from, to)
	if err != nil {
		return nil, err
	}
	items := make([]models.AIAssistantStatsRankingItem, 0, len(report.rows))
	for _, row := range report.rows {
		items = append(items, models.AIAssistantStatsRankingItem{UserID: row.Key, AIAssistantStatsMetrics: row.AIAssistantStatsMetrics})
	}
	return &models.AIAssistantStatsRankingsResponse{AIAssistantStatsPeriod: report.period, Items: items}, nil
}
