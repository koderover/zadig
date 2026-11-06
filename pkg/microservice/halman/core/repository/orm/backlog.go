package orm

import (
	"context"
	"database/sql"
	"time"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
)

func (s *Store) HistoryBacklog(ctx context.Context, cutoff time.Time) (*time.Time, *time.Time, error) {
	var oldest *time.Time
	for _, table := range []struct {
		name, column string
		native       bool
	}{
		{"session_events", "created_at", true},
		{"session_track_events", "created_at", true},
		{"session_states", "updated_at", true},
		{"session_summaries", "updated_at", true},
		{"app_states", "updated_at", true},
		{"user_states", "updated_at", true},
		{"ai_run", "created_at", false},
		{"ai_feedback", "created_at", false},
		{"ai_conversation", "last_active_at", false},
	} {
		query := "SELECT " + table.column + " FROM " + table.name + " WHERE " + table.column + "<?"
		args := []any{cutoff}
		if table.native {
			query += " AND app_name=?"
			args = append(args, halmanconfig.AppName)
		}

		query += " ORDER BY " + table.column + ",id LIMIT 1"
		var at time.Time
		err := s.DB.WithContext(ctx).Raw(query, args...).Row().Scan(&at)
		if err == sql.ErrNoRows {
			continue
		}

		if err != nil {
			return nil, nil, err
		}

		if oldest == nil || at.Before(*oldest) {
			copy := at
			oldest = &copy
		}
	}

	var deleted time.Time
	err := s.DB.WithContext(ctx).Raw("SELECT deleted_at FROM ai_conversation WHERE deleted_at IS NOT NULL ORDER BY deleted_at,id LIMIT 1").
		Row().
		Scan(&deleted)
	if err == sql.ErrNoRows {
		return oldest, nil, nil
	}

	if err != nil {
		return nil, nil, err
	}

	return oldest, &deleted, nil
}
