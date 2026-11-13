package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statorm "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/orm"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
)

var (
	ErrAIAssistantStatsUnavailable = errors.New("AI 助手统计数据暂时不可用")
	ErrAIAssistantStatsRange       = errors.New("统计日期范围无效")
	getAIAssistantStatsStore       = statorm.GetAIAssistantStatsStore
	newAIAssistantStatsUserClient  = user.New
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

// 上期按自然日计算，避免夏令时切换导致周期少一天。
func previousStatsRange(period models.AIAssistantStatsPeriod) (string, string) {
	from, _ := time.Parse("2006-01-02", period.From)
	to, _ := time.Parse("2006-01-02", period.To)
	days := int(to.Sub(from)/(24*time.Hour)) + 1
	return from.AddDate(0, 0, -days).Format("2006-01-02"), from.AddDate(0, 0, -1).Format("2006-01-02")
}

func statsUserClient(ctx context.Context) *user.Client {
	client := newAIAssistantStatsUserClient()
	client.SetTimeout(3 * time.Second)
	client.OnBeforeRequest(func(_ *resty.Client, request *resty.Request) error {
		request.SetContext(ctx)
		return nil
	})
	return client
}

func assistantOverviewData(report *aiAssistantStatsData) models.AIAssistantStatsOverviewData {
	result := models.AIAssistantStatsOverviewData{
		AIAssistantStatsPeriod: report.period,
		Estimation:             true,
		EstimatedSavedMinutes:  report.savedMinutes,
		Metrics:                models.AIAssistantStatsMetrics{ModelUsage: []models.AIAssistantStatsModelUsage{}},
	}
	if len(report.rows) > 0 {
		result.Metrics = report.rows[0].AIAssistantStatsMetrics
	}
	if result.Metrics.AdoptionRate != nil {
		days := result.EstimatedSavedMinutes * *result.Metrics.AdoptionRate / 60 / 8
		result.EstimatedSavedPersonDays = &days
	}
	return result
}

func GetAIAssistantStatsOverview(ctx context.Context, from, to string) (*models.AIAssistantStatsOverviewResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	report, err := queryAIAssistantStats(ctx, "overview", from, to)
	if err != nil {
		return nil, err
	}
	result := &models.AIAssistantStatsOverviewResponse{AIAssistantStatsOverviewData: assistantOverviewData(report)}
	previousFrom, previousTo := previousStatsRange(report.period)
	// 上期超出完整保留范围时返回 null，不能把已清理的数据当成零使用量。
	if _, _, err := reportRange(previousFrom, previousTo, time.Now().In(time.Local)); err == nil {
		previous, err := queryAIAssistantStats(ctx, "overview", previousFrom, previousTo)
		if err != nil {
			return nil, err
		}
		data := assistantOverviewData(previous)
		result.Previous = &data
	}
	count, err := statsUserClient(ctx).CountUsers()
	if err != nil {
		return nil, fmt.Errorf("%w: 查询系统用户总数失败: %v", ErrAIAssistantStatsUnavailable, err)
	}
	result.TotalUsers = count.TotalUser
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
		items[i].ModelUsage = []models.AIAssistantStatsModelUsage{}
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
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	report, err := queryAIAssistantStats(ctx, "rankings", from, to)
	if err != nil {
		return nil, err
	}
	items := make([]models.AIAssistantStatsRankingItem, 0, len(report.rows))
	for _, row := range report.rows {
		items = append(items, models.AIAssistantStatsRankingItem{UserID: row.Key, UserName: row.Key, AIAssistantStatsMetrics: row.AIAssistantStatsMetrics})
	}
	if len(items) > 0 {
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.UserID)
		}
		users, err := statsUserClient(ctx).ListUsers(&user.SearchArgs{UIDs: ids})
		if err != nil {
			return nil, fmt.Errorf("%w: 查询排行用户名失败: %v", ErrAIAssistantStatsUnavailable, err)
		}
		names := make(map[string]string, len(users))
		for _, user := range users {
			if user != nil && user.Name != "" {
				names[user.UID] = user.Name
			}
		}
		for i := range items {
			if name := names[items[i].UserID]; name != "" {
				items[i].UserName = name
			}
		}
	}
	return &models.AIAssistantStatsRankingsResponse{AIAssistantStatsPeriod: report.period, Items: items}, nil
}
