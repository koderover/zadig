package handler

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	feedbackmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func aiReviewStatMatch(start, end int64, scope aiReviewQueryScope) bson.M {
	match := bson.M{"reviewed_at": bson.M{"$gte": start, "$lt": end}}
	if scope.Project != "" {
		match["project_name"] = scope.Project
	}
	if scope.CodehostID > 0 {
		match["codehost_id"], match["repo_owner"], match["repo_name"] = scope.CodehostID, scope.Owner, scope.Name
	}
	return match
}

func aiReviewPagePipeline(match bson.M, kind string, args AIReviewStatsListRequest) mongo.Pipeline {
	keyFields := []string{"codehost_id", "repo_owner", "repo_name"}
	if kind == "project" {
		keyFields = []string{"project_name"}
	} else if kind == "pr" {
		keyFields = append(keyFields, "pr")
	}
	group := bson.M{}
	for _, field := range keyFields {
		group[field] = "$" + field
	}
	direction := -1
	if args.SortOrder == "asc" {
		direction = 1
	}
	sortBy := args.SortBy
	if sortBy == "" {
		sortBy = "pr_count"
	}
	if kind == "pr" {
		sortBy, direction = "reviewed_at", -1
	}
	ordering := bson.D{}
	if sortBy == "resolution_rate" || sortBy == "up_down_ratio" {
		ordering = append(ordering, bson.E{Key: "sort_known", Value: -1})
	}
	ordering = append(ordering, bson.E{Key: sortBy, Value: direction})
	for _, field := range keyFields {
		if kind == "pr" {
			field = "_id." + field
		}
		ordering = append(ordering, bson.E{Key: field, Value: 1})
	}
	skip := int64(args.Page - 1)
	if skip > math.MaxInt64/int64(args.PageSize) {
		skip = math.MaxInt64
	} else {
		skip *= int64(args.PageSize)
	}
	pipeline := aiReviewPRMetricsPipeline(match, group, kind)
	items := mongo.Pipeline{
		{{Key: "$sort", Value: ordering}},
		{{Key: "$skip", Value: skip}},
		{{Key: "$limit", Value: args.PageSize}},
	}
	if kind == "pr" {
		// PR ordering depends only on reports, so join feedback after pagination.
		items = append(items, aiReviewFeedbackMetricsPipeline()...)
		items = append(items, aiReviewSummaryMetricsPipeline(group, kind)...)
	} else {
		pipeline = append(pipeline, aiReviewFeedbackMetricsPipeline()...)
		pipeline = append(pipeline, aiReviewSummaryMetricsPipeline(group, kind)...)
		pipeline = append(pipeline, bson.D{{Key: "$set", Value: bson.M{"sort_known": bson.M{"$ne": bson.A{"$" + sortBy, nil}}}}})
	}
	return append(pipeline, bson.D{{Key: "$facet", Value: bson.M{
		"total": mongo.Pipeline{{{Key: "$count", Value: "count"}}},
		"items": items,
	}}})
}

func aiReviewFeedbackLookup(key interface{}) bson.D {
	feedbackMatch := bson.A{}
	for _, field := range []string{"codehost_id", "repo_owner", "repo_name", "pr"} {
		feedbackMatch = append(feedbackMatch, bson.M{"$eq": bson.A{"$" + field, "$$key." + field}})
	}
	return bson.D{{Key: "$lookup", Value: bson.M{"from": (feedbackmodels.AIReviewFeedback{}).TableName(), "let": bson.M{"key": key}, "pipeline": bson.A{
		bson.M{"$match": bson.M{"$expr": bson.M{"$and": feedbackMatch}}},
		bson.M{"$project": bson.M{"provider": 1, "synced_at": 1, "pr_title": 1, "up": 1, "down": 1, "inline_total": 1, "inline_resolved": 1, "resolution_synced_at": 1}},
	}, "as": "feedback"}}}
}

