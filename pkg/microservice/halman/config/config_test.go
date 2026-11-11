package config

import (
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestLimitsDefaultsAndEnvironment(t *testing.T) {
	for _, name := range []string{"HALMAN_MAX_CONCURRENT_RUNS", "HALMAN_ADMISSION_WAIT_TIMEOUT", "HALMAN_DB_MAX_CONNECTIONS"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := Load()
	if err != nil || cfg.MaxConcurrentRuns != 32 || cfg.AdmissionWaitTimeout != 2*time.Second || cfg.DBMaxConnections != 50 {
		t.Fatalf("默认预算不正确: %+v %v", cfg, err)
	}

	t.Setenv("HALMAN_MAX_CONCURRENT_RUNS", "7")
	t.Setenv("HALMAN_ADMISSION_WAIT_TIMEOUT", "25ms")
	t.Setenv("HALMAN_DB_MAX_CONNECTIONS", "17")
	cfg, err = Load()
	if err != nil || cfg.MaxConcurrentRuns != 7 || cfg.AdmissionWaitTimeout != 25*time.Millisecond || cfg.DBMaxConnections != 17 {
		t.Fatalf("环境变量未生效: %+v %v", cfg, err)
	}
}

func TestInvalidLimitsFailStartup(t *testing.T) {
	for _, item := range []struct{ name, value string }{
		{"HALMAN_MAX_CONCURRENT_RUNS", "0"},
		{"HALMAN_MAX_CONCURRENT_RUNS", "1025"},
		{"HALMAN_MAX_CONCURRENT_RUNS", "abc"},
		{"HALMAN_DB_MAX_CONNECTIONS", "1"},
		{"HALMAN_DB_MAX_CONNECTIONS", "10001"},
		{"HALMAN_ADMISSION_WAIT_TIMEOUT", "-1s"},
		{"HALMAN_ADMISSION_WAIT_TIMEOUT", "31s"},
		{"HALMAN_ADMISSION_WAIT_TIMEOUT", "2"},
		{"HALMAN_ADMISSION_WAIT_TIMEOUT", ""},
	} {
		t.Run(item.name+"="+item.value, func(t *testing.T) {
			t.Setenv("HALMAN_MAX_CONCURRENT_RUNS", "32")
			t.Setenv("HALMAN_DB_MAX_CONNECTIONS", "50")
			t.Setenv("HALMAN_ADMISSION_WAIT_TIMEOUT", "2s")
			t.Setenv(item.name, item.value)
			if _, err := Load(); err == nil {
				t.Fatal("非法配置不能静默使用默认值")
			}
		})
	}
}

func TestDatabaseDSNUsesAIDatabaseEnvironment(t *testing.T) {
	for _, tc := range []struct{ env, want string }{
		{"", ""},
		{"custom_ai", "custom_ai"},
		{"tenant-ai", "tenant-ai"},
		{"tenant/ai", "tenant/ai"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Setenv("MYSQL_AI_DB", tc.env)
			cfg, err := mysql.ParseDSN(DatabaseDSN())
			if err != nil {
				t.Fatal(err)
			}
			if cfg.DBName != tc.want {
				t.Fatalf("database = %q, want %q", cfg.DBName, tc.want)
			}
		})
	}
}
