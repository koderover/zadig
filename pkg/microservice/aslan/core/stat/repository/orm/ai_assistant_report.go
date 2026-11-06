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

		items = append(items, item)
	}

	return items, rows.Err()
}