func aiReviewPRMetricsPipeline(match, group bson.M, kind string) mongo.Pipeline {
	pr := bson.M{"group": group, "codehost_id": "$codehost_id", "repo_owner": "$repo_owner", "repo_name": "$repo_name", "pr": "$pr"}
	perPR := bson.M{"_id": pr, "reviewed_at": bson.M{"$max": "$reviewed_at"}, "project_display_name": bson.M{"$max": "$project_display_name"}}
	if kind == "pr" {
		for _, field := range []string{"pr_title", "pr_author", "pr_url"} {
			perPR[field] = bson.M{"$last": "$" + field}
		}
	}
	for _, field := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		perPR[field] = bson.M{"$sum": "$usage." + field}
	}
	projection := bson.M{"_id": 1, "usage": 1}
	for _, field := range []string{"project_name", "project_display_name", "codehost_id", "repo_owner", "repo_name", "pr", "pr_title", "pr_author", "pr_url", "reviewed_at"} {
		projection[field] = 1
	}
	if kind == "comparison" {
		for _, field := range []string{"duration_ms", "additions", "deletions", "changed_lines"} {
			projection[field] = 1
		}
		lines := bson.M{"$cond": bson.A{
			bson.M{"$and": bson.A{bson.M{"$ne": bson.A{bson.M{"$ifNull": bson.A{"$additions", nil}}, nil}}, bson.M{"$ne": bson.A{bson.M{"$ifNull": bson.A{"$deletions", nil}}, nil}}}},
			bson.M{"$cond": bson.A{bson.M{"$and": bson.A{bson.M{"$gte": bson.A{"$additions", 0}}, bson.M{"$gte": bson.A{"$deletions", 0}}}}, bson.M{"$add": bson.A{bson.M{"$toDouble": "$additions"}, bson.M{"$toDouble": "$deletions"}}}, -1}},
			bson.M{"$ifNull": bson.A{"$changed_lines", -1}},
		}}
		valid := bson.M{"$and": bson.A{bson.M{"$gt": bson.A{"$duration_ms", 0}}, bson.M{"$gte": bson.A{lines, 0}}}}
		perPR["human_minutes"] = bson.M{"$sum": bson.M{"$cond": bson.A{valid, bson.M{"$add": bson.A{10, bson.M{"$multiply": bson.A{lines, 0.05}}}}, 0}}}
		perPR["ai_minutes"] = bson.M{"$sum": bson.M{"$cond": bson.A{valid, bson.M{"$divide": bson.A{"$duration_ms", 60000}}, 0}}}
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$project", Value: projection}},
	}
	// Only PR rows need the latest report metadata; summary lists need no report sort.
	if kind == "pr" {
		pipeline = append(pipeline, bson.D{{Key: "$sort", Value: bson.D{{Key: "reviewed_at", Value: 1}, {Key: "_id", Value: 1}}}})
		perPR["_id"] = group
	}
	return append(pipeline, bson.D{{Key: "$group", Value: perPR}})
}

func aiReviewFeedbackMetricsPipeline() mongo.Pipeline {
	missing := bson.M{"$eq": bson.A{bson.M{"$ifNull": bson.A{"$feedback.provider", ""}}, ""}}
	resolutionUnknown := bson.M{"$or": bson.A{bson.M{"$eq": bson.A{bson.M{"$ifNull": bson.A{"$feedback", nil}}, nil}}, bson.M{"$and": bson.A{
		bson.M{"$gt": bson.A{"$feedback.inline_total", 0}},
		bson.M{"$lte": bson.A{bson.M{"$ifNull": bson.A{"$feedback.resolution_synced_at", time.Time{}}}, time.Time{}}},
	}}}}
	feedbackUnknown := bson.M{"$or": bson.A{missing, bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{"$feedback.provider", setting.SourceFromGithub}},
		bson.M{"$lte": bson.A{bson.M{"$ifNull": bson.A{"$feedback.synced_at", time.Time{}}}, time.Time{}}},
	}}}}
	return mongo.Pipeline{
		aiReviewFeedbackLookup("$_id"),
		{{Key: "$unwind", Value: bson.M{"path": "$feedback", "preserveNullAndEmptyArrays": true}}},
		{{Key: "$set", Value: bson.M{
			"pr_title":         bson.M{"$cond": bson.A{bson.M{"$ne": bson.A{bson.M{"$ifNull": bson.A{"$feedback.pr_title", ""}}, ""}}, "$feedback.pr_title", "$pr_title"}},
			"resolution_known": bson.M{"$not": bson.A{resolutionUnknown}},
			"feedback_known":   bson.M{"$not": bson.A{feedbackUnknown}},
		}}},
		{{Key: "$set", Value: bson.M{"inline_unresolved": bson.M{"$cond": bson.A{"$resolution_known", bson.M{"$max": bson.A{0, bson.M{"$subtract": bson.A{"$feedback.inline_total", "$feedback.inline_resolved"}}}}, 0}}}}},
	}
}

