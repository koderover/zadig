package orm

import (
	"context"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// 只定位当前消息页涉及的运行和回答版本，不扫描消息 JSON。
func (s *Store) MessageVersions(ctx context.Context, user, conversation string, cutoff time.Time, ids []string) (map[string]models.MessageVersion, error) {
	result := make(map[string]models.MessageVersion)
	if len(ids) == 0 {
		return result, nil
	}

	if len(ids) > 128 {
		return nil, common.ErrInvalidRequest
	}

	args := []any{
		cutoff,
		cutoff,
		user,
		conversation,
		cutoff,
	}
	marks := make([]string, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}

	query := "SELECT r.id,COALESCE(r.question_source_run_id,r.id),r.answer_version," +
		"r.question_source_run_id IS NOT NULL,o.id IS NOT NULL," +
		"COALESCE((SELECT n.id FROM ai_run n WHERE n.user_id=r.user_id AND n.conversation_id=r.conversation_id AND n.created_at>=? AND n.answer_available=TRUE AND (n.id=COALESCE(r.question_source_run_id,r.id) OR n.question_source_run_id=COALESCE(r.question_source_run_id,r.id)) ORDER BY n.answer_version DESC,n.created_at DESC,n.id DESC LIMIT 1),COALESCE(r.question_source_run_id,r.id)) " +
		"FROM ai_run r LEFT JOIN ai_run o ON o.id=COALESCE(r.question_source_run_id,r.id) AND o.user_id=r.user_id AND o.conversation_id=r.conversation_id AND o.created_at>=? " +
		"WHERE r.user_id=? AND r.conversation_id=? AND r.created_at>=? AND r.id IN (" + strings.Join(marks, ",") + ")"
	rows, err := s.DB.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var id string
		var v models.MessageVersion
		if err := rows.Scan(&id, &v.OriginalID, &v.Version, &v.Regenerated, &v.QuestionAvailable, &v.LatestID); err != nil {
			return nil, err
		}

		result[id] = v
	}

	return result, rows.Err()
}
