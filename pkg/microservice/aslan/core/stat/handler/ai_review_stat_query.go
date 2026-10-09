package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	feedbackmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	feedbackrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	statmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
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

func loadAIReviewStats(ctx context.Context, start, end int64, scope aiReviewQueryScope) ([]statmodels.AIReviewStat, error) {
	match := aiReviewStatMatch(start, end, scope)
	cursor, err := statrepo.NewAIReviewStatColl().Find(ctx, match, options.Find().SetSort(bson.D{{Key: "reviewed_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	result := make([]statmodels.AIReviewStat, 0)
	for cursor.Next(ctx) {
		var record statmodels.AIReviewStat
		if err := cursor.Decode(&record); err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, cursor.Err()
}

func loadAIReviewFeedback(ctx context.Context, sets ...[]statmodels.AIReviewStat) (map[aiReviewPRKey]*feedbackmodels.AIReviewFeedback, error) {
	keys := make(map[aiReviewPRKey]bool)
	for _, records := range sets {
		for _, record := range records {
			keys[aiReviewKey(record)] = true
		}
	}
	result := make(map[aiReviewPRKey]*feedbackmodels.AIReviewFeedback, len(keys))
	clauses := make(bson.A, 0, 100)
	flush := func() error {
		if len(clauses) == 0 {
			return nil
		}
		cursor, err := feedbackrepo.NewAIReviewFeedbackColl().Find(ctx, bson.M{"$or": clauses}, options.Find().SetProjection(bson.M{
			"codehost_id": 1, "repo_owner": 1, "repo_name": 1, "pr": 1, "pr_title": 1,
			"up": 1, "down": 1, "inline_total": 1, "inline_resolved": 1, "resolution_synced_at": 1,
			"provider": 1, "synced_at": 1,
		}))
		if err != nil {
			return err
		}
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var feedback feedbackmodels.AIReviewFeedback
			if err := cursor.Decode(&feedback); err != nil {
				return err
			}
			result[aiReviewPRKey{feedback.CodehostID, feedback.RepoOwner, feedback.RepoName, feedback.PR}] = &feedback
		}
		clauses = clauses[:0]
		return cursor.Err()
	}
	for key := range keys {
		clauses = append(clauses, bson.M{"codehost_id": key.CodehostID, "repo_owner": key.Owner, "repo_name": key.Name, "pr": key.PR})
		if len(clauses) == 100 {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return result, nil
}

type aiReviewAggregate struct {
	records  []statmodels.AIReviewStat
	feedback map[aiReviewPRKey]*feedbackmodels.AIReviewFeedback
}

func (a aiReviewAggregate) summarize() (AIReviewStatsMetrics, AIReviewStatsDistributions, int64) {
	metrics := AIReviewStatsMetrics{ModelUsage: []AIReviewStatsModelUsage{}}
	distributions := AIReviewStatsDistributions{Severities: []AIReviewStatsDistributionItem{}, ProblemTypes: []AIReviewStatsDistributionItem{}}
	prs := make(map[aiReviewPRKey]bool)
	repos := make(map[aiReviewRepoKey]bool)
	fingerprints := make(map[aiReviewPRKey]map[string]statmodels.AIReviewStatFinding)
	severity := make(map[string]int64)
	types := make(map[string]AIReviewStatsDistributionItem)
	models := make(map[string]AIReviewStatsModelUsage)
	for _, record := range a.records {
		key := aiReviewKey(record)
		prs[key] = true
		repos[aiReviewRepoKey{record.CodehostID, record.RepoOwner, record.RepoName}] = true
		metrics.PromptTokens += record.Usage.PromptTokens
		metrics.CompletionTokens += record.Usage.CompletionTokens
		metrics.TotalTokens += record.Usage.TotalTokens
		usage := models[record.Model]
		usage.Model = record.Model
		usage.PromptTokens += record.Usage.PromptTokens
		usage.CompletionTokens += record.Usage.CompletionTokens
		usage.TotalTokens += record.Usage.TotalTokens
		models[record.Model] = usage
		if fingerprints[key] == nil {
			fingerprints[key] = make(map[string]statmodels.AIReviewStatFinding)
		}
		for _, finding := range record.Findings {
			if finding.Fingerprint == "" {
				addAIReviewFinding(&metrics, severity, types, finding)
				continue
			}
			// Review records are sorted oldest first; the latest occurrence wins.
			fingerprints[key][finding.Fingerprint] = finding
		}
	}
	for _, byFingerprint := range fingerprints {
		for _, finding := range byFingerprint {
			addAIReviewFinding(&metrics, severity, types, finding)
		}
	}
	metrics.PRCount = int64(len(prs))
	feedbackKnown, resolutionKnown := true, true
	for key := range prs {
		feedback := a.feedback[key]
		if feedback == nil {
			feedbackKnown, resolutionKnown = false, false
			continue
		}
		if feedback.Provider == "" || (feedback.Provider == setting.SourceFromGithub && feedback.SyncedAt.IsZero()) {
			feedbackKnown = false
		}
		metrics.Up += int64(feedback.Up)
		metrics.Down += int64(feedback.Down)
		metrics.InlineTotal += int64(feedback.InlineTotal)
		metrics.InlineResolved += int64(feedback.InlineResolved)
		if feedback.ResolutionSyncedAt.IsZero() && feedback.InlineTotal > 0 {
			resolutionKnown = false
		} else {
			metrics.InlineUnresolved += int64(max(0, feedback.InlineTotal-feedback.InlineResolved))
		}
	}
	if resolutionKnown && metrics.InlineTotal > 0 {
		metrics.ResolutionRate = aiReviewRatio(metrics.InlineResolved, metrics.InlineTotal)
	}
	if feedbackKnown {
		ratio := math.Round(float64(metrics.Up)/float64(max(1, metrics.Down))*10) / 10
		metrics.UpDownRatio = &ratio
		metrics.ApprovalRate = aiReviewRatio(metrics.Up, max(1, metrics.Up+metrics.Down))
	}
	for _, usage := range models {
		metrics.ModelUsage = append(metrics.ModelUsage, usage)
	}
	sort.Slice(metrics.ModelUsage, func(i, j int) bool { return metrics.ModelUsage[i].Model < metrics.ModelUsage[j].Model })
	for id, count := range severity {
		distributions.Severities = append(distributions.Severities, AIReviewStatsDistributionItem{ID: id, Name: id, Count: count})
	}
	for _, item := range types {
		distributions.ProblemTypes = append(distributions.ProblemTypes, item)
	}
	sort.Slice(distributions.Severities, func(i, j int) bool { return distributions.Severities[i].ID < distributions.Severities[j].ID })
	sort.Slice(distributions.ProblemTypes, func(i, j int) bool { return distributions.ProblemTypes[i].ID < distributions.ProblemTypes[j].ID })
	return metrics, distributions, int64(len(repos))
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
	current, err := loadAIReviewStats(ctx, args.StartTime, args.EndTime, filter)
	if err != nil {
		return AIReviewStatsOverviewResponse{}, err
	}
	period := args.EndTime - args.StartTime
	previous, err := loadAIReviewStats(ctx, args.StartTime-period, args.StartTime, filter)
	if err != nil {
		return AIReviewStatsOverviewResponse{}, err
	}
	feedback, err := loadAIReviewFeedback(ctx, current, previous)
	if err != nil {
		return AIReviewStatsOverviewResponse{}, err
	}
	metrics, distributions, repoCount := (aiReviewAggregate{current, feedback}).summarize()
	prior, _, _ := (aiReviewAggregate{previous, feedback}).summarize()
	scope.RepoCount = repoCount
	if scope.Type == "project" || scope.Type == "repo" {
		scope.ProjectDisplayName = scope.ProjectName
		for _, record := range current {
			if record.ProjectDisplayName != "" {
				scope.ProjectDisplayName = record.ProjectDisplayName
				break
			}
		}
	}
	if scope.Type == "repo" {
		scope.RepoDisplayName = scope.RepoName
	}
	response := AIReviewStatsOverviewResponse{Scope: scope, Metrics: metrics, AIReviewStatsDistributions: distributions,
		Comparison:  AIReviewStatsComparison{PRCount: aiReviewRelative(metrics.PRCount, prior.PRCount), ResolutionRate: aiReviewDifference(metrics.ResolutionRate, prior.ResolutionRate, 100), UpDownRatio: aiReviewDifference(metrics.UpDownRatio, prior.UpDownRatio, 1), TotalTokens: aiReviewRelative(metrics.TotalTokens, prior.TotalTokens)},
		WeeklyTrend: []AIReviewStatsWeeklyTrend{},
	}
	const week = int64(7 * 24 * 60 * 60)
	for start := args.StartTime; start < args.EndTime; {
		if err := ctx.Err(); err != nil {
			return AIReviewStatsOverviewResponse{}, err
		}
		end := start + week
		if end < start || end > args.EndTime {
			end = args.EndTime
		}
		bucket := make([]statmodels.AIReviewStat, 0)
		for _, record := range current {
			if record.ReviewedAt >= start && record.ReviewedAt < end {
				bucket = append(bucket, record)
			}
		}
		item, _, _ := (aiReviewAggregate{bucket, feedback}).summarize()
		response.WeeklyTrend = append(response.WeeklyTrend, AIReviewStatsWeeklyTrend{StartTime: start, EndTime: end, PRCount: item.PRCount, InlineTotal: item.InlineTotal, InlineResolved: item.InlineResolved, ResolutionRate: item.ResolutionRate})
		start = end
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
