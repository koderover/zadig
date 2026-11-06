package orm

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	sessionmysql "trpc.group/trpc-go/trpc-agent-go/session/mysql"
	storagemysql "trpc.group/trpc-go/trpc-agent-go/storage/mysql"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
)

//go:embed init/mysql.sql
var businessSchema string

type Store struct {
	DB    *gorm.DB
	SQLDB *sql.DB
}

// GORM 复用传入的业务连接池；SQLDB 仅用于健康检查、连接统计和关闭。
func NewStore(db *sql.DB, maxConnections int) (*Store, error) {
	if maxConnections < 2 || maxConnections > 10000 {
		return nil, fmt.Errorf("数据库连接总预算必须在 2 到 10000 之间")
	}

	business := maxConnections * 3 / 5
	db.SetMaxOpenConns(business)
	db.SetMaxIdleConns(min(5, business))
	db.SetConnMaxLifetime(5 * time.Minute)
	orm, err := gorm.Open(
		gormmysql.New(gormmysql.Config{
			Conn:                      db,
			SkipInitializeWithVersion: true,
		}),
		&gorm.Config{
			SkipDefaultTransaction: true,
			DisableAutomaticPing:   true,
			// 运行参数可能包含聊天内容或配置，禁止 GORM 输出 SQL 及其参数。
			Logger: logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		return nil, err
	}

	return &Store{DB: orm, SQLDB: db}, nil
}

func OpenStore(ctx context.Context, dsn string, maxConnections int) (*Store, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "zadig_ai" {
		return nil, fmt.Errorf("必须连接独立的 zadig_ai 数据库")
	}

	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	if err := initializeDatabase(ctx, cfg); err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("AI 数据库配置无效")
	}

	store, err := NewStore(db, maxConnections)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("AI 数据库访问初始化失败")
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("AI 数据库连接失败")
	}
	// 总预算按业务池六成、框架池剩余分配，不额外创建连接池。
	framework := maxConnections - maxConnections*3/5
	storagemysql.RegisterMySQLInstance(
		halmanconfig.AppName,
		storagemysql.WithClientBuilderDSN(cfg.FormatDSN()),
		storagemysql.WithMaxOpenConns(framework),
		storagemysql.WithMaxIdleConns(min(5, framework)),
		storagemysql.WithConnMaxLifetime(5*time.Minute),
	)
	return store, nil
}

func initializeDatabase(ctx context.Context, cfg *mysql.Config) error {
	bootstrap := *cfg
	bootstrap.DBName = ""
	db, err := sql.Open("mysql", bootstrap.FormatDSN())
	if err != nil {
		return fmt.Errorf("AI 数据库配置无效")
	}

	defer db.Close()
	// 引导连接在业务池和框架池创建前关闭，不叠加运行期连接预算。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `zadig_ai` DEFAULT CHARACTER SET utf8mb4"); err != nil {
		return fmt.Errorf("初始化 zadig_ai 数据库失败")
	}

	return nil
}

func (s *Store) initializeBusinessTables(ctx context.Context) error {
	for _, statement := range strings.Split(businessSchema, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}

		if err := s.DB.WithContext(ctx).Exec(statement).Error; err != nil {
			return fmt.Errorf("初始化 AI 业务表失败: %w", err)
		}
	}
	// 未上线阶段仅维护完整建表结构，不自动升级或重建已有开发表。
	for _, query := range []string{
		"SELECT title_source_at FROM ai_conversation LIMIT 0",
		"SELECT " + runColumns + ",active_plan_run_id FROM ai_run LIMIT 0",
		"SELECT completed_elapsed_ms,model_id,model_name FROM ai_usage_daily LIMIT 0",
	} {
		rows, err := s.DB.WithContext(ctx).Raw(query).Rows()
		if err != nil {
			return fmt.Errorf("AI 表结构与建表 SQL 不一致，请检查开发数据库: %w", err)
		}

		rows.Close()
	}

	var obsolete int
	if err := s.DB.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ai_run' AND COLUMN_NAME IN (?,?)",
		"framework_request_id", "replaces_run_id",
	).Row().Scan(&obsolete); err != nil {
		return fmt.Errorf("检查 AI 表结构失败: %w", err)
	}
	if obsolete > 0 {
		return fmt.Errorf("ai_run 包含已移除的开发字段，请重建开发库或手动对齐表结构")
	}

	return nil
}

func (s *Store) Initialize(ctx context.Context) error {
	if err := s.initializeBusinessTables(ctx); err != nil {
		return err
	}
	if err := s.syncBuiltinSkills(ctx); err != nil {
		return fmt.Errorf("同步内置 Skill 失败: %w", err)
	}
	// 框架表由锁定版本初始化，之后补齐受控清理所需索引。
	sessions, err := sessionmysql.NewService(sessionmysql.WithMySQLInstance(halmanconfig.AppName))
	if err != nil {
		return fmt.Errorf("初始化框架消息表失败")
	}

	defer sessions.Close()
	for _, table := range []string{"session_events", "session_track_events", "session_states"} {
		err := s.DB.WithContext(ctx).Exec("ALTER TABLE " + table + " ADD INDEX idx_halman_created (created_at, id)").
			Error
		var mysqlErr *mysql.MySQLError
		if err != nil && !(errors.As(err, &mysqlErr) && mysqlErr.Number == 1061) {
			return err
		}
	}

	for _, table := range []string{
		"session_states",
		"session_summaries",
		"app_states",
		"user_states",
	} {
		if err := s.DB.WithContext(ctx).Exec("ALTER TABLE " + table + " ADD INDEX idx_halman_updated (updated_at,id)").
			Error; err != nil {
			var mysqlErr *mysql.MySQLError
			if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1061 {
				return err
			}
		}
	}

	return nil
}
