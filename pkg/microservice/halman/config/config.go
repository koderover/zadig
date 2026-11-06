package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/koderover/zadig/v2/pkg/config"
)

const AppName = "halman"

const (
	DefaultMaxConcurrentRuns    = 32
	DefaultAdmissionWaitTimeout = 2 * time.Second
	DefaultDBMaxConnections     = 50
)

type Config struct {
	UserAuthAddress, ListenAddress      string
	MaxConcurrentRuns, DBMaxConnections int
	AdmissionWaitTimeout                time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		UserAuthAddress: os.Getenv("HALMAN_USER_AUTH_ADDRESS"),
		ListenAddress:   os.Getenv("HALMAN_LISTEN_ADDRESS"),
	}

	if c.UserAuthAddress == "" {
		c.UserAuthAddress = "user:8080"
	}

	if c.ListenAddress == "" {
		c.ListenAddress = ":8080"
	}

	var err error
	c.MaxConcurrentRuns, err = readInt("HALMAN_MAX_CONCURRENT_RUNS", DefaultMaxConcurrentRuns, 1, 1024)
	if err != nil {
		return nil, err
	}

	c.DBMaxConnections, err = readInt("HALMAN_DB_MAX_CONNECTIONS", DefaultDBMaxConnections, 2, 10000)
	if err != nil {
		return nil, err
	}

	c.AdmissionWaitTimeout = DefaultAdmissionWaitTimeout
	if raw, exists := os.LookupEnv("HALMAN_ADMISSION_WAIT_TIMEOUT"); exists {
		c.AdmissionWaitTimeout, err = time.ParseDuration(raw)
		if err != nil || c.AdmissionWaitTimeout < 0 || c.AdmissionWaitTimeout > 30*time.Second {
			return nil, fmt.Errorf("HALMAN_ADMISSION_WAIT_TIMEOUT 必须是 0 到 30s 之间的时长，例如 2s")
		}
	}

	return c, nil
}

func readInt(name string, fallback, lower, upper int) (int, error) {
	raw, exists := os.LookupEnv(name)
	if !exists {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < lower || value > upper {
		return 0, fmt.Errorf("%s 必须是 %d 到 %d 之间的整数", name, lower, upper)
	}

	return value, nil
}
func DatabaseDSN() string {
	cfg := mysql.NewConfig()
	cfg.User = config.MysqlUser()
	cfg.Passwd = config.MysqlPassword()
	cfg.Net = "tcp"
	cfg.Addr = config.MysqlHost()
	cfg.DBName = "zadig_ai"
	return cfg.FormatDSN()
}
