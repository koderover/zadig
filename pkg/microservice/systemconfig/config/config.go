package config

import (
	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/pkg/config"
)

func DexMysqlDB() string {
	return viper.GetString(ENVDexMysqlDB)
}

func DexMysqlHost() string {
	return viper.GetString(ENVDexMysqlHost)
}

func DexMysqlUser() string {
	return viper.GetString(ENVDexMysqlUser)
}

func DexMysqlPassword() string {
	return viper.GetString(ENVDexMysqlPassword)
}
