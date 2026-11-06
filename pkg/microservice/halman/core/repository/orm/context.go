package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) ContextScopes(ctx context.Context, user, conversation string, cutoff time.Time, ids []string) ([]models.ContextScope, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	if len(ids) > 128 {
		return nil, common.ErrInvalidRequest
	}

	args := []any{user, conversation, cutoff}
	placeholders := []string{}
	for _, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, "?")
	}

	rows, err := s.DB.WithContext(ctx).Raw(
		"SELECT id,context,resource_scopes FROM ai_run WHERE user_id=? AND conversation_id=? AND created_at>=? AND id IN ("+strings.Join(placeholders, ",")+")",
		args...,
	).
		Rows()
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	items := []models.ContextScope{}
	for rows.Next() {
		var item models.ContextScope
		var rawContext, rawScopes []byte
		if err := rows.Scan(&item.ID, &rawContext, &rawScopes); err != nil {
			return nil, err
		}

		var scopes []models.ResourceScope
		if json.Unmarshal(rawContext, &item.Context) != nil || json.Unmarshal(rawScopes, &scopes) != nil {
			return nil, fmt.Errorf("历史运行范围无效")
		}

		item.Scoped = len(scopes) > 0
		item.Scopes = scopes
		items = append(items, item)
	}

	return items, rows.Err()
}