func aiReviewSummaryMetricsPipeline(group bson.M, kind string) mongo.Pipeline {
	rollup := bson.M{
		"_id": "$_id.group", "pr_count": bson.M{"$sum": 1},
		"repos":                bson.M{"$addToSet": bson.M{"codehost_id": "$_id.codehost_id", "repo_owner": "$_id.repo_owner", "repo_name": "$_id.repo_name"}},
		"project_display_name": bson.M{"$max": "$project_display_name"}, "reviewed_at": bson.M{"$max": "$reviewed_at"},
		"resolution_known":  bson.M{"$min": "$resolution_known"},
		"feedback_known":    bson.M{"$min": "$feedback_known"},
		"inline_unresolved": bson.M{"$sum": "$inline_unresolved"},
	}
	for _, field := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		rollup[field] = bson.M{"$sum": "$" + field}
	}
	for _, field := range []string{"up", "down", "inline_total", "inline_resolved"} {
		rollup[field] = bson.M{"$sum": "$feedback." + field}
	}
	if kind == "comparison" {
		valid := bson.M{"$and": bson.A{"$feedback_known", bson.M{"$gt": bson.A{"$human_minutes", 0}}}}
		for _, field := range []string{"human_minutes", "ai_minutes"} {
			rollup[field] = bson.M{"$sum": bson.M{"$cond": bson.A{valid, "$" + field, 0}}}
		}
		for _, field := range []string{"up", "down"} {
			rollup["saved_"+field] = bson.M{"$sum": bson.M{"$cond": bson.A{valid, "$feedback." + field, 0}}}
		}
	}
	ratio := bson.M{"$divide": bson.A{"$up", bson.M{"$max": bson.A{1, "$down"}}}}
	roundedRatio := bson.M{"$divide": bson.A{bson.M{"$floor": bson.M{"$add": bson.A{bson.M{"$multiply": bson.A{ratio, 10}}, 0.5}}}, 10}}
	fields := bson.M{
		"repo_count":      bson.M{"$size": "$repos"},
		"resolution_rate": bson.M{"$cond": bson.A{bson.M{"$and": bson.A{"$resolution_known", bson.M{"$gt": bson.A{"$inline_total", 0}}}}, bson.M{"$divide": bson.A{"$inline_resolved", "$inline_total"}}, nil}},
		"up_down_ratio":   bson.M{"$cond": bson.A{"$feedback_known", roundedRatio, nil}},
	}
	fields["approval_rate"] = bson.M{"$cond": bson.A{"$feedback_known", bson.M{"$divide": bson.A{"$up", bson.M{"$max": bson.A{1, bson.M{"$add": bson.A{"$up", "$down"}}}}}}, nil}}
	for field := range group {
		fields[field] = "$_id." + field
	}
	fields["repo_display_name"] = "$_id.repo_name"
	pipeline := mongo.Pipeline{{{Key: "$group", Value: rollup}}}
	if kind == "pr" {
		fields["repo_count"] = 1
		values := bson.M{"pr_count": 1, "resolution_synced_at": "$feedback.resolution_synced_at"}
		for _, field := range []string{"up", "down", "inline_total", "inline_resolved"} {
			values[field] = bson.M{"$ifNull": bson.A{"$feedback." + field, 0}}
		}
		pipeline = mongo.Pipeline{{{Key: "$set", Value: values}}}
	}
	return append(pipeline,
		bson.D{{Key: "$set", Value: fields}},
		bson.D{{Key: "$project", Value: bson.M{"repos": 0, "feedback": 0}}},
	)
}

