package orm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// HistoryRuns reads the current plan state for the visible runs on one message page.
func (s *Store) HistoryRuns(ctx context.Context, user, conversation string, cutoff time.Time, ids []string) (map[string]models.HistoryRun, error) {
	runs := make(map[string]models.HistoryRun)
	if len(ids) == 0 {
		return runs, nil
	}
	if len(ids) > 128 {
		return nil, common.ErrInvalidRequest
	}

	marks := make([]string, len(ids))
	args := []any{user, conversation, cutoff}
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	query := "SELECT id,status,plan,plan_version,plan_status,plan_expires_at,error FROM ai_run" +
		" WHERE user_id=? AND conversation_id=? AND created_at>=? AND plan IS NOT NULL AND plan_version>0 AND id IN (" + strings.Join(marks, ",") + ")"
	rows, err := s.DB.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var run models.HistoryRun
		var plan []byte
		if err := rows.Scan(&run.ID, &run.Status, &plan, &run.PlanVersion, &run.PlanStatus, &run.PlanExpiresAt, &run.Error); err != nil {
			return nil, err
		}
		run.Plan = json.RawMessage(plan)
		runs[run.ID] = run
	}
	return runs, rows.Err()
}
