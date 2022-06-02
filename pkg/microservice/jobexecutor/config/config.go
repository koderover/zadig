package config

import (
	"github.com/koderover/zadig/pkg/setting"
	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/pkg/config"
)

func JobConfigFile() string {
	return viper.GetString(setting.JobConfigFile)
}

func Path() string {
	return viper.GetString(setting.Path)
}

func DockerHost() string {
	return viper.GetString(setting.DockerHost)
}

func Home() string {
	return viper.GetString(setting.Home)
}
