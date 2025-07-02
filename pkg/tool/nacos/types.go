package nacos

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/setting"
)

type NacosConfig struct {
	Type            string      `json:"type" bson:"type"`
	ServerAddress   string      `json:"server_address"`
	NacosAuthConfig interface{} `json:"auth_config"`
}
type NacosAuthConfig struct {
	UserName string `json:"user_name" bson:"user_name"`
	Password string `json:"password" bson:"password"`
}

type NacosEEMSEAuthConfig struct {
	InstanceId      string `json:"instance_id" bson:"instance_id"`
	AccessKeyId     string `json:"access_key_id" bson:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret" bson:"access_key_secret"`
}

func (c *NacosConfig) GetNacosAuthConfig() (*NacosAuthConfig, error) {
	if c.NacosAuthConfig == nil {
		return nil, fmt.Errorf("nacos auth config is nil")
	}

	if c.Type != setting.SourceFromNacos {
		return nil, fmt.Errorf("nacos type is not nacos")
	}

	resp := &NacosAuthConfig{}
	err := IToi(c.NacosAuthConfig, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get nacos auth config: %w", err)
	}
	return resp, nil
}

func (c *NacosConfig) GetNacosEEMSEAuthConfig() (*NacosEEMSEAuthConfig, error) {
	if c.NacosAuthConfig == nil {
		return nil, fmt.Errorf("nacos auth config is nil")
	}

	if c.Type != setting.SourceFromNacosEEMSE {
		return nil, fmt.Errorf("nacos type is not nacos ee mse")
	}

	resp := &NacosEEMSEAuthConfig{}
	err := IToi(c.NacosAuthConfig, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get nacos auth config: %w", err)
	}
	return resp, nil
}
