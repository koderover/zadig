package nacos

import (
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/pkg/errors"
)

type IClient interface {
	GetConfig(dataID, group, namespaceID string) (*types.NacosConfig, error)
	ListNamespaces() ([]*types.NacosNamespace, error)
	ListConfigs(namespaceID, groupName string) ([]*types.NacosConfig, error)
	ListGroups(namespaceID, keyword string) ([]*types.NacosDataID, error)
	GetConfigHistory(dataID, group, namespaceID string) ([]*types.NacosConfigHistory, error)
	UpdateConfig(dataID, group, namespaceID, content, format string) error
	Validate() error
}

func NewNacosClient(nacosType string, serverAddr string, AuthConfig interface{}) (IClient, error) {
	switch nacosType {
	case setting.SourceFromNacos:
		nacosConfig := &NacosAuthConfig{}
		err := IToi(AuthConfig, nacosConfig)
		if err != nil {
			return nil, err
		}

		return NewNacos1Client(serverAddr, nacosConfig.UserName, nacosConfig.Password)
	case setting.SourceFromNacos3:
		nacosConfig := &NacosAuthConfig{}
		err := IToi(AuthConfig, nacosConfig)
		if err != nil {
			return nil, err
		}

		return NewNacos3Client(serverAddr, nacosConfig.UserName, nacosConfig.Password)
	case setting.SourceFromNacosEEMSE:
		nacosConfig := &NacosEEMSEAuthConfig{}
		err := IToi(AuthConfig, nacosConfig)
		if err != nil {
			return nil, err
		}

		return NewMSENacosClient(serverAddr, nacosConfig.AccessKeyId, nacosConfig.AccessKeySecret, nacosConfig.InstanceId)
	}
	return nil, errors.New("invalid nacos type")
}

func IToi(before interface{}, after interface{}) error {
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := json.Unmarshal(b, &after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}
	return nil
}
