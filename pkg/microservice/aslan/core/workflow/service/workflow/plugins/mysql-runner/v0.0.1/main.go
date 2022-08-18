package main

import (
	"github.com/spf13/viper"

	"github.com/koderover/zadig/pkg/tool/log"
)

// envs
const (
	MysqlHost = "MYSQL_HOST"
	MysqlPort = "MYSQL_PORT"
	Username  = "USERNAME"
	Password  = "PASSWORD"
	Query     = "QUERY"
)

func main() {
	log.Init(&log.Config{
		Level:       "info",
		Development: false,
		MaxSize:     5,
	})
	viper.AutomaticEnv()

	mysqlHost := viper.GetString(MysqlHost)
	mysqlPort := viper.GetInt(MysqlPort)
	username := viper.GetString(Username)
	password := viper.GetString(Password)
	query := viper.GetString(Query)

	log.Infof("mysql host is: %s", mysqlHost)
	log.Infof("mysql port is: %s", mysqlPort)
	log.Infof("mysql username is: %s", username)
	log.Infof("mysql password is: %s", password)
	log.Infof("query is: %s", query)

}
