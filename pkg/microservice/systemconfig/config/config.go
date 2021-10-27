package config

import (
	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/pkg/config"
)

func MysqlDexDB() string {
	return viper.GetString(ENVMysqlDexDB)
}
