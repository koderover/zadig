package main

import (
	"os"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/spf13/viper"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

// envs
const (
	ServerIP   = "SERVER_IP"
	ServerPort = "SERVER_PORT"
	TenantID   = "TENANT_ID"
	DataID     = "DATA_ID"
	Group      = "GROUP"
	Content    = "CONTENT"
	Type       = "TYPE"
	UserName   = "USER_NAME"
	Password   = "PASSWORD"
	Schema     = "SCHEMA"
)

func main() {
	log.Init(&log.Config{
		Level:       "info",
		Development: false,
		MaxSize:     5,
	})
	viper.AutomaticEnv()

	serverIP := viper.GetString(ServerIP)
	serverPort := viper.GetInt(ServerPort)
	tenantID := viper.GetString(TenantID)
	dataID := viper.GetString(DataID)
	group := viper.GetString(Group)
	content := viper.GetString(Content)
	contentType := viper.GetString(Type)
	userName := viper.GetString(UserName)
	password := viper.GetString(Password)
	schema := viper.GetString(Schema)
	if schema == "" {
		schema = "http"
	}

	clientConfig := constant.ClientConfig{
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogLevel:            "error",
		Username:            userName,
		Password:            password,
		NamespaceId:         tenantID,
		LogSampling:         &constant.ClientLogSamplingConfig{Thereafter: 100},
	}

	serverConfigs := []constant.ServerConfig{
		{
			IpAddr: serverIP,
			Port:   uint64(serverPort),
			Scheme: schema,
		},
	}

	configClient, err := clients.CreateConfigClient(map[string]interface{}{
		"clientConfig":  clientConfig,
		"serverConfigs": serverConfigs,
	})

	if err != nil {
		log.Errorf("create config client error: %s", err.Error())
		os.Exit(1)
	}
	oldConfig, err := configClient.GetConfig(vo.ConfigParam{
		DataId:  dataID,
		Group:   group,
		DatumId: tenantID,
	})
	if err != nil {
		log.Errorf("get old config error: %s", err.Error())
		os.Exit(1)
	}
	log.Infof("old config:\n %s", oldConfig)
	log.Infof("new config:\n %s", content)

	success, err := configClient.PublishConfig(vo.ConfigParam{
		DataId:  dataID,
		Group:   group,
		Content: content,
		Type:    vo.ConfigType(contentType)})
	if err != nil {
		log.Errorf("create config client error: %s", err.Error())
		os.Exit(1)
	}
	if !success {
		log.Errorf("publish coonfig failed")
		os.Exit(1)
	}
	log.Infof("create config success, dataID: %s, group: %s, namespace: %s", dataID, group, tenantID)
}
