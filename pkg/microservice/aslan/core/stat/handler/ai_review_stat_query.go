package handler

import (
	"context"
	"encoding/json"
	"fmt"

	statmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type aiReviewPRKey struct {
	CodehostID  int
	Owner, Name string
	PR          int
}
type aiReviewRepoKey struct {
	CodehostID  int
	Owner, Name string
}
type aiReviewQueryScope struct {
	Project     string
	CodehostID  int
	Owner, Name string
}

func aiReviewKey(record statmodels.AIReviewStat) aiReviewPRKey {
	return aiReviewPRKey{record.CodehostID, record.RepoOwner, record.RepoName, record.PR}
}

func addAIReviewFinding(metrics *AIReviewStatsMetrics, severity map[string]int64, types map[string]AIReviewStatsDistributionItem, finding statmodels.AIReviewStatFinding) {
	metrics.FindingTotal++
	if finding.Severity != "" {
		severity[finding.Severity]++
	}
	if finding.Category != "" {
		item := types[finding.Category]
		item.ID, item.Name = finding.Category, finding.CategoryName
		if item.Name == "" {
			item.Name = item.ID
		}
		item.Count++
		types[finding.Category] = item
	}
}

func aiReviewRatio(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}

func aiReviewRelative(now, previous int64) *float64 {
	if previous == 0 {
		return nil
	}
	value := float64(now-previous) / float64(previous)
	return &value
}

func aiReviewDifference(now, previous *float64, multiplier float64) *float64 {
	if now == nil || previous == nil {
		return nil
	}
	value := (*now - *previous) * multiplier
	return &value
}

func queryAIReviewOverview(ctx context.Context, args *AIReviewStatsOverviewRequest, scope AIReviewStatsScope) (AIReviewStatsOverviewResponse, error) {
	if args.EndTime-args.StartTime > 366*24*60*60 {
		return AIReviewStatsOverviewResponse{}, e.ErrInvalidParam.AddErr(fmt.Errorf("overview time range must not exceed 366 days"))
	}
	if err := ctx.Err(); err != nil {
		return AIReviewStatsOverviewResponse{}, err
	}
	filter := aiReviewQueryScope{Project: args.ProjectName, Owner: args.RepoOwner, Name: args.RepoName}
	if args.CodehostID != nil {
		filter.CodehostID = *args.CodehostID
	}
	current, err := loadAIReviewOverview(ctx, args.StartTime, args.EndTime, filter)
	if err != nil {
		return AIReviewStatsOverviewResponse{}, err
	}
	period := args.EndTime - args.StartTime
	prior, err := loadAIReviewComparisonMetrics(ctx, args.StartTime-period, args.StartTime, filter)
	if err != nil {
		return AIReviewStatsOverviewResponse{}, err
	}
	metrics, distributions, repoCount, savedPersonHours, weeklyTrend := current.result()
	scope.RepoCount = repoCount
	if scope.Type == "project" || scope.Type == "repo" {
		scope.ProjectDisplayName = scope.ProjectName
		if current.projectDisplayName != "" {
			scope.ProjectDisplayName = current.projectDisplayName
		}
	}
	if scope.Type == "repo" {
		scope.RepoDisplayName = scope.RepoName
	}
	var savedHoursChange *float64
	if prior.SavedPersonHours > 0 {
		change := (savedPersonHours - prior.SavedPersonHours) / prior.SavedPersonHours
		savedHoursChange = &change
	}
	response := AIReviewStatsOverviewResponse{Scope: scope, Metrics: AIReviewStatsOverviewMetrics{AIReviewStatsMetrics: metrics, SavedPersonHours: savedPersonHours}, AIReviewStatsDistributions: distributions,
		Comparison:  AIReviewStatsComparison{SavedPersonHours: savedHoursChange, PRCount: aiReviewRelative(metrics.PRCount, prior.PRCount), ResolutionRate: aiReviewDifference(metrics.ResolutionRate, prior.ResolutionRate, 100), UpDownRatio: aiReviewDifference(metrics.UpDownRatio, prior.UpDownRatio, 1), TotalTokens: aiReviewRelative(metrics.TotalTokens, prior.TotalTokens)},
		WeeklyTrend: weeklyTrend,
	}
	return response, nil
}

func queryAIReviewProjects(ctx context.Context, args *AIReviewStatsListRequest) (AIReviewStatsProjectListResponse, error) {
	rows, total, err := loadAIReviewPage(ctx, args.StartTime, args.EndTime, aiReviewQueryScope{}, "project", *args)
	response := AIReviewStatsProjectListResponse{AIReviewStatsPage: AIReviewStatsPage{Total: total, Page: args.Page, PageSize: args.PageSize}, Items: []AIReviewStatsProjectItem{}}
	if err != nil {
		return response, err
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(data, &response.Items)
	return response, err
}

func queryAIReviewRepos(ctx context.Context, args *AIReviewStatsRepoListRequest) (AIReviewStatsRepoListResponse, error) {
	rows, total, err := loadAIReviewPage(ctx, args.StartTime, args.EndTime, aiReviewQueryScope{Project: args.ProjectName}, "repo", args.AIReviewStatsListRequest)
	response := AIReviewStatsRepoListResponse{AIReviewStatsPage: AIReviewStatsPage{Total: total, Page: args.Page, PageSize: args.PageSize}, Items: []AIReviewStatsRepoItem{}}
	if err != nil {
		return response, err
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(data, &response.Items)
	return response, err
}

func queryAIReviewPRs(ctx context.Context, args *AIReviewStatsRepoPRListRequest, repoName string) (AIReviewStatsPRListResponse, error) {
	rows, total, err := loadAIReviewPage(ctx, args.StartTime, args.EndTime, aiReviewQueryScope{Project: args.ProjectName, CodehostID: args.CodehostID, Owner: args.RepoOwner, Name: repoName}, "pr", AIReviewStatsListRequest{AIReviewStatsPagination: args.AIReviewStatsPagination})
	response := AIReviewStatsPRListResponse{AIReviewStatsPage: AIReviewStatsPage{Total: total, Page: args.Page, PageSize: args.PageSize}, Items: []AIReviewStatsPRItem{}}
	if err != nil {
		return response, err
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(data, &response.Items)
	return response, err
}
