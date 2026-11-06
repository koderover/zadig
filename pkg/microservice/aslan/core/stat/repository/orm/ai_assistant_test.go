package orm

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/spf13/viper"
)

func TestAIAssistantStatsDatabaseIsLazyAndDoesNotInitializeSchema(t *testing.T) {
	CloseAIAssistantStatsStore()
	oldHost, oldUser := viper.Get(setting.ENVMysqlHost), viper.Get(setting.ENVMysqlUser)
	t.Cleanup(func() {
		CloseAIAssistantStatsStore()
		viper.Set(setting.ENVMysqlHost, oldHost)
		viper.Set(setting.ENVMysqlUser, oldUser)
	})
	viper.Set(setting.ENVMysqlHost, "")
	viper.Set(setting.ENVMysqlUser, "test-user")
	if _, err := GetAIAssistantStatsStore(); err == nil || assistantStatsDatabase.store != nil {
		t.Fatal("配置缺失时只能使统计不可用，不能缓存失败实例")
	}
	// 此地址无需运行 MySQL；初始化过程中不得连接或执行建表 SQL。
	viper.Set(setting.ENVMysqlHost, "127.0.0.1:1")
	store, err := GetAIAssistantStatsStore()
	if err != nil || store.SQLDB.Stats().OpenConnections != 0 || store.SQLDB.Stats().MaxOpenConnections != 3 {
		t.Fatalf("首次初始化必须延迟连接且限制连接预算: %v", err)
	}
	again, err := GetAIAssistantStatsStore()
	if err != nil || again != store {
		t.Fatal("统计请求必须复用连接池")
	}
	CloseAIAssistantStatsStore()
	if assistantStatsDatabase.store != nil {
		t.Fatal("关闭时必须释放连接池")
	}
}

func mockStatsStore(t *testing.T) (*AIAssistantStatsStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewAIAssistantStatsStore(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	if db.Stats().MaxOpenConnections != 3 {
		t.Fatal("统计连接池必须限制为三个连接")
	}
	return store, mock
}

func TestReportOnlyReadsDailyAggregateAndLeavesUnknownRatesEmpty(t *testing.T) {
	s, mock := mockStatsStore(t)
	mock.ExpectQuery("SELECT .*FROM ai_usage_daily WHERE date>=.*GROUP BY group_key").WillReturnRows(sqlmock.NewRows([]string{
		"key",
		"runs",
		"completed",
		"terminal",
		"in",
		"out",
		"elapsed",
		"completed_elapsed",
		"positive",
		"negative",
		"users",
	}).
		AddRow("overview", 1, 0, 0, 10, 5, 100, 0, 0, 0, 1))
	items, err := s.Report(context.Background(), "overview", time.Now(), time.Now())
	if err != nil || len(items) != 1 || items[0].AdoptionRate != nil || items[0].CompletionRate != nil {
		t.Fatalf("没有反馈或终态不能伪造百分比: %+v %v", items, err)
	}
}

func TestAIAssistantStatsOnlyUsesBoundedWhitelistedReadQueries(t *testing.T) {
	for _, item := range []struct{ kind, group, extra string }{
		{"overview", "'overview'", ""},
		{"trends", "DATE_FORMAT\\(date,'%Y-%m-%d'\\)", ""},
		{"skills", "s.category", regexp.QuoteMeta(" AND s.category IN ('发布辅助','故障排查','实施辅助','巡检诊断')")},
		{"skill_details", "skill_id", " AND skill_id<>''"},
		{"rankings", "user_id", ""},
	} {
		t.Run(item.kind, func(t *testing.T) {
			store, mock := mockStatsStore(t)
			join := ""
			if item.kind == "skills" {
				join = " JOIN ai_skill s ON s.id=ai_usage_daily.skill_id"
			}
			query := "SELECT " + item.group + " AS group_key,.*" + regexp.QuoteMeta("COUNT(DISTINCT CASE WHEN run_count>0 OR completed_count>0 OR terminal_count>0 THEN user_id END)") + " FROM ai_usage_daily" + regexp.QuoteMeta(join) + " WHERE date>=\\? AND date<=\\?" + item.extra + " GROUP BY group_key ORDER BY "
			if item.kind == "rankings" {
				query += "completed_count DESC,run_count DESC,group_key LIMIT 20"
			} else {
				query += "group_key$"
			}
			mock.ExpectQuery(query).WithArgs("2026-11-01", "2026-11-04").WillReturnRows(sqlmock.NewRows([]string{"key", "runs", "completed", "terminal", "in", "out", "elapsed", "completed_elapsed", "positive", "negative", "users"}).AddRow("key", 2, 1, 2, 10, 5, 100, 100, 3, 1, 1))
			from, _ := time.Parse("2006-01-02", "2026-11-01")
			to := from.AddDate(0, 0, 3)
			rows, err := store.Report(context.Background(), item.kind, from, to)
			if err != nil || len(rows) != 1 || *rows[0].CompletionRate != 0.5 || *rows[0].AdoptionRate != 0.75 {
				t.Fatalf("查询或比例计算错误: %+v %v", rows, err)
			}
			if _, err := store.Report(context.Background(), "overview;DROP TABLE ai_run", from, to); err == nil {
				t.Fatal("报告类型必须为固定白名单")
			}
		})
	}
}

func TestAIAssistantStatsReadErrorIsNotReturnedAsEmptySuccess(t *testing.T) {
	store, mock := mockStatsStore(t)
	mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows([]string{"key", "runs", "completed", "terminal", "in", "out", "elapsed", "completed_elapsed", "positive", "negative", "users"}).AddRow("key", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0).RowError(0, errors.New("read failed")))
	if _, err := store.Report(context.Background(), "overview", time.Now(), time.Now()); err == nil {
		t.Fatal("读取失败不能伪装为空报告")
	}
}