func loadAIReviewPage(ctx context.Context, start, end int64, scope aiReviewQueryScope, kind string, args AIReviewStatsListRequest) ([]bson.M, int64, error) {
	match := aiReviewStatMatch(start, end, scope)
	coll := statrepo.NewAIReviewStatColl()
	cursor, err := coll.Aggregate(ctx, aiReviewPagePipeline(match, kind, args), options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var pages []struct {
		Items []bson.M `bson:"items"`
		Total []struct {
			Count int64 `bson:"count"`
		} `bson:"total"`
	}
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, 0, err
	}
	items := []bson.M{}
	var total int64
	if len(pages) > 0 {
		items = pages[0].Items
		if len(pages[0].Total) > 0 {
			total = pages[0].Total[0].Count
		}
	}
	if len(items) == 0 {
		return []bson.M{}, total, nil
	}
	selectors := bson.A{}
	indices := map[string]int{}
	group := bson.M{}
	for i, item := range items {
		identity := item["_id"].(bson.M)
		encoded, err := json.Marshal(identity)
		if err != nil {
			return nil, 0, err
		}
		indices[string(encoded)] = i
		if kind == "pr" {
			selectors = append(selectors, identity["pr"])
		} else {
			selectors = append(selectors, identity)
		}
		for field := range identity {
			group[field] = "$" + field
		}
		item["model_usage"] = []AIReviewStatsModelUsage{}
		item["severities"] = map[string]AIReviewStatsDistributionItem{}
		item["problem_types"] = map[string]AIReviewStatsDistributionItem{}
		item["finding_total"] = int64(0)
		syncedAt := item["resolution_synced_at"]
		item["resolution_synced_at"] = nil
		if synced, ok := syncedAt.(primitive.DateTime); ok && !synced.Time().IsZero() {
			item["resolution_synced_at"] = synced.Time().Unix()
		}
		if item["project_display_name"] == "" {
			item["project_display_name"] = item["project_name"]
		}
	}
	if kind == "pr" {
		match["pr"] = bson.M{"$in": selectors}
	} else {
		match["$or"] = selectors
	}
	for query, pipeline := range aiReviewDetailPipelines(match, group) {
		cursor, err := coll.Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
		if err != nil {
			return nil, 0, err
		}
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var row struct {
				ID               bson.M `bson:"_id"`
				Model            string `bson:"model"`
				PromptTokens     int64  `bson:"prompt_tokens"`
				CompletionTokens int64  `bson:"completion_tokens"`
				TotalTokens      int64  `bson:"total_tokens"`
				Severity         string `bson:"severity"`
				Category         string `bson:"category"`
				CategoryName     string `bson:"category_name"`
				Count            int64  `bson:"count"`
			}
			if err := cursor.Decode(&row); err != nil {
				return nil, 0, err
			}
			key, err := json.Marshal(row.ID)
			if err != nil {
				return nil, 0, err
			}
			i, ok := indices[string(key)]
			if !ok {
				continue
			}
			item := items[i]
			if query == 0 {
				item["model_usage"] = append(item["model_usage"].([]AIReviewStatsModelUsage), AIReviewStatsModelUsage{Model: row.Model, PromptTokens: row.PromptTokens, CompletionTokens: row.CompletionTokens, TotalTokens: row.TotalTokens})
				continue
			}
			item["finding_total"] = item["finding_total"].(int64) + row.Count
			for _, distribution := range []struct{ field, id, name string }{
				{"severities", row.Severity, row.Severity},
				{"problem_types", row.Category, row.CategoryName},
			} {
				if distribution.id == "" {
					continue
				}
				counts := item[distribution.field].(map[string]AIReviewStatsDistributionItem)
				entry := counts[distribution.id]
				entry.ID, entry.Name = distribution.id, distribution.name
				if entry.Name == "" {
					entry.Name = entry.ID
				}
				entry.Count += row.Count
				counts[entry.ID] = entry
			}
		}
		if err := cursor.Err(); err != nil {
			return nil, 0, err
		}
	}
	for _, item := range items {
		models := item["model_usage"].([]AIReviewStatsModelUsage)
		sort.Slice(models, func(i, j int) bool { return models[i].Model < models[j].Model })
		for _, field := range []string{"severities", "problem_types"} {
			entries := []AIReviewStatsDistributionItem{}
			for _, entry := range item[field].(map[string]AIReviewStatsDistributionItem) {
				entries = append(entries, entry)
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
			item[field] = entries
		}
	}
	return items, total, nil
}

func aiReviewDetailPipelines(match, group bson.M) []mongo.Pipeline {
	models := bson.M{"_id": bson.M{"group": "$stat_group", "model": "$model"}}
	for _, field := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		models[field] = bson.M{"$sum": "$usage." + field}
	}
	modelPipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$set", Value: bson.M{"stat_group": group}}},
		{{Key: "$group", Value: models}},
		{{Key: "$project", Value: bson.M{"_id": "$_id.group", "model": "$_id.model", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 1}}},
	}
	findingPipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$set", Value: bson.M{"stat_group": group}}},
		{{Key: "$project", Value: bson.M{"stat_group": 1, "findings.severity": 1, "findings.category": 1, "findings.category_name": 1}}},
		{{Key: "$unwind", Value: "$findings"}},
		{{Key: "$group", Value: bson.M{"_id": bson.M{"group": "$stat_group", "severity": "$findings.severity", "category": "$findings.category"}, "category_name": bson.M{"$last": "$findings.category_name"}, "count": bson.M{"$sum": 1}}}},
		{{Key: "$project", Value: bson.M{"_id": "$_id.group", "severity": "$_id.severity", "category": "$_id.category", "category_name": 1, "count": 1}}},
	}
	return []mongo.Pipeline{modelPipeline, findingPipeline}
}
