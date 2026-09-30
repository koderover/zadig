package handler

import (
	"context"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	feedbackmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	feedbackrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	statmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
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
	match := bson.M{"reviewed_at": bson.M{"$gte": start, "$lt": end}}
	if scope.Project != "" {
		match["project_name"] = scope.Project
	}
	if scope.CodehostID > 0 {
		match["codehost_id"], match["repo_owner"], match["repo_name"] = scope.CodehostID, scope.Owner, scope.Name
	}
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
		metrics.UpDownRatio = aiReviewRatio(metrics.Up, metrics.Down)
		metrics.ApprovalRate = aiReviewRatio(metrics.Up, metrics.Up+metrics.Down)
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
	records, err := loadAIReviewStats(ctx, args.StartTime, args.EndTime, aiReviewQueryScope{})
	if err != nil {
		return AIReviewStatsProjectListResponse{}, err
	}
	feedback, err := loadAIReviewFeedback(ctx, records)
	if err != nil {
		return AIReviewStatsProjectListResponse{}, err
	}
	groups := map[string][]statmodels.AIReviewStat{}
	for _, record := range records {
		groups[record.ProjectName] = append(groups[record.ProjectName], record)
	}
	items := make([]AIReviewStatsProjectItem, 0, len(groups))
	for name, group := range groups {
		metrics, distribution, count := (aiReviewAggregate{group, feedback}).summarize()
		display := name
		for _, record := range group {
			if record.ProjectDisplayName != "" {
				display = record.ProjectDisplayName
				break
			}
		}
		items = append(items, AIReviewStatsProjectItem{ProjectName: name, ProjectDisplayName: display, RepoCount: count, AIReviewStatsMetrics: metrics, AIReviewStatsDistributions: distribution})
	}
	sort.Slice(items, func(i, j int) bool {
		return aiReviewLess(items[i].AIReviewStatsMetrics, items[j].AIReviewStatsMetrics, items[i].ProjectName, items[j].ProjectName, args.SortBy, args.SortOrder)
	})
	total := int64(len(items))
	items = aiReviewPage(items, args.Page, args.PageSize)
	return AIReviewStatsProjectListResponse{AIReviewStatsPage: AIReviewStatsPage{Total: total, Page: args.Page, PageSize: args.PageSize}, Items: items}, nil
}

func queryAIReviewRepos(ctx context.Context, args *AIReviewStatsRepoListRequest) (AIReviewStatsRepoListResponse, error) {
	records, err := loadAIReviewStats(ctx, args.StartTime, args.EndTime, aiReviewQueryScope{Project: args.ProjectName})
	if err != nil {
		return AIReviewStatsRepoListResponse{}, err
	}
	feedback, err := loadAIReviewFeedback(ctx, records)
	if err != nil {
		return AIReviewStatsRepoListResponse{}, err
	}
	groups := map[aiReviewRepoKey][]statmodels.AIReviewStat{}
	for _, record := range records {
		key := aiReviewRepoKey{record.CodehostID, record.RepoOwner, record.RepoName}
		groups[key] = append(groups[key], record)
	}
	items := make([]AIReviewStatsRepoItem, 0, len(groups))
	for key, group := range groups {
		metrics, distribution, _ := (aiReviewAggregate{group, feedback}).summarize()
		items = append(items, AIReviewStatsRepoItem{CodehostID: key.CodehostID, RepoOwner: key.Owner, RepoName: key.Name, RepoDisplayName: key.Name, AIReviewStatsMetrics: metrics, AIReviewStatsDistributions: distribution})
	}
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		return aiReviewLess(left.AIReviewStatsMetrics, right.AIReviewStatsMetrics, fmt.Sprintf("%d/%s/%s", left.CodehostID, left.RepoOwner, left.RepoName), fmt.Sprintf("%d/%s/%s", right.CodehostID, right.RepoOwner, right.RepoName), args.SortBy, args.SortOrder)
	})
	total := int64(len(items))
	items = aiReviewPage(items, args.Page, args.PageSize)
	return AIReviewStatsRepoListResponse{AIReviewStatsPage: AIReviewStatsPage{Total: total, Page: args.Page, PageSize: args.PageSize}, Items: items}, nil
}

func queryAIReviewPRs(ctx context.Context, args *AIReviewStatsRepoPRListRequest, repoName string) (AIReviewStatsPRListResponse, error) {
	records, err := loadAIReviewStats(ctx, args.StartTime, args.EndTime, aiReviewQueryScope{Project: args.ProjectName, CodehostID: args.CodehostID, Owner: args.RepoOwner, Name: repoName})
	if err != nil {
		return AIReviewStatsPRListResponse{}, err
	}
	feedback, err := loadAIReviewFeedback(ctx, records)
	if err != nil {
		return AIReviewStatsPRListResponse{}, err
	}
	groups := map[aiReviewPRKey][]statmodels.AIReviewStat{}
	for _, record := range records {
		groups[aiReviewKey(record)] = append(groups[aiReviewKey(record)], record)
	}
	items := make([]AIReviewStatsPRItem, 0, len(groups))
	for key, group := range groups {
		sort.Slice(group, func(i, j int) bool { return group[i].ReviewedAt < group[j].ReviewedAt })
		last := group[len(group)-1]
		metrics, distribution, _ := (aiReviewAggregate{group, feedback}).summarize()
		item := AIReviewStatsPRItem{CodehostID: key.CodehostID, RepoOwner: key.Owner, RepoName: key.Name, PR: key.PR, PRTitle: last.PRTitle, PRAuthor: last.PRAuthor, PRURL: last.PRURL, ReviewedAt: last.ReviewedAt, AIReviewStatsMetrics: metrics, AIReviewStatsDistributions: distribution}
		if state := feedback[key]; state != nil {
			if state.PRTitle != "" {
				item.PRTitle = state.PRTitle
			}
			if !state.ResolutionSyncedAt.IsZero() {
				synced := state.ResolutionSyncedAt.Unix()
				item.ResolutionSyncedAt = &synced
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ReviewedAt != items[j].ReviewedAt {
			return items[i].ReviewedAt > items[j].ReviewedAt
		}
		return items[i].PR < items[j].PR
	})
	total := int64(len(items))
	items = aiReviewPage(items, args.Page, args.PageSize)
	return AIReviewStatsPRListResponse{AIReviewStatsPage: AIReviewStatsPage{Total: total, Page: args.Page, PageSize: args.PageSize}, Items: items}, nil
}

func aiReviewLess(left, right AIReviewStatsMetrics, leftKey, rightKey, by, order string) bool {
	var l, r float64
	var lp, rp *float64
	switch by {
	case "inline_total":
		l, r = float64(left.InlineTotal), float64(right.InlineTotal)
	case "resolution_rate":
		lp, rp = left.ResolutionRate, right.ResolutionRate
	case "up_down_ratio":
		lp, rp = left.UpDownRatio, right.UpDownRatio
	case "total_tokens":
		l, r = float64(left.TotalTokens), float64(right.TotalTokens)
	default:
		l, r = float64(left.PRCount), float64(right.PRCount)
	}
	if lp != nil || rp != nil {
		if lp == nil {
			return false
		}
		if rp == nil {
			return true
		}
		l, r = *lp, *rp
	}
	if l == r {
		return leftKey < rightKey
	}
	if order == "asc" {
		return l < r
	}
	return l > r
}

func aiReviewPage[T any](items []T, page, size int) []T {
	if page-1 > len(items)/size {
		return []T{}
	}
	start := (page - 1) * size
	if start >= len(items) {
		return []T{}
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
