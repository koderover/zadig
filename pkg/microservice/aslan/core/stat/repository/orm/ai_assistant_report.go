package orm

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
)

func (s *AIAssistantStatsStore) Report(ctx context.Context, kind string, from, to time.Time) ([]models.AIAssistantUsageReportRow, error) {
	group, order, extra := "'overview'", "group_key", ""
	join := ""
	switch kind {
	case "overview":
	case "trends":
		group = "DATE_FORMAT(date,'%Y-%m-%d')"
	case "skills":
		group = "s.category"
		join = " JOIN ai_skill s ON s.id=ai_usage_daily.skill_id"
		extra = " AND s.category IN ('发布辅助','故障排查','实施辅助','巡检诊断')"
	case "skill_details":
		group = "skill_id"
		extra = " AND skill_id<>''"
	case "rankings":
		group = "user_id"
		order = "completed_count DESC,run_count DESC,group_key"
	default:
		return nil, fmt.Errorf("报告类型不存在")
	}

	query := "SELECT " + group + " AS group_key,COALESCE(SUM(run_count),0) AS run_count,COALESCE(SUM(completed_count),0) AS completed_count,COALESCE(SUM(terminal_count),0),COALESCE(SUM(input_tokens),0),COALESCE(SUM(output_tokens),0),COALESCE(SUM(elapsed_ms),0),COALESCE(SUM(completed_elapsed_ms),0),COALESCE(SUM(positive_count),0),COALESCE(SUM(negative_count),0),COUNT(DISTINCT CASE WHEN run_count>0 OR completed_count>0 OR terminal_count>0 THEN user_id END) FROM ai_usage_daily" + join + " WHERE date>=? AND date<=?" + extra + " GROUP BY group_key ORDER BY " + order
	if kind == "rankings" {
		query += " LIMIT 20"
	}

	rows, err := s.DB.WithContext(ctx).Raw(query, from.Format("2006-01-02"), to.Format("2006-01-02")).Rows()
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	items := []models.AIAssistantUsageReportRow{}
	for rows.Next() {
		var item models.AIAssistantUsageReportRow
		if err := rows.Scan(
			&item.Key,
			&item.RunCount,
			&item.Completed,
			&item.Terminal,
			&item.InputTokens,
			&item.OutputTokens,
			&item.ElapsedMS,
			&item.CompletedElapsedMS,
			&item.Positive,
			&item.Negative,
			&item.ActiveUsers,
		); err != nil {
			return nil, err
		}

		if item.Terminal > 0 {
			rate := float64(item.Completed) / float64(item.Terminal)
			item.CompletionRate = &rate
		}

		if item.Positive+item.Negative > 0 {
			rate := float64(item.Positive) / float64(item.Positive+item.Negative)
			item.AdoptionRate = &rate
		}

		item.TotalTokens = item.InputTokens + item.OutputTokens
		item.ModelUsage = []models.AIAssistantStatsModelUsage{}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	// 仅加载报告实际返回的分组，避免为用户排行读取全部用户的模型明细。
	keys := []string{}
	indices := make(map[string]int, len(items))
	for i, item := range items {
		indices[item.Key] = i
		if item.InputTokens != 0 || item.OutputTokens != 0 {
			keys = append(keys, item.Key)
		}
	}
	if len(keys) == 0 || kind == "skill_details" {
		return items, nil
	}
	// Token 总数与模型明细使用同一次查询结果，避免并发写入时总量与明细不一致。
	for _, key := range keys {
		item := &items[indices[key]]
		item.InputTokens, item.OutputTokens, item.TotalTokens = 0, 0, 0
	}
	modelRows, err := s.DB.WithContext(ctx).Raw("SELECT "+group+" AS group_key,model_id,model_name,SUM(input_tokens),SUM(output_tokens) FROM ai_usage_daily"+join+" WHERE date>=? AND date<=?"+extra+" AND (input_tokens<>0 OR output_tokens<>0) AND "+group+" IN (?) GROUP BY group_key,model_id,model_name ORDER BY group_key,model_id,model_name", from.Format("2006-01-02"), to.Format("2006-01-02"), keys).Rows()
	if err != nil {
		return nil, err
	}
	defer modelRows.Close()
	for modelRows.Next() {
		var key string
		var usage models.AIAssistantStatsModelUsage
		if err := modelRows.Scan(&key, &usage.ModelID, &usage.Model, &usage.InputTokens, &usage.OutputTokens); err != nil {
			return nil, err
		}
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		if i, ok := indices[key]; ok {
			items[i].ModelUsage = append(items[i].ModelUsage, usage)
			items[i].InputTokens += usage.InputTokens
			items[i].OutputTokens += usage.OutputTokens
			items[i].TotalTokens += usage.TotalTokens
		}
	}
	return items, modelRows.Err()
}
