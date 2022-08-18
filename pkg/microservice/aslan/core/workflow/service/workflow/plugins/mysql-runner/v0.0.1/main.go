package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

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
	mysqlPort := viper.GetString(MysqlPort)
	username := viper.GetString(Username)
	password := viper.GetString(Password)
	query := viper.GetString(Query)

	log.Infof("executing mysql query on host: %s, port: %s", mysqlHost, mysqlPort)

	// write the query into a sql file
	err := ioutil.WriteFile("/script.sql", []byte(query), 0777)
	if err != nil {
		log.Errorf("failed to write script into /script.sql, error: %s", err)
		os.Exit(1)
	}

	// run mysql command
	mysqlCommand := fmt.Sprintf("mysql -h %s -P %s -u %s -p%s < /script.sql", mysqlHost, mysqlPort, username, password)
	args := []string{"-c", mysqlCommand}
	cmd := exec.Command("sh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	err = cmd.Run()
	if err != nil {
		log.Errorf("failed to execute /script.sql, error: %s", err)
		os.Exit(1)
	}

	log.Infof("Mysql script execution complete")
}
