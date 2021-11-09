package config

import (
	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/pkg/config"
	configbase "github.com/koderover/zadig/pkg/config"
)

func MysqlDexDB() string {
	return viper.GetString(ENVMysqlDexDB)
}

func Features() string {
	return viper.GetString(FeatureFlag)
}

func MongoURI() string {
	return configbase.MongoURI()
}

func MongoDatabase() string {
	return configbase.MongoDatabase()
}

func HttpPorxy() string {
	return configbase.HttpProxy()
}
