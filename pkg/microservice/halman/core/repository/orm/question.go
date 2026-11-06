package orm

import (
	"context"
	"encoding/json"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// 原问题只从已有消息读取，按运行时间定位小窗口，不扫描整个会话 JSON。
func (s *Store) UserQuestion(ctx context.Context, user string, r *models.Run) (string, error) {
	rows, err := s.DB.WithContext(ctx).Raw(
		"SELECT event"+
			" FROM session_events"+
			" WHERE app_name=?"+
			" AND user_id=?"+
			" AND session_id=?"+
			" AND deleted_at IS NULL AND created_at>=? AND created_at<? ORDER BY created_at,id LIMIT 4",
		halmanconfig.AppName,
		user,
		r.ConversationID,
		r.CreatedAt,
		r.CreatedAt.Add(5*time.Minute),
	).
		Rows()
	if err != nil {
		return "", err
	}

	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return "", err
		}

		var e event.Event
		if err := json.Unmarshal(raw, &e); err != nil {
			return "", err
		}

		if e.RequestID != r.ID || e.Response == nil {
			continue
		}

		for _, choice := range e.Choices {
			if choice.Message.Role == model.RoleUser && choice.Message.Content != "" {
				return choice.Message.Content, nil
			}
		}
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	return "", common.ErrNotFound
}
