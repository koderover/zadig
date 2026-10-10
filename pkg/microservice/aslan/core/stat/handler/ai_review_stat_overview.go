package handler

import (
	"context"
	"math"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	feedbackmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	statmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func aiReviewOverviewPipeline(start, end int64, scope aiReviewQueryScope) mongo.Pipeline {
	projection := bson.M{"_id": 0, "findings.severity": 1, "findings.category": 1, "findings.category_name": 1}
	for _, field := range []string{"project_display_name", "codehost_id", "repo_owner", "repo_name", "pr", "reviewed_at", "model", "usage", "duration_ms", "additions", "deletions", "changed_lines"} {
		projection[field] = 1
	}
	return mongo.Pipeline{
		{{Key: "$match", Value: aiReviewStatMatch(start, end, scope)}},
		{{Key: "$sort", Value: bson.D{{Key: "reviewed_at", Value: 1}}}},
		{{Key: "$project", Value: projection}},
		aiReviewFeedbackLookup(bson.M{"codehost_id": "$codehost_id", "repo_owner": "$repo_owner", "repo_name": "$repo_name", "pr": "$pr"}),
		{{Key: "$unwind", Value: bson.M{"path": "$feedback", "preserveNullAndEmptyArrays": true}}},
	}
}

func loadAIReviewOverview(ctx context.Context, start, end int64, scope aiReviewQueryScope) (*aiReviewOverviewAggregate, error) {
	cursor, err := statrepo.NewAIReviewStatColl().Aggregate(ctx, aiReviewOverviewPipeline(start, end, scope), options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	aggregate := newAIReviewOverviewAggregate(start, end)
	var row struct {
		statmodels.AIReviewStat `bson:",inline"`
		Feedback                *feedbackmodels.AIReviewFeedback `bson:"feedback"`
	}
	for cursor.Next(ctx) {
		row.AIReviewStat, row.Feedback = statmodels.AIReviewStat{}, nil
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		aggregate.add(row.AIReviewStat, row.Feedback)
	}
	return aggregate, cursor.Err()
}

func loadAIReviewComparisonMetrics(ctx context.Context, start, end int64, scope aiReviewQueryScope) (AIReviewStatsOverviewMetrics, error) {
	group := bson.M{}
	pipeline := aiReviewPRMetricsPipeline(aiReviewStatMatch(start, end, scope), group, "comparison")
	pipeline = append(pipeline, aiReviewFeedbackMetricsPipeline()...)
	pipeline = append(pipeline, aiReviewSummaryMetricsPipeline(group, "comparison")...)
	pipeline = append(pipeline, bson.D{{Key: "$project", Value: bson.M{"_id": 0, "pr_count": 1, "resolution_rate": 1, "up_down_ratio": 1, "total_tokens": 1, "human_minutes": 1, "ai_minutes": 1, "saved_up": 1, "saved_down": 1}}})
	cursor, err := statrepo.NewAIReviewStatColl().Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return AIReviewStatsOverviewMetrics{}, err
	}
	defer cursor.Close(ctx)
	metrics := AIReviewStatsOverviewMetrics{AIReviewStatsMetrics: AIReviewStatsMetrics{UpDownRatio: aiReviewRatio(0, 1)}}
	if cursor.Next(ctx) {
		var row struct {
			HumanMinutes   float64  `bson:"human_minutes"`
			AIMinutes      float64  `bson:"ai_minutes"`
			SavedUp        float64  `bson:"saved_up"`
			SavedDown      float64  `bson:"saved_down"`
			PRCount        int64    `bson:"pr_count"`
			ResolutionRate *float64 `bson:"resolution_rate"`
			UpDownRatio    *float64 `bson:"up_down_ratio"`
			TotalTokens    int64    `bson:"total_tokens"`
		}
		if err := cursor.Decode(&row); err != nil {
			return AIReviewStatsOverviewMetrics{}, err
		}
		metrics.SavedPersonHours = aiReviewSavedHours(row.HumanMinutes, row.AIMinutes, row.SavedUp, row.SavedDown)
		metrics.PRCount, metrics.ResolutionRate, metrics.UpDownRatio, metrics.TotalTokens = row.PRCount, row.ResolutionRate, row.UpDownRatio, row.TotalTokens
	}
	return metrics, cursor.Err()
}

type aiReviewOverviewAggregate struct {
	metrics                           AIReviewStatsMetrics
	prs                               map[aiReviewPRKey]*feedbackmodels.AIReviewFeedback
	savedPRs                          map[aiReviewPRKey]bool
	repos                             map[aiReviewRepoKey]bool
	severity                          map[string]int64
	problemTypes                      map[string]AIReviewStatsDistributionItem
	models                            map[string]AIReviewStatsModelUsage
	feedbackKnown, resolutionKnown    bool
	humanMinutes, aiMinutes, up, down float64
	start, end                        int64
	weekly                            []AIReviewStatsWeeklyTrend
	weeklyPRs                         []map[aiReviewPRKey]bool
	weeklyUnknown                     []bool
	projectDisplayName                string
}

const aiReviewWeek = int64(7 * 24 * 60 * 60)

func newAIReviewOverviewAggregate(start, end int64) *aiReviewOverviewAggregate {
	a := &aiReviewOverviewAggregate{
		metrics: AIReviewStatsMetrics{ModelUsage: []AIReviewStatsModelUsage{}},
		prs:     make(map[aiReviewPRKey]*feedbackmodels.AIReviewFeedback), savedPRs: make(map[aiReviewPRKey]bool),
		repos: make(map[aiReviewRepoKey]bool), severity: make(map[string]int64),
		problemTypes: make(map[string]AIReviewStatsDistributionItem), models: make(map[string]AIReviewStatsModelUsage),
		feedbackKnown: true, resolutionKnown: true, start: start, end: end,
		weekly: []AIReviewStatsWeeklyTrend{},
	}
	for from := start; from < end; {
		to := from + min(aiReviewWeek, end-from)
		a.weekly = append(a.weekly, AIReviewStatsWeeklyTrend{StartTime: from, EndTime: to})
		from = to
	}
	a.weeklyPRs = make([]map[aiReviewPRKey]bool, len(a.weekly))
	a.weeklyUnknown = make([]bool, len(a.weekly))
	return a
}

func (a *aiReviewOverviewAggregate) add(record statmodels.AIReviewStat, feedback *feedbackmodels.AIReviewFeedback) {
	key := aiReviewKey(record)
	knownFeedback, seen := a.prs[key]
	if seen {
		feedback = knownFeedback
	}
	if a.projectDisplayName == "" {
		a.projectDisplayName = record.ProjectDisplayName
	}
	a.repos[aiReviewRepoKey{record.CodehostID, record.RepoOwner, record.RepoName}] = true
	a.metrics.PromptTokens += record.Usage.PromptTokens
	a.metrics.CompletionTokens += record.Usage.CompletionTokens
	a.metrics.TotalTokens += record.Usage.TotalTokens
	usage := a.models[record.Model]
	usage.Model = record.Model
	usage.PromptTokens += record.Usage.PromptTokens
	usage.CompletionTokens += record.Usage.CompletionTokens
	usage.TotalTokens += record.Usage.TotalTokens
	a.models[record.Model] = usage
	for _, finding := range record.Findings {
		addAIReviewFinding(&a.metrics, a.severity, a.problemTypes, finding)
	}
	if !seen {
		a.prs[key] = feedback
		if feedback == nil {
			a.feedbackKnown, a.resolutionKnown = false, false
		} else {
			if feedback.Provider == "" || (feedback.Provider == setting.SourceFromGithub && feedback.SyncedAt.IsZero()) {
				a.feedbackKnown = false
			}
			a.metrics.Up += int64(feedback.Up)
			a.metrics.Down += int64(feedback.Down)
			a.metrics.InlineTotal += int64(feedback.InlineTotal)
			a.metrics.InlineResolved += int64(feedback.InlineResolved)
			if feedback.ResolutionSyncedAt.IsZero() && feedback.InlineTotal > 0 {
				a.resolutionKnown = false
			} else {
				a.metrics.InlineUnresolved += int64(max(0, feedback.InlineTotal-feedback.InlineResolved))
			}
		}
	}
	a.addSavedTime(record, feedback)
	if record.ReviewedAt < a.start || record.ReviewedAt >= a.end {
		return
	}
	bucket := (record.ReviewedAt - a.start) / aiReviewWeek
	if a.weeklyPRs[bucket] == nil {
		a.weeklyPRs[bucket] = make(map[aiReviewPRKey]bool)
	}
	if a.weeklyPRs[bucket][key] {
		return
	}
	a.weeklyPRs[bucket][key] = true
	item := &a.weekly[bucket]
	item.PRCount++
	if feedback == nil {
		a.weeklyUnknown[bucket] = true
		return
	}
	item.InlineTotal += int64(feedback.InlineTotal)
	item.InlineResolved += int64(feedback.InlineResolved)
	if feedback.InlineTotal > 0 && feedback.ResolutionSyncedAt.IsZero() {
		a.weeklyUnknown[bucket] = true
	}
}

func (a *aiReviewOverviewAggregate) addSavedTime(record statmodels.AIReviewStat, feedback *feedbackmodels.AIReviewFeedback) {
	if record.DurationMS <= 0 || feedback == nil || feedback.Provider == "" || (feedback.Provider == setting.SourceFromGithub && feedback.SyncedAt.IsZero()) {
		return
	}
	var lines float64
	if record.Additions != nil && record.Deletions != nil {
		if *record.Additions < 0 || *record.Deletions < 0 {
			return
		}
		lines = float64(*record.Additions) + float64(*record.Deletions)
	} else if record.ChangedLines != nil && *record.ChangedLines >= 0 {
		lines = float64(*record.ChangedLines)
	} else {
		return
	}
	a.humanMinutes += 10 + lines*0.05
	a.aiMinutes += float64(record.DurationMS) / 60000
	key := aiReviewKey(record)
	if !a.savedPRs[key] {
		a.savedPRs[key] = true
		a.up += float64(feedback.Up)
		a.down += float64(feedback.Down)
	}
}

func (a *aiReviewOverviewAggregate) result() (AIReviewStatsMetrics, AIReviewStatsDistributions, int64, float64, []AIReviewStatsWeeklyTrend) {
	metrics := a.metrics
	metrics.PRCount = int64(len(a.prs))
	if a.resolutionKnown && metrics.InlineTotal > 0 {
		metrics.ResolutionRate = aiReviewRatio(metrics.InlineResolved, metrics.InlineTotal)
	}
	if a.feedbackKnown {
		ratio := math.Round(float64(metrics.Up)/float64(max(1, metrics.Down))*10) / 10
		metrics.UpDownRatio = &ratio
		metrics.ApprovalRate = aiReviewRatio(metrics.Up, max(1, metrics.Up+metrics.Down))
	}
	for _, usage := range a.models {
		metrics.ModelUsage = append(metrics.ModelUsage, usage)
	}
	sort.Slice(metrics.ModelUsage, func(i, j int) bool { return metrics.ModelUsage[i].Model < metrics.ModelUsage[j].Model })
	distributions := AIReviewStatsDistributions{Severities: []AIReviewStatsDistributionItem{}, ProblemTypes: []AIReviewStatsDistributionItem{}}
	for id, count := range a.severity {
		distributions.Severities = append(distributions.Severities, AIReviewStatsDistributionItem{ID: id, Name: id, Count: count})
	}
	for _, item := range a.problemTypes {
		distributions.ProblemTypes = append(distributions.ProblemTypes, item)
	}
	sort.Slice(distributions.Severities, func(i, j int) bool { return distributions.Severities[i].ID < distributions.Severities[j].ID })
	sort.Slice(distributions.ProblemTypes, func(i, j int) bool { return distributions.ProblemTypes[i].ID < distributions.ProblemTypes[j].ID })
	saved := aiReviewSavedHours(a.humanMinutes, a.aiMinutes, a.up, a.down)
	for i := range a.weekly {
		if !a.weeklyUnknown[i] && a.weekly[i].InlineTotal > 0 {
			a.weekly[i].ResolutionRate = aiReviewRatio(a.weekly[i].InlineResolved, a.weekly[i].InlineTotal)
		}
	}
	return metrics, distributions, int64(len(a.repos)), saved, a.weekly
}

func aiReviewSavedHours(humanMinutes, aiMinutes, up, down float64) float64 {
	if up+down <= 0 {
		return 0
	}
	return math.Round(max(0, humanMinutes-aiMinutes)*(up/(up+down))*1.5/60*100) / 100
}
