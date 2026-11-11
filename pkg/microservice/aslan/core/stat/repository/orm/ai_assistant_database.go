package orm

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/koderover/zadig/v2/pkg/config"
)

type AIAssistantStatsStore struct {
	DB    *gorm.DB
	SQLDB *sql.DB
}

var assistantStatsDatabase struct {
	sync.Mutex
	store *AIAssistantStatsStore
}

func NewAIAssistantStatsStore(db *sql.DB) (*AIAssistantStatsStore, error) {
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	orm, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{
		SkipDefaultTransaction: true,
		DisableAutomaticPing:   true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	return &AIAssistantStatsStore{DB: orm, SQLDB: db}, nil
}

func GetAIAssistantStatsStore() (*AIAssistantStatsStore, error) {
	assistantStatsDatabase.Lock()
	defer assistantStatsDatabase.Unlock()
	if assistantStatsDatabase.store != nil {
		return assistantStatsDatabase.store, nil
	}
	if config.MysqlHost() == "" || config.MysqlUser() == "" {
		return nil, fmt.Errorf("AI 助手统计 MySQL 配置缺失")
	}
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Addr = config.MysqlUser(), config.MysqlPassword(), config.MysqlHost()
	cfg.Net, cfg.DBName = "tcp", config.MysqlAIDB()
	cfg.ParseTime, cfg.Loc = true, time.UTC
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 3*time.Second, 3*time.Second
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("AI 助手统计 MySQL 配置无效")
	}
	// 不探测数据库或初始化表，统计库故障不能阻止 aslan 启动及其他功能使用。
	store, err := NewAIAssistantStatsStore(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("AI 助手统计数据库访问初始化失败")
	}
	assistantStatsDatabase.store = store
	return store, nil
}

func CloseAIAssistantStatsStore() {
	assistantStatsDatabase.Lock()
	defer assistantStatsDatabase.Unlock()
	if assistantStatsDatabase.store != nil {
		assistantStatsDatabase.store.SQLDB.Close()
		assistantStatsDatabase.store = nil
	}
}
