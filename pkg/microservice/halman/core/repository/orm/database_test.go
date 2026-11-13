package orm

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
)

func TestGORMReusesBoundedBusinessPool(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	store, err := NewStore(db, halmanconfig.DefaultDBMaxConnections)
	if err != nil {
		t.Fatal(err)
	}

	underlying, err := store.DB.DB()
	if err != nil || underlying != db || store.SQLDB != db {
		t.Fatalf("GORM 必须复用业务连接池: %v", err)
	}

	if db.Stats().MaxOpenConnections != 30 {
		t.Fatal("业务池必须使用总预算的六成")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelledContextStopsGORMReadAndWrite(t *testing.T) {
	store, _ := mockStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.CreateConversation(ctx, "user", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消后的请求不能创建会话: %v", err)
	}

	if err := store.CheckConversation(ctx, "user", "conversation"); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消后的请求不能继续查询: %v", err)
	}
}

func TestCustomBusinessConnectionBudget(t *testing.T) {
	for _, total := range []int{
		1,
		2,
		17,
		50,
	} {
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}

		store, err := NewStore(db, total)
		db.Close()
		if total < 2 {
			if err == nil || store != nil {
				t.Fatal("总预算必须至少给两个池各分配一个连接")
			}

			continue
		}

		if err != nil || db.Stats().MaxOpenConnections != total*3/5 {
			t.Fatalf("业务池未按总预算分配: 总数 %d，%v", total, err)
		}
	}
}

func expectBusinessTables(mock sqlmock.Sqlmock) {
	for _, query := range []string{
		"CREATE TABLE IF NOT EXISTS ai_conversation .*title_source_at.*idx_conversation_title.*idx_conversation_activity.*idx_conversation_deleted",
		"CREATE TABLE IF NOT EXISTS ai_skill .*source.*content.*enabled.*custom_rules",
		"CREATE TABLE IF NOT EXISTS ai_run .*active_plan_run_id.*answer_available.*question_source_run_id.*idx_run_plan_status.*idx_run_question_version",
		"CREATE TABLE IF NOT EXISTS ai_feedback",
		"CREATE TABLE IF NOT EXISTS ai_usage_daily .*completed_elapsed_ms",
	} {
		mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(0, 0))
	}
}

func TestBusinessInitializationUsesCompleteSchemaWithoutUpgrading(t *testing.T) {
	store, mock := mockStore(t)
	for i := 0; i < 2; i++ {
		expectBusinessTables(mock)
		mock.ExpectQuery("SELECT title_source_at FROM ai_conversation LIMIT 0").WillReturnRows(sqlmock.NewRows([]string{"title_source_at"}))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + runColumns + ",active_plan_run_id FROM ai_run LIMIT 0")).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectQuery("SELECT completed_elapsed_ms,model_id,model_name FROM ai_usage_daily LIMIT 0").WillReturnRows(sqlmock.NewRows([]string{"completed_elapsed_ms"}))
		mock.ExpectQuery("SELECT COUNT.*FROM information_schema.COLUMNS").WithArgs("framework_request_id", "replaces_run_id").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		if err := store.initializeBusinessTables(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIncompatibleDevelopmentSchemaFailsWithoutAlterOrDrop(t *testing.T) {
	store, mock := mockStore(t)
	expectBusinessTables(mock)
	dbErr := errors.New("旧开发表缺少字段")
	mock.ExpectQuery("SELECT title_source_at").WillReturnError(dbErr)
	err := store.Initialize(context.Background())
	if !errors.Is(err, dbErr) || !strings.Contains(err.Error(), "表结构与建表 SQL 不一致") {
		t.Fatalf("结构不符时应停止启动，不能修改或删除开发表: %v", err)
	}
}

func TestInitializationStopsOnDatabaseError(t *testing.T) {
	store, mock := mockStore(t)
	dbErr := errors.New("建表权限不足")
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS ai_conversation").WillReturnError(dbErr)
	if err := store.Initialize(context.Background()); !errors.Is(err, dbErr) {
		t.Fatalf("初始化失败后不能继续启动: %v", err)
	}
}

func TestRemovedDevelopmentColumnsFailBeforeAcceptingRuns(t *testing.T) {
	store, mock := mockStore(t)
	expectBusinessTables(mock)
	mock.ExpectQuery("SELECT title_source_at").WillReturnRows(sqlmock.NewRows([]string{"title_source_at"}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + runColumns + ",active_plan_run_id FROM ai_run LIMIT 0")).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT completed_elapsed_ms").WillReturnRows(sqlmock.NewRows([]string{"completed_elapsed_ms"}))
	mock.ExpectQuery("SELECT COUNT.*FROM information_schema.COLUMNS").WithArgs("framework_request_id", "replaces_run_id").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	if err := store.initializeBusinessTables(context.Background()); err == nil || !strings.Contains(err.Error(), "已移除") {
		t.Fatalf("不能让旧的必填字段在接收问题后才导致 INSERT 失败: %v", err)
	}
}

func TestOpenStoreRejectsOtherDatabase(t *testing.T) {
	t.Setenv("MYSQL_AI_DB", "custom_ai")
	for _, dsn := range []string{"invalid", "user@tcp(localhost:3306)/zadig_ai", "user@tcp(localhost:3306)/"} {
		store, err := OpenStore(context.Background(), dsn, 50)
		if store != nil || err == nil || err.Error() != "必须连接配置的 AI 数据库" {
			t.Fatalf("必须在连接前拒绝其他数据库: %v", err)
		}
	}
}
