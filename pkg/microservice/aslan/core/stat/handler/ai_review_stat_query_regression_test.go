package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	feedbackmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	feedbackrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	statmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

func TestAIReviewQueryRegression(t *testing.T) {
	uri := os.Getenv("AI_REVIEW_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set AI_REVIEW_TEST_MONGO_URI for local integration test")
	}
	log.Init(&log.Config{Level: "error"})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("ai_review_query_test_%d", time.Now().UnixNano())
	oldDB := viper.Get(setting.ENVAslanDBName)
	viper.Set(setting.ENVAslanDBName, dbName)
	defer viper.Set(setting.ENVAslanDBName, oldDB)
	mongotool.Init(ctx, uri)
	db := mongotool.Database(dbName)
	defer db.Drop(context.Background())
	stats, feedback := statrepo.NewAIReviewStatColl(), feedbackrepo.NewAIReviewFeedbackColl()
	if err := stats.EnsureIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if err := feedback.EnsureIndex(ctx); err != nil {
		t.Fatal(err)
	}
	docs := make([]interface{}, 1000)
	for i := range docs {
		docs[i] = statmodels.AIReviewStat{ProjectName: fmt.Sprint("p", i%2), ProjectDisplayName: "project", CodehostID: 1, RepoOwner: "org", RepoName: "repo", PR: i%100 + 1, WorkflowName: "review", TaskID: int64(i), ReviewedAt: int64(100 + i), PRTitle: fmt.Sprint("title-", i), PRAuthor: "author", PRURL: "url", Model: "model", Usage: step.AIReviewTokenUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}, Findings: []statmodels.AIReviewStatFinding{{Severity: "high", Category: "bug"}}}
	}
	for i := range docs {
		r := docs[i].(statmodels.AIReviewStat)
		lines, zero, bad := int64(100), int64(0), int64(-1)
		r.Additions, r.Deletions, r.DurationMS = &lines, &zero, 30000
		r.Incomplete = i%3 == 0
		switch i % 6 {
		case 0:
			r.Additions, r.Deletions = nil, nil
		case 1:
			r.Additions, r.ChangedLines = &bad, &lines
		case 2:
			r.Additions, r.ChangedLines = nil, &lines
		case 3:
			r.Additions = &zero
		case 4:
			r.DurationMS = 0
		case 5:
			r.DurationMS = 1200000
		}
		docs[i] = r
	}
	if _, err := stats.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	docs = make([]interface{}, 99)
	for i := range docs {
		docs[i] = feedbackmodels.AIReviewFeedback{CodehostID: 1, RepoOwner: "org", RepoName: "repo", PR: i + 1, Provider: setting.SourceFromGitlab, Up: 5, Down: 4, InlineTotal: 10, InlineResolved: 3, ResolutionSyncedAt: time.Unix(100, 0)}
	}
	if _, err := feedback.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	scope := aiReviewQueryScope{CodehostID: 1, Owner: "org", Name: "repo"}
	args := AIReviewStatsListRequest{AIReviewStatsPagination: AIReviewStatsPagination{Page: 1, PageSize: 2}}
	rows, total, err := loadAIReviewPage(ctx, 100, 1200, scope, "pr", args)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var items []AIReviewStatsPRItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatal(err)
	}
	if total != 100 || len(items) != 2 || items[0].PR != 100 || items[1].PR != 99 {
		t.Fatalf("bad PR page: %+v total=%d", items, total)
	}
	if items[0].PRTitle != "title-999" || items[0].PRAuthor != "author" || items[0].PRURL != "url" || items[0].ReviewedAt != 1099 || items[0].ResolutionRate != nil || items[0].UpDownRatio != nil || items[0].InlineTotal != 0 || items[0].ResolutionSyncedAt != nil {
		t.Fatalf("missing feedback/latest metadata: %+v", items[0])
	}
	if items[1].ResolutionRate == nil || *items[1].ResolutionRate != 0.3 || items[1].UpDownRatio == nil || *items[1].UpDownRatio != 1.3 || items[1].Up != 5 || items[1].Down != 4 || items[1].ResolutionSyncedAt == nil {
		t.Fatalf("known feedback: %+v", items[1])
	}
	for _, item := range items {
		if item.PRCount != 1 || item.FindingTotal != 10 || item.PromptTokens != 10 || item.CompletionTokens != 20 || item.TotalTokens != 30 || len(item.ModelUsage) != 1 || item.ModelUsage[0].TotalTokens != 30 {
			t.Fatalf("repeated PR reports: %+v", item)
		}
	}
	// Page feedback lookup must be after limit, and PRs must only be grouped once.
	pipeline := aiReviewPagePipeline(aiReviewStatMatch(100, 1200, scope), "pr", args)
	groups := 0
	for _, stage := range pipeline {
		if _, ok := stage.Map()["$lookup"]; ok {
			t.Fatal("feedback joined before pagination")
		}
		if _, ok := stage.Map()["$group"]; ok {
			groups++
		}
	}
	if groups != 1 {
		t.Fatalf("PR groups=%d", groups)
	}
	facet := pipeline[len(pipeline)-1].Map()["$facet"].(bson.M)
	page := facet["items"].(mongo.Pipeline)
	limited := false
	for _, stage := range page {
		if _, ok := stage.Map()["$limit"]; ok {
			limited = true
		}
		if _, ok := stage.Map()["$lookup"]; ok && !limited {
			t.Fatal("feedback joined before limit")
		}
		if _, ok := stage.Map()["$group"]; ok {
			t.Fatal("redundant PR rollup")
		}
	}
	// The PR index should restrict detail reads to the current page's 20 reports.
	match := aiReviewStatMatch(100, 1200, scope)
	match["pr"] = bson.M{"$in": bson.A{99, 100}}
	group := bson.M{"codehost_id": "$codehost_id", "repo_owner": "$repo_owner", "repo_name": "$repo_name", "pr": "$pr"}
	for _, detail := range aiReviewDetailPipelines(match, group) {
		var explain struct {
			Stages []bson.M `bson:"stages"`
		}
		if err := db.RunCommand(ctx, bson.D{{Key: "explain", Value: bson.D{{Key: "aggregate", Value: stats.GetCollectionName()}, {Key: "pipeline", Value: detail}, {Key: "cursor", Value: bson.M{}}}}, {Key: "verbosity", Value: "executionStats"}}).Decode(&explain); err != nil {
			t.Fatal(err)
		}
		cursor, ok := explain.Stages[0]["$cursor"].(bson.M)
		if !ok {
			t.Fatal("missing cursor stats")
		}
		execution := cursor["executionStats"].(bson.M)
		if fmt.Sprint(execution["totalDocsExamined"]) != "20" {
			t.Fatalf("detail scanned outside page: %+v", execution)
		}
	}
	args.Page = 51
	rows, total, err = loadAIReviewPage(ctx, 100, 1200, scope, "pr", args)
	if err != nil || len(rows) != 0 || total != 100 {
		t.Fatalf("out of range: %v %d %v", rows, total, err)
	}
	// Compare the lightweight previous-period query with complete overview metrics.
	for _, phase := range []string{"missing", "known", "zero"} {
		if phase != "missing" {
			for pr := 1; pr <= 100; pr++ {
				state := feedbackmodels.AIReviewFeedback{CodehostID: 1, RepoOwner: "org", RepoName: "repo", PR: pr, Provider: setting.SourceFromGitlab}
				if phase == "known" {
					state.Up, state.Down, state.InlineTotal, state.InlineResolved = 5, 4, 10, 3
					state.ResolutionSyncedAt = time.Unix(100, 0)
				}
				if _, err := feedback.UpdateOne(ctx, bson.M{"codehost_id": 1, "repo_owner": "org", "repo_name": "repo", "pr": pr}, bson.M{"$set": state}, options.Update().SetUpsert(true)); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, filter := range []aiReviewQueryScope{{}, {Project: "p0"}, scope, {Project: "missing"}} {
			full, err := loadAIReviewOverview(ctx, 100, 1200, filter)
			if err != nil {
				t.Fatal(err)
			}
			// Compare all overview fields with the original per-report feedback join.
			legacy := newAIReviewOverviewAggregate(100, 1200)
			pipeline := aiReviewOverviewPipeline(100, 1200, filter)
			pipeline = append(pipeline, aiReviewFeedbackLookup(bson.M{"codehost_id": "$codehost_id", "repo_owner": "$repo_owner", "repo_name": "$repo_name", "pr": "$pr"}), bson.D{{Key: "$unwind", Value: bson.M{"path": "$feedback", "preserveNullAndEmptyArrays": true}}})
			cursor, err := stats.Aggregate(ctx, pipeline)
			if err != nil {
				t.Fatal(err)
			}
			for cursor.Next(ctx) {
				var row struct {
					statmodels.AIReviewStat `bson:",inline"`
					Feedback                *feedbackmodels.AIReviewFeedback `bson:"feedback"`
				}
				if err := cursor.Decode(&row); err != nil {
					t.Fatal(err)
				}
				legacy.add(row.AIReviewStat, row.Feedback)
			}
			if err := cursor.Err(); err != nil {
				t.Fatal(err)
			}
			cursor.Close(ctx)
			fullMetrics, fullDistributions, fullRepoCount, fullHours, fullWeekly := full.result()
			legacyMetrics, legacyDistributions, legacyRepoCount, legacyHours, legacyWeekly := legacy.result()
			if !reflect.DeepEqual([]interface{}{fullMetrics, fullDistributions, fullRepoCount, fullHours, fullWeekly}, []interface{}{legacyMetrics, legacyDistributions, legacyRepoCount, legacyHours, legacyWeekly}) {
				t.Fatalf("%s scope=%+v batched overview differs from per-report join", phase, filter)
			}
			want, _, _, hours, _ := full.result()
			got, err := loadAIReviewComparisonMetrics(ctx, 100, 1200, filter)
			if err != nil {
				t.Fatal(err)
			}
			if got.SavedPersonHours != hours || got.PRCount != want.PRCount || got.TotalTokens != want.TotalTokens || !reflect.DeepEqual(got.ResolutionRate, want.ResolutionRate) || !reflect.DeepEqual(got.UpDownRatio, want.UpDownRatio) {
				t.Fatalf("%s scope=%+v comparison=%+v full=%+v", phase, filter, got, want)
			}
		}
		request := &AIReviewStatsOverviewRequest{AIReviewStatsTimeRange: AIReviewStatsTimeRange{StartTime: 600, EndTime: 1100}}
		response, err := queryAIReviewOverview(ctx, request, AIReviewStatsScope{})
		if err != nil {
			t.Fatal(err)
		}
		full, err := loadAIReviewOverview(ctx, 600, 1100, aiReviewQueryScope{})
		if err != nil {
			t.Fatal(err)
		}
		_, _, _, currentHours, _ := full.result()
		previous, err := loadAIReviewComparisonMetrics(ctx, 100, 600, aiReviewQueryScope{})
		if err != nil {
			t.Fatal(err)
		}
		var change *float64
		if previous.SavedPersonHours > 0 {
			value := (currentHours - previous.SavedPersonHours) / previous.SavedPersonHours
			change = &value
		}
		if response.Metrics.SavedPersonHours != currentHours || !reflect.DeepEqual(response.Comparison.SavedPersonHours, change) {
			t.Fatalf("%s hours/comparison: %+v", phase, response)
		}
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := loadAIReviewComparisonMetrics(canceled, 100, 1200, scope); err == nil {
		t.Fatal("comparison ignored cancellation")
	}
	if _, err := loadAIReviewOverview(canceled, 100, 1200, scope); err == nil {
		t.Fatal("overview ignored cancellation")
	}
	// Cached PRs, including missing feedback, must not be queried on later batches.
	a := newAIReviewOverviewAggregate(100, 1200)
	record := statmodels.AIReviewStat{CodehostID: 1, RepoOwner: "org", RepoName: "repo", PR: 100}
	if _, err := feedback.DeleteOne(ctx, bson.M{"pr": 100}); err != nil {
		t.Fatal(err)
	}
	if err := a.addBatch(ctx, []statmodels.AIReviewStat{record, record}); err != nil {
		t.Fatal(err)
	}
	if _, err := feedback.InsertOne(ctx, feedbackmodels.AIReviewFeedback{CodehostID: 1, RepoOwner: "org", RepoName: "repo", PR: 100, Up: 9}); err != nil {
		t.Fatal(err)
	}
	if err := a.addBatch(ctx, []statmodels.AIReviewStat{record}); err != nil {
		t.Fatal(err)
	}
	if len(a.prs) != 1 || a.prs[aiReviewKey(record)] != nil || a.metrics.Up != 0 {
		t.Fatal("missing feedback was queried again")
	}
	// Equal PR numbers on different codehosts must keep separate feedback snapshots.
	record.CodehostID = 2
	if _, err := feedback.InsertOne(ctx, feedbackmodels.AIReviewFeedback{CodehostID: 2, RepoOwner: "org", RepoName: "repo", PR: 100, Provider: setting.SourceFromGitlab, Up: 7}); err != nil {
		t.Fatal(err)
	}
	if err := a.addBatch(ctx, []statmodels.AIReviewStat{record, record}); err != nil {
		t.Fatal(err)
	}
	if len(a.prs) != 2 || a.metrics.Up != 7 {
		t.Fatal("feedback keys mixed codehosts or counted duplicate PRs")
	}
}

func TestAIReviewOverviewHasNoFeedbackLookup(t *testing.T) {
	for _, stage := range aiReviewOverviewPipeline(100, 1200, aiReviewQueryScope{}) {
		if _, ok := stage.Map()["$lookup"]; ok {
			t.Fatal("overview still joins feedback per report")
		}
	}
}

func TestAIReviewSavedHoursIgnoresAIDuration(t *testing.T) {
	lines := int64(200)
	feedback := &feedbackmodels.AIReviewFeedback{Provider: setting.SourceFromGitlab, Up: 8, Down: 2}
	for _, duration := range []int64{30000, 2000000} {
		a := newAIReviewOverviewAggregate(0, 1)
		a.addSavedTime(statmodels.AIReviewStat{ChangedLines: &lines, DurationMS: duration}, feedback)
		if got := aiReviewSavedHours(a.humanMinutes, a.up, a.down); got != 0.4 {
			t.Fatalf("duration %d: got %v, want 0.4", duration, got)
		}
	}
}

func TestAIReviewSavedHours(t *testing.T) {
	for _, tc := range []struct{ human, up, down, want float64 }{
		{20, 8, 2, 0.4}, {9.5, 8, 2, 0.19}, {60, 1, 0, 1.5}, {-20, 8, 2, 0}, {20, 0, 0, 0},
	} {
		if got := aiReviewSavedHours(tc.human, tc.up, tc.down); got != tc.want {
			t.Fatalf("%+v: got %v", tc, got)
		}
	}
	change := 0.5
	raw, err := json.Marshal(AIReviewStatsOverviewResponse{Metrics: AIReviewStatsOverviewMetrics{SavedPersonHours: 1.5}, Comparison: AIReviewStatsComparison{SavedPersonHours: &change}})
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"metrics", "comparison"} {
		part := response[field].(map[string]interface{})
		if _, ok := part["saved_person_days"]; ok {
			t.Fatal("old days field returned")
		}
		want := 1.5
		if field == "comparison" {
			want = 0.5
		}
		if part["saved_person_hours"] != want {
			t.Fatal(string(raw))
		}
	}
}
